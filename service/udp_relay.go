package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/entity"
	"h-ui/util"
)

type FixedUDPRelay struct {
	Name string `json:"name" validate:"required,max=64"`
	Addr string `json:"addr" validate:"required"`
}

type FixedUDPRelayInventory struct {
	Supported bool            `json:"supported"`
	Relays    []FixedUDPRelay `json:"relays"`
}

func ListFixedUDPRelays() (FixedUDPRelayInventory, error) {
	config, err := GetHysteria2Config()
	if err != nil {
		return FixedUDPRelayInventory{}, err
	}
	list := FixedUDPRelayInventory{
		Supported: kernelSupportsUDPRelay(util.GetHysteria2BinPath()),
		Relays:    []FixedUDPRelay{},
	}
	for _, entry := range config.Outbounds {
		if isUDPRelay(entry) && entry.Name != nil && entry.UDPRelay != nil {
			list.Relays = append(list.Relays, FixedUDPRelay{Name: *entry.Name, Addr: entry.UDPRelay.Addr})
		}
	}
	return list, nil
}

// ChangeFixedUDPRelay adds a relay when previous is empty and removes it when next is nil.
func ChangeFixedUDPRelay(previous string, next *FixedUDPRelay) error {
	previous = strings.TrimSpace(previous)
	if previous == "" && next == nil {
		return errors.New("UDP relay name is required")
	}
	return runHysteria2Lifecycle(func() error {
		hysteria2ConfigApplyMutex.Lock()
		defer hysteria2ConfigApplyMutex.Unlock()
		config, err := GetHysteria2Config()
		if err != nil {
			return err
		}
		var accounts []entity.Account
		if previous != "" && (next == nil || !strings.EqualFold(previous, strings.TrimSpace(next.Name))) {
			accounts, err = dao.ListAccount("role = ?", "user")
			if err != nil {
				return err
			}
		}
		if err := changeFixedUDPRelay(&config, previous, next, accounts); err != nil {
			return err
		}
		return applyHysteria2Config(config, defaultHysteria2ConfigApplyOps())
	})
}

func changeFixedUDPRelay(config *bo.Hysteria2ServerConfig, previous string, next *FixedUDPRelay, accounts []entity.Account) error {
	index := -1
	if previous != "" {
		index = slices.IndexFunc(config.Outbounds, func(entry bo.ServerConfigOutboundEntry) bool {
			return isUDPRelay(entry) && entry.Name != nil && strings.EqualFold(*entry.Name, previous)
		})
		if index < 0 {
			return fmt.Errorf("UDP relay %q not found", previous)
		}
		if (next == nil || !strings.EqualFold(previous, strings.TrimSpace(next.Name))) && OutboundInUse(previous, accounts) {
			return fmt.Errorf("UDP relay %q is still assigned to an account", previous)
		}
	}
	if next == nil {
		config.Outbounds = slices.Delete(config.Outbounds, index, index+1)
		return nil
	}
	name, kind := strings.TrimSpace(next.Name), "udpRelay"
	entry := bo.ServerConfigOutboundEntry{
		Name: &name, Type: &kind,
		UDPRelay: &bo.ServerConfigOutboundUDPRelay{Addr: strings.TrimSpace(next.Addr)},
	}
	if err := validateOutboundEntry(entry); err != nil {
		return err
	}
	outbounds := slices.Clone(config.Outbounds)
	if index < 0 {
		outbounds = append(outbounds, entry)
	} else {
		outbounds[index] = entry
	}
	if _, err := outboundNames(outbounds); err != nil {
		return err
	}
	config.Outbounds = outbounds
	return nil
}

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
