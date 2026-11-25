package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"h-ui/model/bo"
)

func isUDPRelay(entry bo.ServerConfigOutboundEntry) bool {
	return entry.Type != nil && strings.EqualFold(*entry.Type, "udpRelay")
}

func hasUDPRelay(config bo.Hysteria2ServerConfig) bool {
	for _, entry := range config.Outbounds {
		if isUDPRelay(entry) {
			return true
		}
	}
	return false
}

func kernelSupportsUDPRelay(path string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, path, "version", "--json").Output()
	var caps struct {
		UDPRelay int `json:"udpRelay"`
	}
	return err == nil && json.Unmarshal(output, &caps) == nil && caps.UDPRelay == 1
}

func validateUDPRelayKernel(config bo.Hysteria2ServerConfig, kernelPath string) error {
	if hasUDPRelay(config) && !kernelSupportsUDPRelay(kernelPath) {
		return errors.New("installed Hysteria2 kernel does not support fixed UDP relay (udpRelay: 1)")
	}
	return nil
}

// Keep unbound relay definitions in panel storage for account selection.
func runtimeHysteria2YAML(content []byte) ([]byte, error) {
	var config bo.Hysteria2ServerConfig
	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, err
	}
	if !hasUDPRelay(config) {
		return content, nil
	}
	bound := map[string]bool{}
	for _, name := range config.UserOutbounds {
		bound[strings.ToLower(name)] = true
	}
	outbounds := config.Outbounds[:0]
	for _, entry := range config.Outbounds {
		if !isUDPRelay(entry) || (entry.Name != nil && bound[strings.ToLower(*entry.Name)]) {
			outbounds = append(outbounds, entry)
		}
	}
	config.Outbounds = outbounds
	return yaml.Marshal(config)
}

// Existing account bindings promise a fixed relay. A config edit cannot
// reuse its name for an unrestricted outbound, even if the endpoint changes.
func preserveUDPRelayTypes(previous, next []byte) error {
	var old, current bo.Hysteria2ServerConfig
	if yaml.Unmarshal(previous, &old) != nil || !hasUDPRelay(old) {
		return nil
	}
	if err := yaml.Unmarshal(next, &current); err != nil {
		return err
	}
	for _, before := range old.Outbounds {
		if !isUDPRelay(before) || before.Name == nil {
			continue
		}
		for _, after := range current.Outbounds {
			if after.Name != nil && strings.EqualFold(*before.Name, *after.Name) && !isUDPRelay(after) {
				return fmt.Errorf("UDP relay %q cannot change outbound type", *before.Name)
			}
		}
	}
	return nil
}
