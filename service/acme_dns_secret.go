package service

import (
	"errors"
	"maps"
	"strings"

	"gopkg.in/yaml.v3"
	"h-ui/model/bo"
)

const acmeDNSRedacted = "********"

func RedactACMEDNS(config *bo.Hysteria2ServerConfig) {
	if config.ACME == nil || config.ACME.DNS == nil {
		return
	}
	values := maps.Clone(config.ACME.DNS.Config)
	for key := range values {
		values[key] = acmeDNSRedacted
	}
	config.ACME.DNS.Config = values
}

func restoreACMEDNS(config *bo.Hysteria2ServerConfig, readStored func() ([]byte, error)) error {
	if config.ACME == nil || config.ACME.DNS == nil {
		return nil
	}
	dns := config.ACME.DNS
	var stored bo.Hysteria2ServerConfig
	loaded := false
	for key, value := range dns.Config {
		if value != acmeDNSRedacted {
			continue
		}
		if !loaded {
			raw, err := readStored()
			if err != nil || yaml.Unmarshal(raw, &stored) != nil {
				return errors.New("could not read stored ACME DNS credentials")
			}
			dns.Config = maps.Clone(dns.Config)
			loaded = true
		}
		if stored.ACME == nil || stored.ACME.DNS == nil || dns.Name == nil ||
			stored.ACME.DNS.Name == nil || *stored.ACME.DNS.Name != *dns.Name {
			return errors.New("enter credentials for the selected DNS provider")
		}
		secret, ok := stored.ACME.DNS.Config[key]
		if !ok || secret == acmeDNSRedacted {
			return errors.New("enter a value for the new ACME DNS credential")
		}
		dns.Config[key] = secret
	}
	return nil
}

func ACMEDNSValue(provider, key string) (string, error) {
	missing := errors.New("stored ACME DNS credential unavailable")
	config, err := GetHysteria2Config()
	if err != nil || config.ACME == nil || config.ACME.DNS == nil ||
		config.ACME.DNS.Name == nil || *config.ACME.DNS.Name != provider {
		return "", missing
	}
	value, ok := config.ACME.DNS.Config[key]
	if !ok || strings.TrimSpace(value) == "" || value == acmeDNSRedacted {
		return "", missing
	}
	return value, nil
}
