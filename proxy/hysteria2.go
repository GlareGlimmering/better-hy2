package proxy

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"h-ui/model/constant"
	"h-ui/util"
	"sync"
)

type Hysteria2Process struct {
	process
	binPath    string
	configPath string
}

var mutexHysteria2 sync.Mutex
var hysteria2Instance *Hysteria2Process

func init() {
	hysteria2Instance = &Hysteria2Process{process: process{mutex: &mutexHysteria2}, binPath: util.GetHysteria2BinPath(), configPath: constant.Hysteria2ConfigPath}
}

func NewHysteria2Instance() *Hysteria2Process {
	return hysteria2Instance
}

func (h *Hysteria2Process) IsRunning() bool {
	return h.isRunning()
}

func (h *Hysteria2Process) MainServerReady() bool {
	return h.mainServerReady()
}

func (h *Hysteria2Process) LastStartupOutput() string {
	return h.lastOutput()
}

func (h *Hysteria2Process) StartHysteria2() error {
	if err := hysteria2AccessLogger.initialize(); err != nil {
		// Access logging must never interrupt proxy service, but initialization
		// failures must be visible before the first client request arrives.
		logrus.Errorf("initialize hysteria2 access log %s failed: %v", constant.Hysteria2AccessLogPath, err)
	} else {
		logrus.Infof("hysteria2 access log initialized: %s", constant.Hysteria2AccessLogPath)
	}

	// Hysteria's EventLogger emits real TCP/UDP proxy request events at debug
	// level. Use the core's official environment variables so the settings are
	// independent of command-line flag placement; the parser also supports the
	// official console representation for compatibility.
	env := []string{
		"HYSTERIA_LOG_LEVEL=debug",
		"HYSTERIA_LOG_FORMAT=json",
	}
	if err := h.startWithEnv(h.binPath, env, "-c", h.configPath, "server"); err != nil {
		_ = util.RemoveFile(h.configPath)
		logrus.Errorf("start hysteria2 err: %v", err)
		return fmt.Errorf("start hysteria2: %w", err)
	}
	return nil
}

func (h *Hysteria2Process) StopHysteria2() error {
	if err := h.stop(); err != nil {
		logrus.Errorf("stop hysteria2 err: %v", err)
		return fmt.Errorf("stop hysteria2: %w", err)
	}
	_ = util.RemoveFile(h.configPath)
	return nil
}

func (h *Hysteria2Process) Release() error {
	if err := h.release(); err != nil {
		logrus.Errorf("release hysteria2 err: %v", err)
		return fmt.Errorf("release hysteria2: %w", err)
	}
	return nil
}
