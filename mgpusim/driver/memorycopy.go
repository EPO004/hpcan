package driver

import (
	"bytes"
	"encoding/binary"

	"github.com/sarchlab/akita/v3/mem/vm"
	"github.com/sarchlab/akita/v3/sim"
	"github.com/sarchlab/mgpusim/v3/protocol"
)

// defaultMemoryCopyMiddleware handles memory copy commands and related
// communication.
type hostOnlyCopy struct {
	command Command
	queue   *CommandQueue
	ready   sim.VTimeInSec
}
type defaultMemoryCopyMiddleware struct {
	driver *Driver

	cyclesPerH2D int
	cyclesPerD2H int
	cyclesLeft   int
	hostOnly     []hostOnlyCopy

	awaitingReqs []sim.Msg
}

func (m *defaultMemoryCopyMiddleware) ProcessCommand(
	now sim.VTimeInSec,
	cmd Command,
	queue *CommandQueue,
) (processed bool) {
	if observer, ok := m.driver.pageTable.(interface{ ManagedIdle() bool }); ok && !observer.ManagedIdle() {
		return false
	}
	switch cmd := cmd.(type) {
	case *FlushCommand:
		queue.IsRunning = true
		m.sendFlushRequest(now, cmd)
		return true
	case *MemCopyH2DCommand:
		return m.processMemCopyH2DCommand(now, cmd, queue)
	case *MemCopyD2HCommand:
		return m.processMemCopyD2HCommand(now, cmd, queue)
	}

	return false
}

func (m *defaultMemoryCopyMiddleware) processMemCopyH2DCommand(
	now sim.VTimeInSec,
	cmd *MemCopyH2DCommand,
	queue *CommandQueue,
) bool {
	if m.needFlushing(queue.Context, cmd.Dst, uint64(binary.Size(cmd.Src))) {
		m.sendFlushRequest(now, cmd)
	}

	buffer := bytes.NewBuffer(nil)
	err := binary.Write(buffer, binary.LittleEndian, cmd.Src)
	if err != nil {
		panic(err)
	}
	rawBytes := buffer.Bytes()
	awaitingBefore := len(m.awaitingReqs)

	offset := uint64(0)
	addr := uint64(cmd.Dst)
	sizeLeft := uint64(len(rawBytes))
	for sizeLeft > 0 {
		if observer, ok := m.driver.pageTable.(interface{ PrepareManagedHostWrite(vm.PID, uint64) []uint64 }); ok {
			for _, frame := range observer.PrepareManagedHostWrite(queue.Context.pid, addr) {
				m.driver.ReleaseGRITFrame(frame)
			}
		}
		page, found := m.driver.pageTable.Find(queue.Context.pid, addr)
		if !found {
			panic("page not found")
		}

		pAddr := page.PAddr + (addr - page.VAddr)
		sizeLeftInPage := page.PageSize - (addr - page.VAddr)
		sizeToCopy := sizeLeftInPage
		if sizeLeft < sizeLeftInPage {
			sizeToCopy = sizeLeft
		}

		if page.HostBacked {
			if err := m.driver.globalStorage.Write(pAddr, rawBytes[offset:offset+sizeToCopy]); err != nil {
				panic(err)
			}
			sizeLeft -= sizeToCopy
			addr += sizeToCopy
			offset += sizeToCopy
			continue
		}
		gpuID := m.driver.memAllocator.GetDeviceIDByPAddr(pAddr)
		req := protocol.NewMemCopyH2DReq(now,
			m.driver.gpuPort, m.driver.GPUs[gpuID-1],
			rawBytes[offset:offset+sizeToCopy],
			pAddr)
		cmd.Reqs = append(cmd.Reqs, req)
		m.awaitingReqs = append(m.awaitingReqs, req)
		// m.driver.requestsToSend = append(m.driver.requestsToSend, req)

		sizeLeft -= sizeToCopy
		addr += sizeToCopy
		offset += sizeToCopy

		m.driver.logTaskToGPUInitiate(now, cmd, req)
	}

	m.cyclesLeft = m.cyclesPerH2D

	queue.IsRunning = true
	if len(m.awaitingReqs) == awaitingBefore {
		m.hostOnly = append(m.hostOnly, hostOnlyCopy{cmd, queue, now + sim.VTimeInSec(m.cyclesPerH2D)/sim.VTimeInSec(m.driver.Freq)})
	}

	return true
}

