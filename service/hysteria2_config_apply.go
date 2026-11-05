package service

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/constant"
)

var hysteria2ConfigApplyMutex sync.Mutex

type hysteria2ConfigApplyOps struct {
	prepare   func(bo.Hysteria2ServerConfig) ([]byte, error)
	readDB    func() ([]byte, error)
	writeDB   func([]byte) error
	readFile  func(string) ([]byte, error)
	writeFile func(string, []byte, os.FileMode) error
	remove    func(string) error
	validate  func(string) error
	isRunning func() bool
	stop      func() error
	start     func() error
	path      string
}

func ApplyHysteria2Config(config bo.Hysteria2ServerConfig) error {
	return runHysteria2Lifecycle(func() error {
		return applyHysteria2ConfigSerialized(config, defaultHysteria2ConfigApplyOps())
	})
}

func applyHysteria2ConfigSerialized(config bo.Hysteria2ServerConfig, ops hysteria2ConfigApplyOps) error {
	hysteria2ConfigApplyMutex.Lock()
	defer hysteria2ConfigApplyMutex.Unlock()
	return applyHysteria2Config(config, ops)
}

func defaultHysteria2ConfigApplyOps() hysteria2ConfigApplyOps {
	return hysteria2ConfigApplyOps{
		prepare: prepareHysteria2ConfigYAML,
		readDB: func() ([]byte, error) {
			config, err := dao.GetConfig("key = ?", constant.Hysteria2Config)
			if err != nil {
				return nil, err
			}
			return []byte(*config.Value), nil
		},
		writeDB:   persistHysteria2ConfigYAML,
		readFile:  os.ReadFile,
		writeFile: atomicWriteFile,
		remove: func(path string) error {
			err := os.Remove(path)
			if os.IsNotExist(err) {
				return nil
			}
			return err
		},
		validate:  validateHysteria2Candidate,
		isRunning: Hysteria2IsRunning,
		stop:      stopHysteria2,
		start:     startHysteria2Runtime,
		path:      constant.Hysteria2ConfigPath,
	}
}

func applyHysteria2Config(config bo.Hysteria2ServerConfig, ops hysteria2ConfigApplyOps) error {
	newYAML, err := ops.prepare(config)
	if err != nil {
		return fmt.Errorf("validate new Hysteria2 config: %w", err)
	}
	oldDB, err := ops.readDB()
	if err != nil {
		return fmt.Errorf("backup old Hysteria2 database config: %w", err)
	}
	oldYAML, readErr := ops.readFile(ops.path)
	oldYAMLExists := readErr == nil
	if readErr != nil && !os.IsNotExist(readErr) {
		return fmt.Errorf("backup old Hysteria2 YAML: %w", readErr)
	}

	candidate := ops.path + ".candidate"
	if err := ops.writeFile(candidate, newYAML, 0644); err != nil {
		return fmt.Errorf("write Hysteria2 candidate config: %w", err)
	}
	defer ops.remove(candidate)
	if err := ops.validate(candidate); err != nil {
		return fmt.Errorf("validate Hysteria2 candidate config: %w", err)
	}

	wasRunning := ops.isRunning()
	if err := ops.writeDB(newYAML); err != nil {
		return fmt.Errorf("save Hysteria2 database config: %w", err)
	}
	if wasRunning {
		if err := ops.stop(); err != nil {
			return applyFailureWithRollback("stop old Hysteria2 process", err, oldDB, oldYAML, oldYAMLExists, wasRunning, ops)
		}
	}
	if err := ops.writeFile(ops.path, newYAML, 0644); err != nil {
		return applyFailureWithRollback("replace Hysteria2 YAML", err, oldDB, oldYAML, oldYAMLExists, wasRunning, ops)
	}
	if wasRunning {
		if err := ops.start(); err != nil {
			return applyFailureWithRollback("start Hysteria2 with new config", err, oldDB, oldYAML, oldYAMLExists, wasRunning, ops)
		}
	}
	return nil
}

func applyFailureWithRollback(stage string, applyErr error, oldDB, oldYAML []byte, oldYAMLExists, wasRunning bool, ops hysteria2ConfigApplyOps) error {
	rollbackErr := rollbackHysteria2Config(oldDB, oldYAML, oldYAMLExists, wasRunning, ops)
	if rollbackErr != nil {
		return fmt.Errorf("%s: %w; rollback failed: %v", stage, applyErr, rollbackErr)
	}
	return fmt.Errorf("%s: %w; previous configuration restored", stage, applyErr)
}

func rollbackHysteria2Config(oldDB, oldYAML []byte, oldYAMLExists, wasRunning bool, ops hysteria2ConfigApplyOps) error {
	var failures []string
	if err := ops.writeDB(oldDB); err != nil {
		failures = append(failures, "database: "+err.Error())
	}
	if oldYAMLExists {
		if err := ops.writeFile(ops.path, oldYAML, 0644); err != nil {
			failures = append(failures, "YAML: "+err.Error())
		}
	} else if err := ops.remove(ops.path); err != nil {
		failures = append(failures, "remove YAML: "+err.Error())
	}
	if wasRunning && !ops.isRunning() {
		if err := ops.start(); err != nil {
			failures = append(failures, "restart previous Hysteria2: "+err.Error())
		} else if !ops.isRunning() {
			failures = append(failures, "restart previous Hysteria2: process is not running")
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func validateHysteria2Candidate(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var config bo.Hysteria2ServerConfig
	if err := yaml.Unmarshal(content, &config); err != nil {
		return err
	}
	if config.Listen == nil || strings.TrimSpace(*config.Listen) == "" {
		return errors.New("listen is required")
	}
	if config.TrafficStats == nil || config.TrafficStats.Listen == nil {
		return errors.New("trafficStats.listen is required")
	}
	return ValidateHysteria2OutboundConfig(config)
}
