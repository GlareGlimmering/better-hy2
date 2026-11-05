package service

import (
	"encoding/json"
	"gopkg.in/yaml.v3"
	"net"
	"strings"
	"testing"

	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/constant"
)

func optionRule(name, address string) bo.ServerConfigOutboundEntry {
	return bo.ServerConfigOutboundEntry{Name: strPointer(name), Type: strPointer("socks5"), SOCKS5: &bo.ServerConfigOutboundSOCKS5{Addr: strPointer(address)}}
}

func optionSettings(name string, port uint16) bo.XraySettings {
	settings := normalizeXraySettings(validXraySettings())
	settings.Outbounds[0].Name = name
	settings.Outbounds[0].SOCKSPort = port
	return settings
}

func TestAccountOptionsExcludeHistoricalPortWithoutDeletingNativeRules(t *testing.T) {
	current := optionSettings("CURRENT", 10810)
	config := bo.Hysteria2ServerConfig{Outbounds: []bo.ServerConfigOutboundEntry{
		optionRule("OLD", "127.0.0.1:10808"), optionRule("CURRENT", "127.0.0.1:10810"),
		optionRule("PRIVATE_LOCAL", "127.0.0.1:19000"), optionRule("PRIVATE_REMOTE", "192.0.2.10:1080"),
	}}
	before, _ := json.Marshal(config)
	result := accountOutboundOptions(current, current, config, func(address string) bool {
		if address == "192.0.2.10:1080" {
			t.Fatal("native remote proxy must not be probed")
		}
		return address == "127.0.0.1:19000"
	})
	if len(result.Options) != 4 || len(result.Unavailable) != 1 || result.Unavailable[0].Name != "OLD" || result.Unavailable[0].Reason != "local_proxy_unavailable" {
		t.Fatalf("bad candidates: %+v", result)
	}
	after, _ := json.Marshal(config)
	if string(before) != string(after) {
		t.Fatal("unknown historical config was modified")
	}
	payload, _ := json.Marshal(result)
	if strings.Contains(string(payload), testXrayUUID) {
		t.Fatal("selector leaked managed credentials")
	}
}

func TestNativeLocalProxyAvailabilityRefreshDoesNotDeleteConfiguration(t *testing.T) {
	setupSubscriptionTestDatabase(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	config := bo.Hysteria2ServerConfig{Outbounds: []bo.ServerConfigOutboundEntry{optionRule("INDEPENDENT", listener.Addr().String())}}
	data, _ := yaml.Marshal(config)
	if err := dao.UpdateConfig([]string{constant.Hysteria2Config}, map[string]interface{}{"value": string(data)}); err != nil {
		t.Fatal(err)
	}
	before, err := GetAccountOutboundOptions()
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Options) != 2 || before.Options[1].Source != "native" {
		t.Fatalf("independent native proxy hidden: %+v", before)
	}
	listener.Close()
	after, err := GetAccountOutboundOptions()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Options) != 1 || len(after.Unavailable) != 1 {
		t.Fatalf("closed local endpoint still selectable: %+v", after)
	}
	stored, err := GetHysteria2Config()
	if err != nil || len(stored.Outbounds) != 1 || *stored.Outbounds[0].Name != "INDEPENDENT" {
		t.Fatal("native config deleted on availability failure")
	}
}

func TestSelectorLoadsLegacySingleNodeSnapshot(t *testing.T) {
	setupSubscriptionTestDatabase(t)
	legacy := validXraySettings()
	storeNodeTestSettings(t, constant.XrayActiveConfig, legacy)
	storeNodeTestSettings(t, constant.XrayDraftConfig, legacy)
	options, err := GetAccountOutboundOptions()
	if err != nil || len(options.Options) != 2 || options.Options[1].Name != legacy.Name {
		t.Fatalf("legacy snapshot compatibility: %+v %v", options, err)
	}
}

