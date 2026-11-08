package spmv

import (
	"github.com/sarchlab/mgpusim/v3/insts"
	"github.com/sarchlab/mgpusim/v3/kernels"
)

// LoadCSRKernel exposes the existing compiled scalar kernel for deterministic
// rectangular CSR adapters without changing the legacy benchmark behavior.
func LoadCSRKernel() *insts.HsaCo {
	return kernels.LoadProgramFromMemory(hsacoBytes, "spmv_csr_scalar_kernel")
}
