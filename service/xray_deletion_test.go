package service

import (
	"encoding/json"
	"errors"
	"gopkg.in/yaml.v3"
	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/constant"
	"h-ui/model/entity"
	"os"
	"path/filepath"
	"testing"
)

func storeNodeTestSettings(t *testing.T, key string, settings bo.XraySettings) {
	t.Helper()
	data, _ := json.Marshal(settings)
	if err := dao.UpdateConfig([]string{key}, map[string]interface{}{"value": string(data)}); err != nil {
		t.Fatal(err)
	}
}

func TestRetiredNodeRulesAreNotAccountReferences(t *testing.T) {
	id, _ := setupSubscriptionTestDatabase(t)
	if err := dao.UpdateAccount([]int64{id}, map[string]interface{}{"outbound": "DIRECT"}); err != nil {
		t.Fatal(err)
	}
	old := normalizeXraySettings(validXraySettings())
	empty := bo.DefaultXraySettings()
	config, err := GetHysteria2Config()
	if err != nil {
		t.Fatal(err)
	}
	config, err = MergeManagedOutboundNodes(config, old)
	if err != nil {
		t.Fatal(err)
	}
	bytes, _ := yaml.Marshal(config)
	if err := dao.UpdateConfig([]string{constant.Hysteria2Config}, map[string]interface{}{"value": string(bytes)}); err != nil {
		t.Fatal(err)
	}
	storeNodeTestSettings(t, constant.XrayDraftConfig, old)
	storeNodeTestSettings(t, constant.XrayActiveConfig, old)
	if err := SaveXrayDraft(empty); err != nil {
		t.Fatalf("last unreferenced node: %v", err)
	}
	maskedRestore := old
	maskedRestore.Outbounds = append([]bo.XrayManagedOutbound(nil), old.Outbounds...)
	maskedRestore.Outbounds[0].UUID = ""
	if err := SaveXrayDraft(maskedRestore); err != nil {
		t.Fatalf("masked row restore lost active credentials: %v", err)
	}
	restored, _ := loadXraySettings(constant.XrayDraftConfig)
	if restored.Outbounds[0].UUID != old.Outbounds[0].UUID {
		t.Fatal("active credential was not retained")
	}
	if err := SaveXrayDraft(empty); err != nil {
		t.Fatal(err)
	}
	cleaned := pruneRetiredManagedOutbounds(config, old, empty)
	for _, rule := range cleaned.Outbounds {
		if rule.Name != nil && *rule.Name == old.Outbounds[0].Name {
			t.Fatal("retired generated mapping retained")
		}
	}
	direct, err := PrepareHysteria2OutboundConfigWithAccounts(cleaned, []entity.Account{testRoutingAccount("direct-user", "DIRECT", 0, "user")})
	if err != nil || direct.UserOutbounds["direct-user"] != "DIRECT" {
		t.Fatalf("DIRECT broken with no Xray nodes: %v", err)
	}
	// An alias sharing the socket is a real reference only if an account uses it.
	alias := "legacy-alias"
	aliasRule := bo.ServerConfigOutboundEntry{Name: &alias, Type: oldRuleType(), SOCKS5: &bo.ServerConfigOutboundSOCKS5{Addr: strPointer("127.0.0.1:10808")}}
	config.Outbounds = append(config.Outbounds, aliasRule)
	bytes, _ = yaml.Marshal(config)
	if err := dao.UpdateConfig([]string{constant.Hysteria2Config}, map[string]interface{}{"value": string(bytes)}); err != nil {
		t.Fatal(err)
	}
	if err := validateXrayOutboundReferences(old, empty); err != nil {
		t.Fatalf("unused alias wrongly blocked: %v", err)
	}
	if err := dao.UpdateAccount([]int64{id}, map[string]interface{}{"outbound": alias}); err != nil {
		t.Fatal(err)
	}
	if err := validateXrayOutboundReferences(old, empty); err == nil {
		t.Fatal("account-bound alias deleted")
	}
	storeNodeTestSettings(t, constant.XrayDraftConfig, old)
	if err := SaveXrayDraft(empty); err == nil {
		t.Fatal("save API bypassed bound alias protection")
	}
	stored, err := loadXraySettings(constant.XrayDraftConfig)
	if err != nil || len(stored.Outbounds) != 1 {
		t.Fatal("failed save changed stored nodes")
	}
	// A different node reusing the same port must not evade the reference check.
	replacement := normalizeXraySettings(validXraySettings())
	replacement.Outbounds[0].ID = "different-node"
	replacement.Outbounds[0].Name = "different-name"
	if err := validateXrayOutboundReferences(old, replacement); err == nil {
		t.Fatal("port reuse rewired a referenced alias")
	}
}