func (m *defaultMemoryCopyMiddleware) processMemCopyD2HCommand(
	now sim.VTimeInSec,
	cmd *MemCopyD2HCommand,
	queue *CommandQueue,
) bool {
	if m.needFlushing(queue.Context, cmd.Src, uint64(binary.Size(cmd.Dst))) {
		m.sendFlushRequest(now, cmd)
		queue.Context.removeFreedBuffers()
	}

	cmd.RawData = make([]byte, binary.Size(cmd.Dst))
	awaitingBefore := len(m.awaitingReqs)

	offset := uint64(0)
	addr := uint64(cmd.Src)
	sizeLeft := uint64(len(cmd.RawData))
	for sizeLeft > 0 {
		page, found := m.driver.pageTable.Find(queue.Context.pid, addr)
		if !found {
			panic("page not found")
		}

		pAddr := page.PAddr + (addr - page.VAddr)
		sizeLeftInPage := page.PageSize - (addr - page.VAddr)
		sizeToCopy := sizeLeftInPage
		if sizeLeft < sizeLeftInPage {
			sizeToCopy = sizeLeft
		}

		if page.HostBacked {
			data, err := m.driver.globalStorage.Read(pAddr, sizeToCopy)
			if err != nil {
				panic(err)
			}
			copy(cmd.RawData[offset:offset+sizeToCopy], data)
			sizeLeft -= sizeToCopy
			addr += sizeToCopy
			offset += sizeToCopy
			continue
		}
		gpuID := m.driver.memAllocator.GetDeviceIDByPAddr(pAddr)
		req := protocol.NewMemCopyD2HReq(now,
			m.driver.gpuPort, m.driver.GPUs[gpuID-1],
			pAddr, cmd.RawData[offset:offset+sizeToCopy])
		cmd.Reqs = append(cmd.Reqs, req)
		m.awaitingReqs = append(m.awaitingReqs, req)
		// m.driver.requestsToSend = append(m.driver.requestsToSend, req)

		sizeLeft -= sizeToCopy
		addr += sizeToCopy
		offset += sizeToCopy

		m.driver.logTaskToGPUInitiate(now, cmd, req)
	}

	m.cyclesLeft = m.cyclesPerD2H

	queue.IsRunning = true
	if len(m.awaitingReqs) == awaitingBefore {
		m.hostOnly = append(m.hostOnly, hostOnlyCopy{cmd, queue, now + sim.VTimeInSec(m.cyclesPerD2H)/sim.VTimeInSec(m.driver.Freq)})
	}
	return true
}

func (m *defaultMemoryCopyMiddleware) needFlushing(
	ctx *Context,
	vAddr Ptr,
	size uint64,
) bool {
	startAddr := uint64(vAddr)
	endAddr := uint64(vAddr) + size
	for _, buf := range ctx.buffers {
		bufStartAddr := uint64(buf.vAddr)
		bufEndAddr := uint64(buf.vAddr) + buf.size
		if memRangeOverlap(bufStartAddr, bufEndAddr, startAddr, endAddr) {
			if buf.l2Dirty {
				return true
			}
		}
	}

	return false
}

func memRangeOverlap(
	start1, end1, start2, end2 uint64,
) bool {
	if start1 <= start2 && end1 >= start2 {
		return true
	}

	if start1 <= end2 && end1 >= end2 {
		return true
	}

	return false
}

func (m *defaultMemoryCopyMiddleware) sendFlushRequest(
	now sim.VTimeInSec,
	cmd Command,
) {
	for _, gpu := range m.driver.GPUs {
		req := protocol.NewFlushReq(now, m.driver.gpuPort, gpu)
		m.driver.requestsToSend = append(m.driver.requestsToSend, req)
		cmd.AddReq(req)

		m.driver.logTaskToGPUInitiate(now, cmd, req)
	}
}

