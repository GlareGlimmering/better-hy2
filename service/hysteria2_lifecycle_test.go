package service

import (
	"errors"
	"gopkg.in/yaml.v3"
	"h-ui/model/bo"
	"strings"
	"testing"
	"time"
)

func TestACMEDNSStartupAllowsPropagationWindow(t *testing.T) {
	var config bo.Hysteria2ServerConfig
	if err := yaml.Unmarshal([]byte("acme:\n  type: dns\n  dns:\n    name: cloudflare\n    config: {}\n"), &config); err != nil {
		t.Fatal(err)
	}
	if got := hysteria2StartupAttemptsForConfig(&config); got != hysteria2ACMEDNSStartupAttempts {
		t.Fatalf("DNS-01 startup attempts=%d want %d", got, hysteria2ACMEDNSStartupAttempts)
	}
	if time.Duration(hysteria2ACMEDNSStartupAttempts)*hysteria2StartupInterval < 2*time.Minute {
		t.Fatal("DNS-01 readiness window is shorter than CertMagic propagation timeout")
	}
	if got := hysteria2StartupAttemptsForConfig(&bo.Hysteria2ServerConfig{}); got != hysteria2StartupAttempts {
		t.Fatalf("ordinary startup attempts=%d want %d", got, hysteria2StartupAttempts)
	}

	processChecks := 0
	primary := "ACME DNS-01 failed: propagation resolver returned a fatal error"
	err := waitForHysteria2Readiness(hysteria2ReadinessOps{
		processRunning: func() bool {
			processChecks++
			return processChecks <= hysteria2StartupAttempts
		},
		mainReady:     func() bool { return false },
		apiReady:      func() error { return errors.New("unexpected API probe") },
		startupOutput: func() string { return primary },
		sleep:         func(time.Duration) {},
	}, hysteria2StartupAttemptsForConfig(&config))
	if err == nil || !strings.Contains(err.Error(), primary) || strings.Contains(err.Error(), "readiness timed out") {
		t.Fatalf("real ACME fatal error was not preserved after the ordinary readiness window: %v", err)
	}
}

func TestRestartHysteria2UsesStopThenStartAndPropagatesFailures(t *testing.T) {
	var calls []string
	err := restartHysteria2(
		func() error { calls = append(calls, "stop"); return nil },
		func() error { calls = append(calls, "start"); return nil },
	)
	if err != nil || strings.Join(calls, ",") != "stop,start" {
		t.Fatalf("calls=%v err=%v", calls, err)
	}

	calls = nil
	err = restartHysteria2(
		func() error { calls = append(calls, "stop"); return errors.New("busy") },
		func() error { calls = append(calls, "start"); return nil },
	)
	if err == nil || !strings.Contains(err.Error(), "stop hysteria2") || strings.Join(calls, ",") != "stop" {
		t.Fatalf("stop failure calls=%v err=%v", calls, err)
	}

	err = restartHysteria2(func() error { return nil }, func() error { return errors.New("invalid config") })
	if err == nil || !strings.Contains(err.Error(), "start hysteria2") {
		t.Fatalf("start failure err=%v", err)
	}
}

func TestWaitForHysteria2Readiness(t *testing.T) {
	t.Run("API delay succeeds only after main server readiness", func(t *testing.T) {
		apiCalls := 0
		err := waitForHysteria2Readiness(hysteria2ReadinessOps{
			processRunning: func() bool { return true },
			mainReady:      func() bool { return true },
			apiReady: func() error {
				apiCalls++
				if apiCalls < 3 {
					return errors.New("connection refused")
				}
				return nil
			},
			startupOutput: func() string { return "server up and running" },
			sleep:         func(time.Duration) {},
		}, 4)
		if err != nil || apiCalls != 3 {
			t.Fatalf("calls=%d err=%v", apiCalls, err)
		}
	})

	for _, primary := range []string{
		"listen udp :443: bind: address already in use",
		"ACME certificate request failed: authorization denied",
	} {
		t.Run(primary, func(t *testing.T) {
			apiCalls := 0
			err := waitForHysteria2Readiness(hysteria2ReadinessOps{
				processRunning: func() bool { return false },
				mainReady:      func() bool { return false },
				apiReady:       func() error { apiCalls++; return errors.New("127.0.0.1:9999 connection refused") },
				startupOutput:  func() string { return primary },
				sleep:          func(time.Duration) {},
			}, 2)
			if err == nil || !strings.Contains(err.Error(), primary) || strings.Contains(err.Error(), "9999") {
				t.Fatalf("primary startup error was not preserved: %v", err)
			}
			if apiCalls != 0 {
				t.Fatalf("stopped process polled online API %d time(s)", apiCalls)
			}
		})
	}

	t.Run("main server ready but API unavailable", func(t *testing.T) {
		err := waitForHysteria2Readiness(hysteria2ReadinessOps{
			processRunning: func() bool { return true },
			mainReady:      func() bool { return true },
			apiReady:       func() error { return errors.New("127.0.0.1:9999 connection refused") },
			startupOutput:  func() string { return "server up and running" },
			sleep:          func(time.Duration) {},
		}, 2)
		if err == nil || !strings.Contains(err.Error(), "traffic API did not become ready") || !strings.Contains(err.Error(), "9999") {
			t.Fatalf("error=%v", err)
		}
	})

	t.Run("panel restart can establish readiness again", func(t *testing.T) {
		ops := hysteria2ReadinessOps{
			processRunning: func() bool { return true }, mainReady: func() bool { return true },
			apiReady: func() error { return nil }, startupOutput: func() string { return "server up and running" },
			sleep: func(time.Duration) {},
		}
		if err := waitForHysteria2Readiness(ops, 1); err != nil {
			t.Fatal(err)
		}
		if err := waitForHysteria2Readiness(ops, 1); err != nil {
			t.Fatal(err)
		}
	})
}

func TestHysteria2LifecycleRejectsConcurrentOperation(t *testing.T) {
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runHysteria2Lifecycle(func() error {
			close(entered)
			<-release
			return nil
		})
	}()
	<-entered
	err := runHysteria2Lifecycle(func() error { return nil })
	if err == nil || !strings.Contains(err.Error(), "already in progress") {
		t.Fatalf("concurrent operation error=%v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("first operation error=%v", err)
	}
}