func strPointer(s string) *string { return &s }
func oldRuleType() *string        { return strPointer("socks5") }

func TestNodeRenamePrunesOnlyOwnedRulesAndCanBeReadded(t *testing.T) {
	setupSubscriptionTestDatabase(t)
	old := normalizeXraySettings(validXraySettings())
	next := normalizeXraySettings(validXraySettings())
	next.Outbounds[0].Name = "RENAMED"
	config, err := MergeManagedOutboundNodes(bo.Hysteria2ServerConfig{}, old)
	if err != nil {
		t.Fatal(err)
	}
	alias := config.Outbounds[0]
	for _, r := range config.Outbounds {
		if r.Name != nil && *r.Name == old.Outbounds[0].Name {
			alias = r
		}
	}
	alias.Name = strPointer("UNRELATED_LEGACY")
	config.Outbounds = append(config.Outbounds, alias)
	cleaned := pruneRetiredManagedOutbounds(config, old, next)
	cleaned, err = MergeManagedOutboundNodes(cleaned, next)
	if err != nil {
		t.Fatal(err)
	}
	names, _ := outboundNames(cleaned.Outbounds)
	if names["test_vless"] != "" || names["renamed"] != "RENAMED" || names["unrelated_legacy"] != "UNRELATED_LEGACY" {
		t.Fatalf("bad cleanup: %v", names)
	}
	if err := SaveXrayDraft(bo.DefaultXraySettings()); err != nil {
		t.Fatal(err)
	}
	if err := SaveXrayDraft(next); err != nil {
		t.Fatalf("re-add: %v", err)
	}
}

func TestEmptyNodeApplyStopsXrayAndRollsBackOnHY2Failure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "success", true: "hy2-failure"}[fail], func(t *testing.T) {
			setupSubscriptionTestDatabase(t)
			previous := normalizeXraySettings(validXraySettings())
			empty := bo.DefaultXraySettings()
			storeNodeTestSettings(t, constant.XrayActiveConfig, previous)
			storeNodeTestSettings(t, constant.XrayDraftConfig, empty)
			hy2, _ := GetHysteria2Config()
			hy2, err := MergeManagedOutboundNodes(hy2, previous)
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := yaml.Marshal(hy2)
			if err := dao.UpdateConfig([]string{constant.Hysteria2Config}, map[string]interface{}{"value": string(encoded)}); err != nil {
				t.Fatal(err)
			}
			oldPaths, oldFactory, oldApply := managedXrayPaths, xrayRuntimeFactory, applyManagedHysteria2Config
			t.Cleanup(func() {
				managedXrayPaths = oldPaths
				xrayRuntimeFactory = oldFactory
				applyManagedHysteria2Config = oldApply
			})
			managedXrayPaths = testXrayPaths(t.TempDir())
			versionDir := filepath.Join(managedXrayPaths.versionsDir, previous.Version)
			if err := os.MkdirAll(versionDir, 0750); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(versionDir, managedXrayPaths.currentLink); err != nil {
				t.Fatal(err)
			}
			oldData, _ := GenerateXrayConfig(previous)
			if err := os.WriteFile(managedXrayPaths.config, oldData, 0640); err != nil {
				t.Fatal(err)
			}
			runtime := &fakeXrayRuntime{running: true}
			xrayRuntimeFactory = func() xrayRuntime { return runtime }
			applyManagedHysteria2Config = func(config bo.Hysteria2ServerConfig) error {
				for _, rule := range config.Outbounds {
					if rule.Name != nil && *rule.Name == previous.Outbounds[0].Name {
						t.Fatal("deleted node remained in HY2 apply payload")
					}
				}
				if fail {
					return errors.New("injected HY2 failure")
				}
				return nil
			}
			err = ApplyXrayDraft()
			if fail {
				if err == nil || !runtime.running {
					t.Fatalf("rollback missing: %v", err)
				}
			} else if err != nil || runtime.running || runtime.ensureCalls != 0 || runtime.readyCalls != 0 {
				t.Fatalf("zero-node apply: %v %+v", err, runtime)
			}
			active, _ := loadXraySettings(constant.XrayActiveConfig)
			if fail && len(active.Outbounds) != 1 || !fail && len(active.Outbounds) != 0 {
				t.Fatalf("active snapshot inconsistent: %+v", active)
			}
			data, err := os.ReadFile(managedXrayPaths.config)
			if err != nil {
				t.Fatal(err)
			}
			var generated bo.XrayConfig
			if err := json.Unmarshal(data, &generated); err != nil {
				t.Fatal(err)
			}
			if !fail && (len(generated.Inbounds) != 0 || len(generated.Outbounds) != 1 || generated.Outbounds[0].Protocol != "blackhole") {
				t.Fatal("invalid empty Xray config")
			}
		})
	}
}
