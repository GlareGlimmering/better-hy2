package service

import (
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/constant"
	"h-ui/model/vo"
	"h-ui/proxy"
	"h-ui/util"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var hysteria2LifecycleMutex sync.Mutex
var hysteria2RuntimeReady atomic.Bool
var hysteria2RuntimeStatsPort atomic.Int64

const (
	hysteria2StartupAttempts        = 120
	hysteria2ACMEDNSStartupAttempts = 1200
	hysteria2StartupInterval        = 250 * time.Millisecond
)

type hysteria2ReadinessOps struct {
	processRunning func() bool
	mainReady      func() bool
	apiReady       func() error
	startupOutput  func() string
	sleep          func(time.Duration)
}

func runHysteria2Lifecycle(operation func() error) error {
	if !hysteria2LifecycleMutex.TryLock() {
		return errors.New("hysteria2 lifecycle operation already in progress")
	}
	defer hysteria2LifecycleMutex.Unlock()
	return operation()
}

func InitHysteria2() error {
	if err := ensureManagedHysteria2Kernel(util.Exists(util.GetHysteria2BinPath()), util.InstalledHysteria2Version, func() error {
		if proxy.NewHysteria2Instance().IsRunning() {
			return errors.New("stop hysteria2 before updating the managed kernel")
		}
		return util.DownloadHysteria2(constant.Hysteria2CustomVersion)
	}); err != nil {
		return fmt.Errorf("initialize managed hysteria2 kernel: %w", err)
	}

	config, err := dao.GetConfig("key = ?", constant.Hysteria2Enable)
	if err != nil {
		return err
	}

	if *config.Value == "1" {
		if err = StartHysteria2(); err != nil {
			return err
		}
	}
	return nil
}

// Upgrade only known older managed kernels during panel initialization. Keep
// manually installed hui.4 and legacy official kernels intact; never downgrade
// an unknown/future kernel. Downloads use the pinned SHA and version verifier.
func ensureManagedHysteria2Kernel(exists bool, version func() (string, error), install func() error) error {
	if !exists {
		return install()
	}
	installed, err := version()
	if err != nil {
		return err
	}
	switch installed {
	case "v2.13.0-hui.1", "v2.13.0-hui.2", "v2.13.0-hui.3":
		return install()
	default:
		return nil
	}
}

func setHysteria2ConfigYAML() error {
	serverConfig, err := GetHysteria2Config()
	if err != nil {
		return err
	}
	if serverConfig.Listen == nil || *serverConfig.Listen == "" {
		return errors.New("hysteria2 config is empty")
	}
	hysteria2Config, err := prepareHysteria2ConfigYAML(serverConfig)
	if err != nil {
		return err
	}
	if err := persistHysteria2ConfigYAML(hysteria2Config); err != nil {
		return err
	}
	if err := atomicWriteFile(constant.Hysteria2ConfigPath, hysteria2Config, constant.Hysteria2ConfigMode); err != nil {
		logrus.Errorf("write hysteria2 config file err: %v", err)
		return errors.New("hysteria2 config file write err")
	}
	return nil
}

func Hysteria2IsRunning() bool {
	if !proxy.NewHysteria2Instance().IsRunning() {
		hysteria2RuntimeReady.Store(false)
		return false
	}
	return hysteria2RuntimeReady.Load()
}

func StartHysteria2() error {
	return runHysteria2Lifecycle(startHysteria2)
}

func StopHysteria2() error {
	return runHysteria2Lifecycle(stopHysteria2)
}

func RestartHysteria2() error {
	return runHysteria2Lifecycle(func() error {
		return restartHysteria2(stopHysteria2, startHysteria2)
	})
}

func restartHysteria2(stop, start func() error) error {
	if err := stop(); err != nil {
		return fmt.Errorf("stop hysteria2 before restart: %w", err)
	}
	if err := start(); err != nil {
		return fmt.Errorf("start hysteria2 after restart: %w", err)
	}
	return nil
}

func startHysteria2() error {
	if err := setHysteria2ConfigYAML(); err != nil {
		return err
	}
	return startHysteria2Runtime()
}

func stopHysteria2() error {
	instance := proxy.NewHysteria2Instance()
	if !Hysteria2IsRunning() {
		hysteria2RuntimeReady.Store(false)
		return instance.StopHysteria2()
	}
	trafficMutex.Lock()
	defer trafficMutex.Unlock()
	port := hysteria2RuntimeStatsPort.Load()
	api := proxy.NewHysteria2Api(port)
	secret, err := trafficStatsSecret()
	if err != nil {
		return err
	}
	// An ambiguous pause response can still mean the producer is paused.
	// Attempt resume on any failure before returning to the running state.
	stopped := false
	defer func() {
		if !stopped {
			if err := api.ResumeTraffic(secret); err != nil {
				logrus.Warnf("resume Hysteria2 after interrupted drain: %v", err)
			}
		}
	}()
	if err := api.PauseTraffic(secret); err != nil {
		return fmt.Errorf("pause Hysteria2 for settlement: %w", err)
	}
	// With traffic paused there are at most two maps to settle: the existing
	// outstanding batch and the counters recorded after it was produced.
	for i := 0; i < 2; i++ {
		if err := settleManagedTraffic(port, secret); err != nil {
			return fmt.Errorf("drain Hysteria2 traffic before stop: %w", err)
		}
	}
	hysteria2RuntimeReady.Store(false)
	if err := instance.StopHysteria2(); err != nil {
		hysteria2RuntimeReady.Store(instance.IsRunning())
		return err
	}
	stopped = true
	return nil
}

func startHysteria2Runtime() error {
	hysteria2RuntimeReady.Store(false)
	instance := proxy.NewHysteria2Instance()
	if err := instance.StartHysteria2(); err != nil {
		return err
	}
	apiPort, err := GetHysteria2ApiPort()
	if err != nil {
		_ = instance.StopHysteria2()
		return err
	}
	secret, err := trafficStatsSecret()
	if err != nil {
		_ = instance.StopHysteria2()
		return err
	}
	ops := hysteria2ReadinessOps{
		processRunning: instance.IsRunning,
		mainReady:      instance.MainServerReady,
		apiReady:       func() error { return proxy.NewHysteria2Api(apiPort).Ready(secret) },
		startupOutput:  instance.LastStartupOutput,
		sleep:          time.Sleep,
	}
	attempts := hysteria2StartupAttempts
	if config, configErr := GetHysteria2Config(); configErr == nil {
		attempts = hysteria2StartupAttemptsForConfig(&config)
	}
	if err := waitForHysteria2Readiness(ops, attempts); err != nil {
		_ = instance.StopHysteria2()
		return err
	}
	if err := proxy.NewHysteria2Api(apiPort).AccountConsistencyReady(secret); err != nil {
		_ = instance.StopHysteria2()
		return fmt.Errorf("incompatible Hysteria2 kernel: account consistency protocol 2 required: %w", err)
	}
	trafficMutex.Lock()
	settlementErr := settleManagedTraffic(apiPort, secret)
	trafficMutex.Unlock()
	if settlementErr != nil {
		_ = instance.StopHysteria2()
		return fmt.Errorf("initialize traffic settlement: %w", settlementErr)
	}
	hysteria2RuntimeStatsPort.Store(apiPort)
	hysteria2RuntimeReady.Store(true)
	return nil
}

func hysteria2StartupAttemptsForConfig(config *bo.Hysteria2ServerConfig) int {
	if config != nil && config.ACME != nil && config.ACME.DNS != nil {
		// CertMagic's DNS propagation window is two minutes by default. Allow
		// the ACME operation to finish and emit its real fatal error instead of
		// replacing it with an h-ui readiness timeout after thirty seconds.
		return hysteria2ACMEDNSStartupAttempts
	}
	return hysteria2StartupAttempts
}

func waitForHysteria2Readiness(ops hysteria2ReadinessOps, attempts int) error {
	var apiErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if !ops.processRunning() {
			if output := strings.TrimSpace(ops.startupOutput()); output != "" {
				return fmt.Errorf("hysteria2 exited before readiness: %s", output)
			}
			return errors.New("hysteria2 exited before readiness")
		}
		if ops.mainReady() {
			if apiErr = ops.apiReady(); apiErr == nil {
				return nil
			}
		}
		ops.sleep(hysteria2StartupInterval)
	}
	if !ops.mainReady() {
		if output := strings.TrimSpace(ops.startupOutput()); output != "" {
			return fmt.Errorf("hysteria2 main server readiness timed out: %s", output)
		}
		return errors.New("hysteria2 main server readiness timed out")
	}
	if apiErr != nil {
		return fmt.Errorf("hysteria2 main server started but traffic API did not become ready: %w", apiErr)
	}
	return errors.New("hysteria2 traffic API did not become ready")
}