func (m *defaultMemoryCopyMiddleware) Tick(
	now sim.VTimeInSec,
) (madeProgress bool) {
	madeProgress = false

	if m.cyclesLeft > 0 {
		m.cyclesLeft--
		madeProgress = true
	} else if m.cyclesLeft == 0 {
		m.driver.requestsToSend = append(m.driver.requestsToSend, m.awaitingReqs...)
		m.awaitingReqs = nil
		m.cyclesLeft = -1
		madeProgress = true
	}

	pendingHost := m.hostOnly[:0]
	for _, job := range m.hostOnly {
		if now < job.ready || len(job.command.GetReqs()) > 0 {
			pendingHost = append(pendingHost, job)
			continue
		}
		if cmd, ok := job.command.(*MemCopyD2HCommand); ok {
			if err := binary.Read(bytes.NewReader(cmd.RawData), binary.LittleEndian, cmd.Dst); err != nil {
				panic(err)
			}
		}
		job.queue.IsRunning = false
		job.queue.Dequeue()
		m.driver.logCmdComplete(job.command, now)
		madeProgress = true
	}
	m.hostOnly = pendingHost

	req := m.driver.gpuPort.Peek()
	if req == nil {
		return madeProgress || len(m.hostOnly) > 0
	}

	switch req := req.(type) {
	case *sim.GeneralRsp:
		madeProgress = m.processGeneralRsp(now, req)
	}

	return madeProgress
}

func (m *defaultMemoryCopyMiddleware) processGeneralRsp(
	now sim.VTimeInSec,
	rsp *sim.GeneralRsp,
) bool {
	originalReq := rsp.OriginalReq

	switch originalReq := originalReq.(type) {
	case *protocol.FlushReq:
		return m.processFlushReturn(now, originalReq)
	case *protocol.MemCopyH2DReq:
		return m.processMemCopyH2DReturn(now, originalReq)
	case *protocol.MemCopyD2HReq:
		return m.processMemCopyD2HReturn(now, originalReq)
	}

	return false
}

func (m *defaultMemoryCopyMiddleware) processMemCopyH2DReturn(
	now sim.VTimeInSec,
	req *protocol.MemCopyH2DReq,
) bool {
	m.driver.gpuPort.Retrieve(now)

	m.driver.logTaskToGPUClear(now, req)

	cmd, cmdQueue := m.driver.findCommandByReq(req)

	copyCmd := cmd.(*MemCopyH2DCommand)
	newReqs := make([]sim.Msg, 0, len(copyCmd.Reqs)-1)
	for _, r := range copyCmd.GetReqs() {
		if r != req {
			newReqs = append(newReqs, r)
		}
	}
	copyCmd.Reqs = newReqs

	if len(copyCmd.Reqs) == 0 {
		cmdQueue.IsRunning = false
		cmdQueue.Dequeue()

		m.driver.logCmdComplete(cmd, now)
	}

	return true
}

func (m *defaultMemoryCopyMiddleware) processMemCopyD2HReturn(
	now sim.VTimeInSec,
	req *protocol.MemCopyD2HReq,
) bool {
	m.driver.gpuPort.Retrieve(now)

	m.driver.logTaskToGPUClear(now, req)

	cmd, cmdQueue := m.driver.findCommandByReq(req)

	copyCmd := cmd.(*MemCopyD2HCommand)
	copyCmd.RemoveReq(req)

	if len(copyCmd.Reqs) == 0 {
		cmdQueue.IsRunning = false
		buf := bytes.NewReader(copyCmd.RawData)
		err := binary.Read(buf, binary.LittleEndian, copyCmd.Dst)
		if err != nil {
			panic(err)
		}

		cmdQueue.Dequeue()

		m.driver.logCmdComplete(copyCmd, now)
	}

	return true
}

func (m *defaultMemoryCopyMiddleware) processFlushReturn(
	now sim.VTimeInSec,
	req *protocol.FlushReq,
) bool {
	m.driver.gpuPort.Retrieve(now)

	m.driver.logTaskToGPUClear(now, req)

	cmd, queue := m.driver.findCommandByReq(req)
	cmd.RemoveReq(req)
	if _, flushOnly := cmd.(*FlushCommand); flushOnly && len(cmd.GetReqs()) == 0 {
		queue.Context.markAllBuffersClean()
		queue.IsRunning = false
		queue.Dequeue()
		m.driver.logCmdComplete(cmd, now)
	}

	m.driver.logTaskToGPUClear(now, req)

	return true
}