func TestAccountOptionsDeleteRenameReaddAndPortReuse(t *testing.T) {
	old := optionSettings("OLD", 10808)
	config := bo.Hysteria2ServerConfig{Outbounds: []bo.ServerConfigOutboundEntry{optionRule("OLD", "127.0.0.1:10808")}}
	empty := bo.DefaultXraySettings()
	deleted := accountOutboundOptions(empty, empty, config, func(string) bool { return false })
	if len(deleted.Options) != 1 || deleted.Options[0].Name != "DIRECT" {
		t.Fatal("deleted node reappeared")
	}
	next := optionSettings("RENAMED", 10808)
	// Even a listening reused port does not establish ownership of an old alias.
	renamed := accountOutboundOptions(next, next, config, func(string) bool { return true })
	if len(renamed.Options) != 2 || renamed.Options[1].Name != "RENAMED" || len(renamed.Unavailable) != 1 || renamed.Unavailable[0].Reason != "local_mapping_unverified" {
		t.Fatalf("old name returned on port reuse: %+v", renamed)
	}
	readded := accountOutboundOptions(old, old, config, func(string) bool { return true })
	if len(readded.Options) != 2 || len(readded.Unavailable) != 0 {
		t.Fatalf("readd failed: %+v", readded)
	}
	// Existing RC.22 apply lifecycle can prove ownership using both name and
	// endpoint from the old snapshot; unknown private mappings are retained.
	config.Outbounds = append(config.Outbounds, optionRule("PRIVATE", "127.0.0.1:19000"))
	clean := pruneRetiredManagedOutbounds(config, old, empty)
	if len(clean.Outbounds) != 1 || *clean.Outbounds[0].Name != "PRIVATE" {
		t.Fatal("owned cleanup damaged independent config")
	}
}

func TestDraftDisabledOrUnappliedNodesDoNotFallBackThroughHY2(t *testing.T) {
	draft := optionSettings("NEW", 10810)
	active := optionSettings("DISABLED", 10808)
	active.Outbounds[0].Enabled = false
	config := bo.Hysteria2ServerConfig{Outbounds: []bo.ServerConfigOutboundEntry{optionRule("NEW", "127.0.0.1:10810"), optionRule("DISABLED", "127.0.0.1:10808")}}
	result := accountOutboundOptions(active, draft, config, func(string) bool { return true })
	if len(result.Options) != 1 || len(result.Unavailable) != 2 {
		t.Fatalf("draft/disabled mapping leaked into candidates: %+v", result)
	}
}

func TestOptionsAPIRefreshesAppliedInventoryAndPreservesBoundAccounts(t *testing.T) {
	id, _ := setupSubscriptionTestDatabase(t)
	if err := dao.UpdateAccount([]int64{id}, map[string]interface{}{"outbound": "OLD"}); err != nil {
		t.Fatal(err)
	}
	old := optionSettings("OLD", 10808)
	next := optionSettings("NEW", 10810)
	storeNodeTestSettings(t, constant.XrayActiveConfig, old)
	storeNodeTestSettings(t, constant.XrayDraftConfig, old)
	first, err := GetAccountOutboundOptions()
	if err != nil {
		t.Fatal(err)
	}
	if first.Options[1].Name != "OLD" {
		t.Fatal("initial applied node missing")
	}
	storeNodeTestSettings(t, constant.XrayActiveConfig, next)
	storeNodeTestSettings(t, constant.XrayDraftConfig, next)
	second, err := GetAccountOutboundOptions()
	if err != nil {
		t.Fatal(err)
	}
	if second.Options[1].Name != "NEW" {
		t.Fatal("stale cached inventory")
	}
	account, err := dao.GetAccount("id = ?", id)
	if err != nil || *account.Outbound != "OLD" {
		t.Fatal("unavailable binding was rewritten")
	}
	// Re-reading models a reload/upgrade: no new schema or required metadata.
	third, err := GetAccountOutboundOptions()
	if err != nil || third.Options[1].Name != "NEW" {
		t.Fatal("reload compatibility failed")
	}
}