func ReleaseHysteria2() error {
	hysteria2RuntimeReady.Store(false)
	return proxy.NewHysteria2Instance().Release()
}

func Hysteria2AcmePath() (vo.Hysteria2AcmePathVo, error) {
	hysteria2AcmePathVo := vo.Hysteria2AcmePathVo{}
	hysteria2Config, err := GetHysteria2Config()
	if err != nil {
		return hysteria2AcmePathVo, err
	}
	if hysteria2Config.TLS != nil &&
		hysteria2Config.TLS.Cert != nil && *hysteria2Config.TLS.Cert != "" &&
		hysteria2Config.TLS.Key != nil && *hysteria2Config.TLS.Key != "" {
		if util.Exists(*hysteria2Config.TLS.Cert) && util.Exists(*hysteria2Config.TLS.Key) {
			hysteria2AcmePathVo.CrtPath = *hysteria2Config.TLS.Cert
			hysteria2AcmePathVo.KeyPath = *hysteria2Config.TLS.Key
			return hysteria2AcmePathVo, nil
		}
		return hysteria2AcmePathVo, errors.New("cert not found")
	} else if hysteria2Config.ACME != nil &&
		hysteria2Config.ACME.Domains != nil &&
		len(hysteria2Config.ACME.Domains) > 0 &&
		hysteria2Config.ACME.CA != nil &&
		*hysteria2Config.ACME.CA != "" &&
		hysteria2Config.ACME.Dir != nil &&
		*hysteria2Config.ACME.Dir != "" {
		acmeDir := *hysteria2Config.ACME.Dir
		for _, domain := range hysteria2Config.ACME.Domains {
			crtPath, err := util.FindFile(acmeDir, fmt.Sprintf("%s.crt", domain))
			if err != nil {
				continue
			}
			keyPath, err := util.FindFile(acmeDir, fmt.Sprintf("%s.key", domain))
			if err != nil {
				continue
			}
			hysteria2AcmePathVo.CrtPath = crtPath
			hysteria2AcmePathVo.KeyPath = keyPath
			return hysteria2AcmePathVo, nil
		}
	}
	return vo.Hysteria2AcmePathVo{}, errors.New("cert not found")
}
