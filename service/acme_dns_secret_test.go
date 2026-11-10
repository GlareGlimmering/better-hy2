package service

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"h-ui/model/bo"
)

func TestACMEDNSPlaceholdersPreserveStoredCredentials(t *testing.T) {
	const source = "acme:\n  type: dns\n  dns:\n    name: cloudflare\n    config:\n      token: fixture-token\n"
	var config bo.Hysteria2ServerConfig
	if err := yaml.Unmarshal([]byte(source), &config); err != nil {
		t.Fatal(err)
	}
	RedactACMEDNS(&config)
	config.ACME.DNS.Config["another"] = "replacement"
	out, err := yaml.Marshal(config)
	if err != nil || strings.Contains(string(out), "fixture-token") {
		t.Fatal("masked export contains stored credential")
	}
	read := func() ([]byte, error) { return []byte(source), nil }
	if err := restoreACMEDNS(&config, read); err != nil || config.ACME.DNS.Config["token"] != "fixture-token" || config.ACME.DNS.Config["another"] != "replacement" {
		t.Fatal("stored credentials were not merged correctly")
	}
	RedactACMEDNS(&config)
	*config.ACME.DNS.Name = "different"
	if err := restoreACMEDNS(&config, read); err == nil {
		t.Fatal("credentials must not be copied to another provider")
	}
}
