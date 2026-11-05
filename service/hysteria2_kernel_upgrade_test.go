package service

import (
	"errors"
	"h-ui/model/constant"
	"testing"
)

func TestManagedKernelUpgradeRecognizesManualInstall(t *testing.T) {
	for _, tc := range []struct {
		version             string
		exists, wantInstall bool
	}{
		{"", false, true}, {"v2.13.0-hui.3", true, true}, {"v2.13.0-hui.4", true, false}, {"v2.13.0", true, false}, {"v2.13.0-hui.5", true, false},
	} {
		t.Run(tc.version, func(t *testing.T) {
			called := false
			err := ensureManagedHysteria2Kernel(tc.exists, func() (string, error) { return tc.version, nil }, func() error { called = true; return nil })
			if err != nil || called != tc.wantInstall {
				t.Fatalf("install=%v err=%v", called, err)
			}
		})
	}
	if constant.Hysteria2CustomVersion != "v2.13.0-hui.4" {
		t.Fatal("unexpected pin")
	}
	failure := errors.New("checksum failed")
	if err := ensureManagedHysteria2Kernel(true, func() (string, error) { return "v2.13.0-hui.3", nil }, func() error { return failure }); !errors.Is(err, failure) {
		t.Fatalf("installation failure hidden: %v", err)
	}
}
