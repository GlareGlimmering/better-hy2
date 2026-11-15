package service

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"h-ui/model/bo"
)

func TestACMEWorkerRequiresCompatibleKernelBeforeStopping(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires Linux kernel executables or shell fixtures")
	}
	const fixture = "node."
	token := fixture + strings.Repeat("a", 64)
	kernel := func(env, info string) string {
		if path := os.Getenv(env); path != "" {
			return path
		}
		path := filepath.Join(t.TempDir(), "hysteria")
		if err := os.WriteFile(path, []byte("#!/bin/sh\nprintf '%s\\n' '"+info+"'\n"), 0700); err != nil {
			t.Fatal(err)
		}
		return path
	}
	oldKernel := kernel("HUI_TEST_OLD_ACME_KERNEL", `{"accountConsistency":2,"accountBandwidth":1}`)
	newKernel := kernel("HUI_TEST_NEW_ACME_KERNEL", `{"accountConsistency":2,"accountBandwidth":1,"acmeDNSWorker":1}`)
	for _, tc := range []struct {
		name, provider, url, token, kernel, want string
	}{
		{"old kernel", "worker", "https://worker.example", token, oldKernel, "does not support"},
		{"provider URL contains a path", "worker", "https://worker.example/v1/acme/txt", token, newKernel, "HTTPS origin"},
		{"invalid credential", "worker", "https://worker.example", "not-a-token", newKernel, "invalid ACME worker_token"},
		{"new kernel", "worker", "https://worker.example/", token, newKernel, ""},
		{"Cloudflare compatibility", "cloudflare", "", "", oldKernel, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var config bo.Hysteria2ServerConfig
			if err := yaml.Unmarshal([]byte("acme: {type: dns, dns: {name: "+tc.provider+"}}"), &config); err != nil {
				t.Fatal(err)
			}
			config.ACME.DNS.Config = map[string]string{"worker_url": tc.url, "worker_token": tc.token}
			state := newFakeHysteria2ApplyState()
			ops := state.ops()
			candidate := filepath.Join(t.TempDir(), "candidate.yaml")
			ops.validate = func(string) error {
				config.Listen = workerTestString(":443")
				config.TrafficStats = &bo.ServerConfigTrafficStats{Listen: workerTestString("127.0.0.1:9999")}
				yamlConfig, err := yaml.Marshal(config)
				if err != nil {
					return err
				}
				if err := os.WriteFile(candidate, yamlConfig, 0600); err != nil {
					return err
				}
				return validateHysteria2Candidate(candidate, tc.kernel)
			}
			stopped := false
			stop := ops.stop
			ops.stop = func() error { stopped = true; return stop() }
			err := applyHysteria2Config(config, ops)
			if tc.want == "" {
				if err != nil || !stopped || !state.running || string(state.db) != "new" {
					t.Fatalf("apply: %v; stopped=%v running=%v", err, stopped, state.running)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) || strings.Contains(err.Error(), token) {
				t.Fatalf("unexpected ACME validation error: %v", err)
			}
			if stopped || !state.running || string(state.db) != "old-db" || string(state.files["hysteria2.yaml"]) != "old-yaml" {
				t.Fatal("invalid candidate changed the live configuration")
			}
		})
	}
}

func workerTestString(value string) *string { return &value }
