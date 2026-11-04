package service

import (
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"h-ui/dao"
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

const (
	hysteria2StartupAttempts = 120
	hysteria2StartupInterval = 250 * time.Millisecond
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
	if !util.Exists(util.GetHysteria2BinPath()) {
		if err := util.DownloadHysteria2(""); err != nil {
			logrus.Errorf("download hysteria2 bin err: %v", err)
			return errors.New("download hysteria2 bin err")
		}
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

func setHysteria2ConfigYAML() error {
	serverConfig, err := GetHysteria2Config()
	if err != nil {
		return err
	}
	if serverConfig.Listen == nil || *serverConfig.Listen == "" {
		return errors.New("hysteria2 config is empty")
	}
	serverConfig, err = PrepareHysteria2OutboundConfig(serverConfig)
	if err != nil {
		return err
	}
	if err := EnsureHysteria2UserRoutingCompatible(serverConfig); err != nil {
		return err
	}

	authHttpUrl, err := GetAuthHttpUrl()
	if err != nil {
		return err
	}

	// update auth http url
	if *serverConfig.Auth.HTTP.URL != authHttpUrl {
		serverConfig.Auth.HTTP.URL = &authHttpUrl
		if err := UpdateHysteria2Config(serverConfig); err != nil {
			return err
		}
	}

	hysteria2Config, err := yaml.Marshal(&serverConfig)
	if err != nil {
		logrus.Errorf("marshal hysteria2 config err: %v", err)
		return errors.New("marshal hysteria2 config err")
	}
	if err := atomicWriteFile(constant.Hysteria2ConfigPath, hysteria2Config, 0644); err != nil {
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
	hysteria2RuntimeReady.Store(false)
	return proxy.NewHysteria2Instance().StopHysteria2()
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
	secret, err := dao.GetConfig("key = ?", constant.JwtSecret)
	if err != nil {
		_ = instance.StopHysteria2()
		return err
	}
	ops := hysteria2ReadinessOps{
		processRunning: instance.IsRunning,
		mainReady:      instance.MainServerReady,
		apiReady:       func() error { return proxy.NewHysteria2Api(apiPort).Ready(*secret.Value) },
		startupOutput:  instance.LastStartupOutput,
		sleep:          time.Sleep,
	}
	if err := waitForHysteria2Readiness(ops, hysteria2StartupAttempts); err != nil {
		_ = instance.StopHysteria2()
		return err
	}
	hysteria2RuntimeReady.Store(true)
	return nil
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
