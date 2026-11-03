package vm

import "testing"

func TestTranslationAccessIntent(t *testing.T) {
	r := TranslationReqBuilder{}.WithPID(3).WithVAddr(4096).WithDeviceID(2).WithAccessKind(AccessWrite).Build()
	if r.AccessKind != AccessWrite || r.PID != 3 || r.DeviceID != 2 {
		t.Fatal("builder lost access identity")
	}
	if (TranslationReqBuilder{}).Build().AccessKind != AccessRead {
		t.Fatal("legacy request must default to read")
	}
}
