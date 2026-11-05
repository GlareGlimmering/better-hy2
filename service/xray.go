package service

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"h-ui/dao"
	"h-ui/model/bo"
	"h-ui/model/constant"
	"h-ui/model/vo"
	"h-ui/util"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	legacyXrayOutboundName = "VLESS"
	xrayTCPTestHost        = "api.ipify.org"
	xrayTCPTestPort        = 443
	xrayUDPTestHost        = "1.1.1.1"
	xrayUDPTestPort        = 53
	xrayManifestName       = "install-manifest.json"
)

var xrayOperationMutex sync.Mutex
var xrayApplyStatusMutex sync.RWMutex
var xrayLastAppliedAt int64
var xrayLastApplyError string

type xrayInstallManifest struct {
	Version       string `json:"version"`
	BinarySHA256  string `json:"binarySha256"`
	ArchiveSHA256 string `json:"archiveSha256"`
}

type xrayPaths struct {
	dataDir     string
	versionsDir string
	currentLink string
	config      string
	previous    string
	candidate   string
	certDir     string
	ca          string
	systemdUnit string
}

var managedXrayPaths = xrayPaths{
	dataDir:     constant.XrayDataDir,
	versionsDir: constant.XrayVersionsDir,
	currentLink: constant.XrayCurrentLink,
	config:      constant.XrayConfigPath,
	previous:    constant.XrayPreviousPath,
	candidate:   constant.XrayCandidatePath,
	certDir:     constant.XrayCertDir,
	ca:          constant.XrayCAPath,
	systemdUnit: constant.XraySystemdUnitPath,
}

var xrayHTTPClient = &http.Client{Timeout: 2 * time.Minute}
var xrayCommand = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}
var xrayRuntimeFactory = func() xrayRuntime { return systemdXrayRuntime{} }
var applyManagedHysteria2Config = ApplyHysteria2Config

func GetXraySettings() (vo.XraySettingsVo, error) {
	settings, err := loadXraySettings(constant.XrayDraftConfig)
	if err != nil {
		return vo.XraySettingsVo{}, err
	}
	return maskedXraySettings(settings), nil
}

// ManagedXrayKernelRequired keeps fresh/DIRECT-only installations free of Xray
// downloads. Only an applied configuration can require a background install.
func ManagedXrayKernelRequired() (bool, error) {
	settings, err := loadXraySettings(constant.XrayActiveConfig)
	if err != nil {
		return false, err
	}
	return hasXrayAdapters(settings), nil
}

func SaveXrayDraft(input bo.XraySettings) error {
	xrayOperationMutex.Lock()
	defer xrayOperationMutex.Unlock()
	input = normalizeXraySettings(input)
	current, err := loadXraySettings(constant.XrayDraftConfig)
	if err != nil {
		return err
	}
	active, err := loadXraySettings(constant.XrayActiveConfig)
	if err != nil {
		return err
	}
	currentByID := make(map[string]bo.XrayManagedOutbound, len(current.Outbounds))
	// A failed deletion can restore a masked UI row after it has disappeared
	// from the draft; retain its credentials from the last applied snapshot.
	for _, outbound := range active.Outbounds {
		currentByID[outbound.ID] = outbound
	}
	for _, outbound := range current.Outbounds {
		currentByID[outbound.ID] = outbound
	}
	for index := range input.Outbounds {
		if input.Outbounds[index].ID == "" {
			input.Outbounds[index].ID = uuid.NewString()
		}
		if strings.TrimSpace(input.Outbounds[index].UUID) == "" {
			input.Outbounds[index].UUID = currentByID[input.Outbounds[index].ID].UUID
		}
		if input.Outbounds[index].Username == "" {
			input.Outbounds[index].Password = ""
		} else if input.Outbounds[index].Password == "" {
			input.Outbounds[index].Password = currentByID[input.Outbounds[index].ID].Password
		}
		if existing, ok := currentByID[input.Outbounds[index].ID]; ok && existing.OutboundTag != "" {
			input.Outbounds[index].OutboundTag = existing.OutboundTag
		} else if input.Outbounds[index].OutboundTag == "" {
			input.Outbounds[index].OutboundTag = "hui-out-" + input.Outbounds[index].ID
		}
	}
	if err := validateXraySettings(input, false); err != nil {
		return err
	}
	if err := validateXrayOutboundReferences(current, input); err != nil {
		return err
	}
	if err := validateXrayOutboundReferences(active, input); err != nil {
		return err
	}
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return dao.UpdateConfig([]string{constant.XrayDraftConfig}, map[string]interface{}{"value": string(data)})
}

func loadXraySettings(key string) (bo.XraySettings, error) {
	config, err := dao.GetConfig("key = ?", key)
	if err != nil {
		return bo.XraySettings{}, err
	}
	settings := bo.DefaultXraySettings()
	if config.Value == nil || strings.TrimSpace(*config.Value) == "" {
		return settings, nil
	}
	if err := json.Unmarshal([]byte(*config.Value), &settings); err != nil {
		return settings, fmt.Errorf("decode Xray settings: %w", err)
	}
	return normalizeXraySettings(settings), nil
}

func maskedXraySettings(settings bo.XraySettings) vo.XraySettingsVo {
	settings = normalizeXraySettings(settings)
	result := vo.XraySettingsVo{Version: settings.Version, Outbounds: make([]vo.XrayManagedOutboundVo, 0, len(settings.Outbounds))}
	for _, outbound := range settings.Outbounds {
		masked := ""
		if outbound.UUID != "" {
			masked = "********"
		}
		result.Outbounds = append(result.Outbounds, vo.XrayManagedOutboundVo{
			ID: outbound.ID, Name: outbound.Name, Protocol: outbound.Protocol, Enabled: outbound.Enabled,
			Address: outbound.Address, Port: outbound.Port, UUID: masked, UUIDSet: outbound.UUID != "",
			Username: outbound.Username, PasswordSet: outbound.Password != "",
			TCPSupported: true, UDPSupported: outbound.Protocol != "http",
			Network: outbound.Network, TLSSNI: outbound.TLSSNI, UseCustomCA: outbound.UseCustomCA,
			SOCKSListen: outbound.SOCKSListen, SOCKSPort: outbound.SOCKSPort, OutboundTag: outbound.OutboundTag,
		})
	}
	return result
}

func validateXraySettings(settings bo.XraySettings, requireInstalled bool) error {
	settings = normalizeXraySettings(settings)
	if !regexp.MustCompile(`^v[0-9]+(?:\.[0-9A-Za-z-]+)+$`).MatchString(settings.Version) {
		return errors.New("Xray version must be an explicit v-prefixed release tag")
	}
	names := map[string]bool{}
	ports := map[uint16]bool{}
	ids := map[string]bool{}
	tags := map[string]bool{}
	enabled := 0
	for _, outbound := range settings.Outbounds {
		if outbound.ID == "" || ids[outbound.ID] {
			return errors.New("Xray outbound IDs must be present and unique")
		}
		ids[outbound.ID] = true
		nameKey := strings.ToLower(strings.TrimSpace(outbound.Name))
		if nameKey == "" || names[nameKey] {
			return fmt.Errorf("duplicate or empty Xray outbound name %q", outbound.Name)
		}
		names[nameKey] = true
		usesAdapter := outbound.Protocol == "vless" || outbound.Protocol == "http"
		if usesAdapter {
			if outbound.SOCKSListen != constant.XraySOCKSListen {
				return fmt.Errorf("outbound %q SOCKS5 adapter must listen on %s", outbound.Name, constant.XraySOCKSListen)
			}
			if outbound.SOCKSPort == 0 || ports[outbound.SOCKSPort] {
				return fmt.Errorf("outbound %q has an invalid or duplicate SOCKS5 port for its adapter", outbound.Name)
			}
			ports[outbound.SOCKSPort] = true
			if outbound.OutboundTag == "" || tags[outbound.OutboundTag] {
				return fmt.Errorf("outbound %q has an empty or duplicate Xray tag", outbound.Name)
			}
			tags[outbound.OutboundTag] = true
		}
		if !validHost(outbound.Address) || outbound.Port == 0 {
			return fmt.Errorf("outbound %q has an invalid proxy server", outbound.Name)
		}
		switch outbound.Protocol {
		case "vless":
			if outbound.Network != "tcp" {
				return fmt.Errorf("outbound %q only supports VLESS TCP TLS", outbound.Name)
			}
			if _, err := uuid.Parse(outbound.UUID); err != nil {
				return fmt.Errorf("outbound %q has an invalid VLESS UUID", outbound.Name)
			}
			if !validDomain(outbound.TLSSNI) {
				return fmt.Errorf("outbound %q has an invalid TLS SNI", outbound.Name)
			}
		case "http", "socks5":
			if (outbound.Username == "") != (outbound.Password == "") {
				return fmt.Errorf("outbound %q proxy username and password must be provided together", outbound.Name)
			}
			if outbound.Protocol == "socks5" && (len(outbound.Username) > 255 || len(outbound.Password) > 255) {
				return fmt.Errorf("outbound %q SOCKS5 credentials exceed protocol limits", outbound.Name)
			}
		default:
			return fmt.Errorf("outbound %q has unsupported protocol %q", outbound.Name, outbound.Protocol)
		}
		if outbound.Enabled {
			enabled++
		}
	}
	if enabled == 0 && len(settings.Outbounds) > 0 {
		return errors.New("at least one outbound node must be enabled")
	}
	if requireInstalled && hasXrayAdapters(settings) {
		if err := verifyInstalledXrayVersion(managedXrayPaths, settings.Version); err != nil {
			return fmt.Errorf("Xray version %s failed verification: %w", settings.Version, err)
		}
	}
	return nil
}

func hasXrayAdapters(settings bo.XraySettings) bool {
	for _, node := range normalizeXraySettings(settings).Outbounds {
		if node.Enabled && (node.Protocol == "vless" || node.Protocol == "http") {
			return true
		}
	}
	return false
}

func normalizeXraySettings(settings bo.XraySettings) bo.XraySettings {
	if strings.TrimSpace(settings.Version) == "" {
		settings.Version = constant.XrayPinnedVersion
	}
	if len(settings.Outbounds) == 0 && (settings.Address != "" || settings.UUID != "") {
		name := settings.Name
		if name == "" {
			name = legacyXrayOutboundName
		}
		settings.Outbounds = []bo.XrayManagedOutbound{{
			ID: "legacy-vless", Name: name, Protocol: "vless", Enabled: true, Address: settings.Address,
			Port: settings.Port, UUID: settings.UUID, Network: "tcp", TLSSNI: settings.TLSSNI,
			UseCustomCA: settings.UseCustomCA, SOCKSListen: constant.XraySOCKSListen,
			SOCKSPort: constant.XraySOCKSPort, OutboundTag: "hui-out-legacy-vless",
		}}
	}
	for index := range settings.Outbounds {
		outbound := &settings.Outbounds[index]
		outbound.Name = strings.TrimSpace(outbound.Name)
		outbound.Address = strings.TrimSpace(outbound.Address)
		outbound.Protocol = strings.ToLower(strings.TrimSpace(outbound.Protocol))
		outbound.Username = strings.TrimSpace(outbound.Username)
		outbound.UUID = strings.TrimSpace(outbound.UUID)
		outbound.TLSSNI = strings.TrimSpace(outbound.TLSSNI)
		if outbound.Network == "" {
			outbound.Network = "tcp"
		}
		if outbound.Protocol == "" {
			outbound.Protocol = "vless"
		}
		if outbound.SOCKSListen == "" {
			outbound.SOCKSListen = constant.XraySOCKSListen
		}
	}
	settings.Name, settings.Address, settings.Port, settings.UUID, settings.TLSSNI, settings.UseCustomCA = "", "", 0, "", "", false
	return settings
}

func validateXrayOutboundReferences(previous, settings bo.XraySettings) error {
	accounts, accountErr := dao.ListAccount("role = ? and deleted = 0", "user")
	if accountErr != nil {
		return accountErr
	}
	nextByID := make(map[string]bo.XrayManagedOutbound, len(settings.Outbounds))
	for _, outbound := range settings.Outbounds {
		nextByID[outbound.ID] = outbound
	}
	for _, old := range previous.Outbounds {
		if !OutboundInUse(old.Name, accounts) {
			continue
		}
		next, ok := nextByID[old.ID]
		if !ok || !next.Enabled {
			return fmt.Errorf("outbound node %q is still referenced by an account", old.Name)
		}
		if !strings.EqualFold(old.Name, next.Name) {
			return fmt.Errorf("outbound node %q cannot be renamed while referenced by an account", old.Name)
		}
	}
	config, err := GetHysteria2Config()
	if err != nil {
		return fmt.Errorf("inspect Hysteria2 outbound references: %w", err)
	}
	for _, outbound := range config.Outbounds {
		if outbound.Name == nil || outbound.Type == nil || !strings.EqualFold(*outbound.Type, "socks5") || outbound.SOCKS5 == nil || outbound.SOCKS5.Addr == nil {
			continue
		}
		for _, old := range previous.Outbounds {
			if !managedAdapterRule(outbound, old) {
				continue
			}
			next, exists := nextByID[old.ID]
			if exists && next.Enabled && (managedAdapterRule(outbound, next) || strings.EqualFold(*outbound.Name, next.Name)) {
				continue
			}
			if OutboundInUse(*outbound.Name, accounts) {
				return fmt.Errorf("Xray node %q is referenced by account outbound %q at %s", old.Name, *outbound.Name, *outbound.SOCKS5.Addr)
			}
			aclText := ""
			if config.ACL != nil {
				aclText = strings.Join(config.ACL.Inline, "\n")
				if config.ACL.File != nil && *config.ACL.File != "" {
					data, err := os.ReadFile(*config.ACL.File)
					if err != nil {
						return errors.New("cannot verify configured Hysteria2 ACL file references")
					}
					aclText += "\n" + string(data)
				}
			}
			if regexp.MustCompile(`(?mi)^\s*` + regexp.QuoteMeta(*outbound.Name) + `\s*\(`).MatchString(aclText) {
				return fmt.Errorf("Xray node %q has a legacy Hysteria2 rule %q with custom ACL; remove that rule from ACL before deleting the node", old.Name, *outbound.Name)
			}
		}
	}
	return nil
}

func managedAdapterRule(rule bo.ServerConfigOutboundEntry, node bo.XrayManagedOutbound) bool {
	if rule.Type == nil || !strings.EqualFold(*rule.Type, "socks5") || rule.SOCKS5 == nil || rule.SOCKS5.Addr == nil {
		return false
	}
	if node.Protocol == "socks5" {
		return *rule.SOCKS5.Addr == net.JoinHostPort(node.Address, strconv.Itoa(int(node.Port)))
	}
	if node.Protocol != "vless" && node.Protocol != "http" {
		return false
	}
	return *rule.SOCKS5.Addr == net.JoinHostPort(node.SOCKSListen, strconv.Itoa(int(node.SOCKSPort)))
}

// Remove only identifiable generated rules (name AND endpoint). Unmatched
// advanced/legacy rules remain intact, including aliases sharing an endpoint.
func pruneRetiredManagedOutbounds(config bo.Hysteria2ServerConfig, previous, next bo.XraySettings) bo.Hysteria2ServerConfig {
	keep := make(map[string]bool)
	for _, node := range next.Outbounds {
		if node.Enabled {
			keep[node.ID+"\x00"+strings.ToLower(node.Name)] = true
		}
	}
	var rules []bo.ServerConfigOutboundEntry
	for _, rule := range config.Outbounds {
		retired := false
		for _, old := range previous.Outbounds {
			if rule.Name != nil && strings.EqualFold(*rule.Name, old.Name) && managedAdapterRule(rule, old) && !keep[old.ID+"\x00"+strings.ToLower(old.Name)] {
				retired = true
				break
			}
		}
		if !retired {
			rules = append(rules, rule)
		}
	}
	config.Outbounds = rules
	return config
}

func validHost(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "/:@[]") {
		return false
	}
	return net.ParseIP(value) != nil || validDomain(value)
}

func validDomain(value string) bool {
	if len(value) == 0 || len(value) > 253 || strings.HasSuffix(value, ".") {
		return false
	}
	label := regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)
	parts := strings.Split(value, ".")
	if len(parts) < 2 {
		return false
	}
	for _, part := range parts {
		if !label.MatchString(part) {
			return false
		}
	}
	return true
}

func GenerateXrayConfig(settings bo.XraySettings) ([]byte, error) {
	settings = normalizeXraySettings(settings)
	if err := validateXraySettings(settings, false); err != nil {
		return nil, err
	}
	config := bo.XrayConfig{
		Log:      bo.XrayLog{LogLevel: "warning"},
		Inbounds: []bo.XrayInbound{},
		// The default outbound is deliberately a blackhole. Every managed
		// listener below has an explicit rule, and any mismatch fails closed.
		Outbounds: []bo.XrayOutbound{{Tag: "hui-unmatched-block", Protocol: "blackhole", Settings: map[string]interface{}{}}},
		Routing:   bo.XrayRouting{Rules: []bo.XrayRoutingRule{}},
	}
	for _, outbound := range settings.Outbounds {
		if !outbound.Enabled || outbound.Protocol == "socks5" {
			continue
		}
		inboundTag := "hui-socks-" + outbound.ID
		config.Inbounds = append(config.Inbounds, bo.XrayInbound{
			Tag: inboundTag, Listen: outbound.SOCKSListen, Port: outbound.SOCKSPort, Protocol: "socks",
			Settings: bo.XrayInboundSettings{Auth: "noauth", UDP: outbound.Protocol == "vless", IP: outbound.SOCKSListen},
		})
		if outbound.Protocol == "vless" {
			tlsSettings := bo.XrayTLSSettings{ServerName: outbound.TLSSNI, AllowInsecure: false, DisableSystemRoot: outbound.UseCustomCA}
			if outbound.UseCustomCA {
				tlsSettings.Certificates = []bo.XrayCertificate{{CertificateFile: constant.XrayCAPath, Usage: "verify"}}
			}
			config.Outbounds = append(config.Outbounds, bo.XrayOutbound{
				Tag: outbound.OutboundTag, Protocol: "vless",
				Settings:       bo.XrayVLESSSettings{Address: outbound.Address, Port: outbound.Port, ID: outbound.UUID, Encryption: "none"},
				StreamSettings: &bo.XrayStreamSettings{Network: "tcp", Security: "tls", TLSSettings: tlsSettings},
			})
		} else {
			server := bo.XrayProxyServer{Address: outbound.Address, Port: outbound.Port}
			if outbound.Username != "" {
				server.Users = []bo.XrayProxyUser{{User: outbound.Username, Pass: outbound.Password}}
			}
			config.Outbounds = append(config.Outbounds, bo.XrayOutbound{Tag: outbound.OutboundTag, Protocol: "http", Settings: bo.XrayProxySettings{Servers: []bo.XrayProxyServer{server}}})
		}
		config.Routing.Rules = append(config.Routing.Rules, bo.XrayRoutingRule{Type: "field", InboundTag: []string{inboundTag}, OutboundTag: outbound.OutboundTag})
	}
	config.Routing.Rules = append(config.Routing.Rules, bo.XrayRoutingRule{Type: "field", Network: "tcp,udp", OutboundTag: "hui-unmatched-block"})
	return json.MarshalIndent(config, "", "  ")
}

func ValidateXrayDraft() (vo.XrayValidationVo, error) {
	xrayOperationMutex.Lock()
	defer xrayOperationMutex.Unlock()

	settings, err := loadXraySettings(constant.XrayDraftConfig)
	if err != nil {
		return vo.XrayValidationVo{}, err
	}
	result, _, err := validateXraySnapshot(settings)
	return result, err
}

func validateXraySnapshot(settings bo.XraySettings) (vo.XrayValidationVo, []byte, error) {
	if err := validateXraySettings(settings, true); err != nil {
		return vo.XrayValidationVo{}, nil, err
	}
	needsCustomCA := false
	for _, outbound := range settings.Outbounds {
		needsCustomCA = needsCustomCA || (outbound.Enabled && outbound.Protocol == "vless" && outbound.UseCustomCA)
	}
	if caStatus := XrayCAStatus(); needsCustomCA && !caStatus.Valid {
		return vo.XrayValidationVo{}, nil, fmt.Errorf("custom CA is invalid: %s", caStatus.Error)
	}
	data, err := GenerateXrayConfig(settings)
	if err != nil {
		return vo.XrayValidationVo{}, nil, err
	}
	if !hasXrayAdapters(settings) {
		return vo.XrayValidationVo{Valid: true, Output: "No Xray adapter required"}, data, nil
	}
	if err := ensureXrayDirectories(managedXrayPaths); err != nil {
		return vo.XrayValidationVo{}, nil, err
	}
	if err := atomicWriteFile(managedXrayPaths.candidate, data, 0640); err != nil {
		return vo.XrayValidationVo{}, nil, err
	}
	binaryPath, _ := xrayVersionBinary(managedXrayPaths, settings.Version)
	output, err := xrayCommand(binaryPath, "run", "-test", "-c", managedXrayPaths.candidate)
	result := vo.XrayValidationVo{Valid: err == nil, Output: strings.TrimSpace(string(output))}
	if err != nil {
		return result, nil, fmt.Errorf("Xray config validation failed: %s", safeCommandOutput(output, err))
	}
	return result, data, nil
}

func XrayCAStatus() vo.XrayCAStatusVo {
	data, err := os.ReadFile(managedXrayPaths.ca)
	if err != nil {
		return vo.XrayCAStatusVo{Exists: !os.IsNotExist(err), Error: err.Error()}
	}
	status, err := validateCA(data)
	if err != nil {
		return vo.XrayCAStatusVo{Exists: true, Error: err.Error()}
	}
	return status
}

func SaveXrayCA(data []byte) (vo.XrayCAStatusVo, error) {
	status, err := validateCA(data)
	if err != nil {
		return vo.XrayCAStatusVo{}, err
	}
	if err := ensureXrayServiceAccount(); err != nil {
		return vo.XrayCAStatusVo{}, err
	}
	if err := ensureXrayDirectories(managedXrayPaths); err != nil {
		return vo.XrayCAStatusVo{}, err
	}
	if err := atomicWriteFile(managedXrayPaths.ca, data, 0640); err != nil {
		return vo.XrayCAStatusVo{}, err
	}
	if err := setXrayOwnership(managedXrayPaths.certDir); err != nil {
		return vo.XrayCAStatusVo{}, err
	}
	return status, nil
}

func validateCA(data []byte) (vo.XrayCAStatusVo, error) {
	rest := data
	var first *x509.Certificate
	count := 0
	for len(rest) > 0 {
		block, remaining := pem.Decode(rest)
		if block == nil {
			return vo.XrayCAStatusVo{}, errors.New("certificate must contain PEM data only")
		}
		rest = remaining
		if block.Type != "CERTIFICATE" {
			return vo.XrayCAStatusVo{}, fmt.Errorf("unsupported PEM block %s", block.Type)
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return vo.XrayCAStatusVo{}, fmt.Errorf("parse CA certificate: %w", err)
		}
		if !cert.BasicConstraintsValid || !cert.IsCA {
			return vo.XrayCAStatusVo{}, errors.New("uploaded certificate is not a CA certificate")
		}
		now := time.Now()
		if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
			return vo.XrayCAStatusVo{}, errors.New("CA certificate is not currently valid")
		}
		if first == nil {
			first = cert
		}
		count++
	}
	if count == 0 || first == nil {
		return vo.XrayCAStatusVo{}, errors.New("no CA certificate found")
	}
	sum := sha256.Sum256(first.Raw)
	return vo.XrayCAStatusVo{
		Exists: true, Valid: true, Subject: first.Subject.String(),
		Fingerprint: strings.ToUpper(hex.EncodeToString(sum[:])),
	}, nil
}

func ApplyXrayDraft() error {
	xrayOperationMutex.Lock()
	defer xrayOperationMutex.Unlock()

	settings, err := loadXraySettings(constant.XrayDraftConfig)
	if err != nil {
		setXrayApplyStatus(err)
		return err
	}
	previous, previousErr := loadXraySettings(constant.XrayActiveConfig)
	if previousErr != nil {
		return previousErr
	}
	if err := validateXrayOutboundReferences(previous, settings); err != nil {
		setXrayApplyStatus(err)
		return err
	}
	validation, data, err := validateXraySnapshot(settings)
	if err != nil || !validation.Valid {
		if err != nil {
			setXrayApplyStatus(err)
			return err
		}
		err = errors.New("Xray draft is not valid")
		setXrayApplyStatus(err)
		return err
	}
	wasRunning := xrayRuntimeFactory().IsRunning()
	err = applyXrayConfiguration(settings, data, managedXrayPaths, xrayRuntimeFactory())
	if err == nil {
		hy2Config, configErr := GetHysteria2Config()
		if configErr == nil {
			hy2Config = pruneRetiredManagedOutbounds(hy2Config, previous, settings)
			configErr = applyManagedHysteria2Config(hy2Config)
		}
		if configErr != nil {
			if previousErr != nil {
				err = fmt.Errorf("apply managed Hysteria2 mappings: %v; no previous Xray snapshot is available for rollback", configErr)
			} else if previousData, generateErr := GenerateXrayConfig(previous); generateErr != nil {
				err = fmt.Errorf("apply managed Hysteria2 mappings: %v; generate Xray rollback config: %v", configErr, generateErr)
			} else if rollbackErr := applyXrayConfiguration(previous, previousData, managedXrayPaths, xrayRuntimeFactory()); rollbackErr != nil {
				err = fmt.Errorf("apply managed Hysteria2 mappings: %v; Xray rollback failed: %v", configErr, rollbackErr)
			} else {
				if !wasRunning {
					if stopErr := xrayRuntimeFactory().Stop(); stopErr != nil {
						err = fmt.Errorf("apply managed Hysteria2 mappings: %v; restore stopped Xray state failed: %w", configErr, stopErr)
					} else {
						err = fmt.Errorf("apply managed Hysteria2 mappings: %w; previous Xray state restored", configErr)
					}
				} else {
					err = fmt.Errorf("apply managed Hysteria2 mappings: %w; previous Xray state restored", configErr)
				}
			}
		}
	}
	setXrayApplyStatus(err)
	return err
}

func setXrayApplyStatus(err error) {
	xrayApplyStatusMutex.Lock()
	defer xrayApplyStatusMutex.Unlock()
	if err != nil {
		xrayLastApplyError = err.Error()
		return
	}
	xrayLastApplyError = ""
	xrayLastAppliedAt = time.Now().UnixMilli()
}

type xrayRuntime interface {
	Ensure() error
	IsRunning() bool
	Start() error
	Stop() error
	Restart() error
	Ready() error
	PrepareFiles(string) error
}

func applyXrayConfiguration(settings bo.XraySettings, data []byte, paths xrayPaths, runtime xrayRuntime) error {
	needsXray := hasXrayAdapters(settings)
	if needsXray {
		if err := runtime.Ensure(); err != nil {
			return err
		}
	} else if err := os.MkdirAll(paths.dataDir, 0750); err != nil {
		return err
	}
	wasRunning := runtime.IsRunning()
	oldConfig, configErr := os.ReadFile(paths.config)
	if configErr != nil && !os.IsNotExist(configErr) {
		return fmt.Errorf("snapshot current Xray config: %w", configErr)
	}
	oldLink, linkErr := filepath.EvalSymlinks(paths.currentLink)
	if linkErr != nil && !os.IsNotExist(linkErr) {
		return fmt.Errorf("snapshot current Xray version: %w", linkErr)
	}
	if err := atomicWriteFile(paths.candidate, data, 0640); err != nil {
		return err
	}
	if configErr == nil {
		if err := atomicWriteFile(paths.previous, oldConfig, 0640); err != nil {
			return err
		}
	}
	if err := atomicWriteFile(paths.config, data, 0640); err != nil {
		return err
	}
	versionDir := filepath.Join(paths.versionsDir, settings.Version)
	if needsXray {
		if err := replaceSymlink(paths.currentLink, versionDir); err != nil {
			return rollbackXrayApplyError(err, paths, oldConfig, configErr, oldLink, linkErr, wasRunning, runtime)
		}
		if err := runtime.PrepareFiles(paths.dataDir); err != nil {
			return rollbackXrayApplyError(err, paths, oldConfig, configErr, oldLink, linkErr, wasRunning, runtime)
		}
	}
	var applyErr error
	if !needsXray {
		if wasRunning {
			applyErr = runtime.Stop()
		}
		if applyErr == nil && runtime.IsRunning() {
			applyErr = errors.New("Xray remained running with no adapter nodes")
		}
	} else if wasRunning {
		applyErr = runtime.Restart()
	} else {
		applyErr = runtime.Start()
	}
	if applyErr == nil && needsXray {
		applyErr = runtime.Ready()
	}
	if applyErr != nil {
		return rollbackXrayApplyError(applyErr, paths, oldConfig, configErr, oldLink, linkErr, wasRunning, runtime)
	}
	serialized, _ := json.Marshal(settings)
	if err := dao.UpdateConfig([]string{constant.XrayActiveConfig}, map[string]interface{}{"value": string(serialized)}); err != nil {
		rollbackErr := restoreXrayState(paths, oldConfig, configErr, oldLink, linkErr, wasRunning, runtime)
		if rollbackErr != nil {
			return fmt.Errorf("persist active Xray settings failed: %v; rollback failed: %v", err, rollbackErr)
		}
		return err
	}
	return nil
}

func rollbackXrayApplyError(applyErr error, paths xrayPaths, oldConfig []byte, configErr error, oldLink string, linkErr error, wasRunning bool, runtime xrayRuntime) error {
	rollbackErr := restoreXrayState(paths, oldConfig, configErr, oldLink, linkErr, wasRunning, runtime)
	if rollbackErr != nil {
		return fmt.Errorf("apply Xray failed: %v; rollback failed: %v", applyErr, rollbackErr)
	}
	return fmt.Errorf("apply Xray failed and previous state was restored: %w", applyErr)
}

func restoreXrayState(paths xrayPaths, oldConfig []byte, configErr error, oldLink string, linkErr error, wasRunning bool, runtime xrayRuntime) error {
	var failures []string
	if configErr == nil {
		if err := atomicWriteFile(paths.config, oldConfig, 0640); err != nil {
			failures = append(failures, err.Error())
		}
	} else if os.IsNotExist(configErr) {
		if err := os.Remove(paths.config); err != nil && !os.IsNotExist(err) {
			failures = append(failures, err.Error())
		}
	}
	if linkErr == nil {
		if err := replaceSymlink(paths.currentLink, oldLink); err != nil {
			failures = append(failures, err.Error())
		}
	} else {
		if err := os.Remove(paths.currentLink); err != nil && !os.IsNotExist(err) {
			failures = append(failures, err.Error())
		}
	}
	if err := runtime.PrepareFiles(paths.dataDir); err != nil {
		failures = append(failures, err.Error())
	}
	var err error
	if wasRunning {
		err = runtime.Restart()
		if err == nil {
			err = runtime.Ready()
		}
	} else {
		err = runtime.Stop()
		if err == nil && runtime.IsRunning() {
			err = errors.New("Xray remained running after rollback")
		}
	}
	if err != nil {
		failures = append(failures, err.Error())
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func StartXray() error {
	xrayOperationMutex.Lock()
	defer xrayOperationMutex.Unlock()
	if err := validateActiveXrayRuntime(managedXrayPaths); err != nil {
		return err
	}
	runtime := xrayRuntimeFactory()
	if err := runtime.Start(); err != nil {
		return err
	}
	if err := runtime.Ready(); err != nil {
		if stopErr := runtime.Stop(); stopErr != nil {
			return fmt.Errorf("Xray start readiness failed: %v; stop failed: %v", err, stopErr)
		}
		return fmt.Errorf("Xray start readiness failed; service stopped: %w", err)
	}
	return nil
}

func StopXray() error {
	xrayOperationMutex.Lock()
	defer xrayOperationMutex.Unlock()
	if !fileExists(managedXrayPaths.systemdUnit) {
		return errors.New("Xray systemd service is not installed")
	}
	return xrayRuntimeFactory().Stop()
}

func RestartXray() error {
	xrayOperationMutex.Lock()
	defer xrayOperationMutex.Unlock()
	if err := validateActiveXrayRuntime(managedXrayPaths); err != nil {
		return err
	}
	runtime := xrayRuntimeFactory()
	if err := runtime.Restart(); err != nil {
		return err
	}
	if err := runtime.Ready(); err != nil {
		if stopErr := runtime.Stop(); stopErr != nil {
			return fmt.Errorf("Xray restart readiness failed: %v; stop failed: %v", err, stopErr)
		}
		return fmt.Errorf("Xray restart readiness failed; service stopped: %w", err)
	}
	return nil
}

func validateActiveXrayRuntime(paths xrayPaths) error {
	if !fileExists(paths.systemdUnit) {
		return errors.New("Xray systemd service is not installed; install the selected Xray version first")
	}
	settings, err := loadXraySettings(constant.XrayActiveConfig)
	if err != nil || strings.TrimSpace(settings.Version) == "" {
		return errors.New("Xray configuration has not been applied")
	}
	if !hasXrayAdapters(settings) {
		return errors.New("no enabled Xray adapter nodes; Xray is not required")
	}
	if err := verifyInstalledXrayVersion(paths, settings.Version); err != nil {
		return fmt.Errorf("Xray version %s is not installed or failed verification: %w", settings.Version, err)
	}
	target, err := filepath.EvalSymlinks(paths.currentLink)
	if err != nil {
		return fmt.Errorf("Xray active version link is missing: %w", err)
	}
	expected := filepath.Join(paths.versionsDir, settings.Version)
	if filepath.Clean(target) != filepath.Clean(expected) {
		return fmt.Errorf("Xray active version link does not match %s", settings.Version)
	}
	if !fileExists(paths.config) {
		return errors.New("Xray active configuration file is missing")
	}
	return nil
}

func GetXrayStatus() vo.XrayStatusVo {
	status := vo.XrayStatusVo{CA: XrayCAStatus(), InstalledVersions: installedXrayVersions(managedXrayPaths)}
	status.ServiceInstalled = fileExists(managedXrayPaths.systemdUnit)
	status.Running = status.ServiceInstalled && (systemdXrayRuntime{}).IsRunning()
	if target, err := filepath.EvalSymlinks(managedXrayPaths.currentLink); err == nil {
		status.ActiveVersion = filepath.Base(target)
	}
	targetVersion := status.ActiveVersion
	if targetVersion == "" {
		if draft, err := loadXraySettings(constant.XrayDraftConfig); err == nil {
			targetVersion = strings.TrimSpace(draft.Version)
		}
	}
	if targetVersion != "" {
		status.VersionInstalled = verifyInstalledXrayVersion(managedXrayPaths, targetVersion) == nil
	}
	if status.ActiveVersion != "" && fileExists(managedXrayPaths.config) && status.VersionInstalled {
		binaryPath, _ := xrayVersionBinary(managedXrayPaths, status.ActiveVersion)
		_, err := xrayCommand(binaryPath, "run", "-test", "-c", managedXrayPaths.config)
		status.ConfigValid = err == nil
		if err != nil {
			status.Error = "active Xray configuration failed static validation"
		}
	}
	status.RuntimeReady = status.VersionInstalled && status.ServiceInstalled && status.ActiveVersion != "" && status.ConfigValid
	switch {
	case !status.VersionInstalled:
		status.RuntimeState = "not_installed"
		if targetVersion == "" {
			status.Error = "Xray is not installed; select an explicit version and install it"
		} else {
			status.Error = fmt.Sprintf("Xray version %s is not installed or failed verification", targetVersion)
		}
	case !status.ServiceInstalled:
		status.RuntimeState = "service_missing"
		status.Error = "Xray systemd service is not installed"
	case !status.RuntimeReady:
		status.RuntimeState = "not_applied"
		if status.Error == "" {
			status.Error = "Xray configuration has not been applied"
		}
	case status.Running:
		status.RuntimeState = "running"
	default:
		status.RuntimeState = "stopped"
	}
	if draft, draftErr := loadXraySettings(constant.XrayDraftConfig); draftErr == nil {
		if active, activeErr := loadXraySettings(constant.XrayActiveConfig); activeErr == nil {
			if !hasXrayAdapters(active) && !hasXrayAdapters(draft) && !status.Running {
				status.RuntimeState = "not_required"
				status.Error = ""
			}
			draftJSON, _ := json.Marshal(normalizeXraySettings(draft))
			activeJSON, _ := json.Marshal(normalizeXraySettings(active))
			status.PendingChanges = !bytes.Equal(draftJSON, activeJSON)
		}
	}
	xrayApplyStatusMutex.RLock()
	status.LastAppliedAt = xrayLastAppliedAt
	status.LastApplyError = xrayLastApplyError
	xrayApplyStatusMutex.RUnlock()
	return status
}

func TestXrayOutbound(id, protocol string) vo.XrayTestVo {
	result := vo.XrayTestVo{Protocol: protocol, Supported: protocol == "tcp" || protocol == "udp", TestedAt: time.Now().UnixMilli()}
	settings, outbound, err := activeXrayOutbound(id)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if outbound.Protocol == "http" && protocol == "udp" {
		result.Supported = false
		result.Error = "HTTP proxy nodes do not support UDP"
		return result
	}
	proxyAddress := net.JoinHostPort(outbound.SOCKSListen, strconv.Itoa(int(outbound.SOCKSPort)))
	verify := func() error { return verifyActiveManagedXray(settings, outbound) }
	tcpTest := testXrayTCPViaSOCKS
	udpTest := socks5UDPQuery
	if outbound.Protocol == "socks5" {
		proxyAddress = net.JoinHostPort(outbound.Address, strconv.Itoa(int(outbound.Port)))
		verify = func() error {
			return probeSocks5Auth(proxyAddress, outbound.Username, outbound.Password, 5*time.Second)
		}
		tcpTest = func(address string, timeout time.Duration) (string, error) {
			return testTCPViaSOCKSAuth(address, outbound.Username, outbound.Password, timeout)
		}
		udpTest = func(address string, timeout time.Duration) error {
			return socks5UDPQueryAuth(address, outbound.Username, outbound.Password, timeout)
		}
	}
	return runXrayOutboundTest(result, proxyAddress, verify, tcpTest, udpTest)
}

func runXrayOutboundTest(result vo.XrayTestVo, proxyAddress string, verify func() error, tcpTest func(string, time.Duration) (string, error), udpTest func(string, time.Duration) error) vo.XrayTestVo {
	start := time.Now()
	if !result.Supported {
		result.Error = "unsupported connectivity test protocol"
		return result
	}
	if err := verify(); err != nil {
		result.Error = err.Error()
		return result
	}
	result.LocalProxyAvailable = true
	if result.Protocol == "udp" {
		if err := udpTest(proxyAddress, 10*time.Second); err != nil {
			result.Error = err.Error()
			return result
		}
		result.RemoteConnectivity = true
		result.LatencyMs = time.Since(start).Milliseconds()
		return result
	}
	exitIP, err := tcpTest(proxyAddress, 10*time.Second)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.RemoteConnectivity = true
	result.ExitIP = exitIP
	result.ExitIdentityVerified = net.ParseIP(exitIP) != nil
	result.LatencyMs = time.Since(start).Milliseconds()
	return result
}

func testXrayTCPViaSOCKS(proxyAddress string, timeout time.Duration) (string, error) {
	return testTCPViaSOCKSAuth(proxyAddress, "", "", timeout)
}

func testTCPViaSOCKSAuth(proxyAddress, username, password string, timeout time.Duration) (string, error) {
	conn, err := socks5TCPConnectAuth(proxyAddress, xrayTCPTestHost, xrayTCPTestPort, username, password, timeout)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	tlsConn := tls.Client(conn, &tls.Config{ServerName: xrayTCPTestHost, MinVersion: tls.VersionTLS12})
	_ = tlsConn.SetDeadline(time.Now().Add(timeout))
	if err := tlsConn.Handshake(); err != nil {
		return "", fmt.Errorf("TLS through SOCKS failed: %w", err)
	}
	if _, err := fmt.Fprintf(tlsConn, "GET /?format=text HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", xrayTCPTestHost); err != nil {
		return "", fmt.Errorf("request exit identity through SOCKS: %w", err)
	}
	response, err := http.ReadResponse(bufio.NewReader(tlsConn), nil)
	if err != nil {
		return "", fmt.Errorf("read exit identity through SOCKS: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("exit identity service returned HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 128))
	if err != nil {
		return "", err
	}
	exitIP := strings.TrimSpace(string(body))
	if net.ParseIP(exitIP) == nil {
		return "", errors.New("exit identity service returned an invalid IP address")
	}
	return exitIP, nil
}

func activeXrayOutbound(id string) (bo.XraySettings, bo.XrayManagedOutbound, error) {
	settings, err := loadXraySettings(constant.XrayActiveConfig)
	if err != nil {
		return settings, bo.XrayManagedOutbound{}, fmt.Errorf("load active Xray settings: %w", err)
	}
	for _, outbound := range settings.Outbounds {
		if outbound.ID == id {
			if !outbound.Enabled {
				return settings, outbound, errors.New("Xray outbound is disabled")
			}
			return settings, outbound, nil
		}
	}
	return settings, bo.XrayManagedOutbound{}, errors.New("Xray outbound is not present in the applied configuration")
}

func verifyActiveManagedXray(settings bo.XraySettings, outbound bo.XrayManagedOutbound) error {
	if err := validateXraySettings(settings, true); err != nil {
		return err
	}
	expected, err := GenerateXrayConfig(settings)
	if err != nil {
		return err
	}
	actual, err := os.ReadFile(managedXrayPaths.config)
	if err != nil {
		return fmt.Errorf("read active Xray config: %w", err)
	}
	if !bytes.Equal(actual, expected) {
		return errors.New("active Xray config does not match the applied managed snapshot")
	}
	target, err := filepath.EvalSymlinks(managedXrayPaths.currentLink)
	if err != nil {
		return fmt.Errorf("resolve active Xray version: %w", err)
	}
	expectedTarget := filepath.Join(managedXrayPaths.versionsDir, settings.Version)
	if filepath.Clean(target) != filepath.Clean(expectedTarget) {
		return errors.New("active Xray binary version does not match the applied managed snapshot")
	}
	return verifyManagedXrayListener(outbound.SOCKSListen, outbound.SOCKSPort)
}

func waitForManagedXray(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		if err := verifyConfiguredXrayListeners(); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("managed Xray SOCKS listener did not become ready: %w", lastErr)
}

func verifyConfiguredXrayListeners() error {
	data, err := os.ReadFile(managedXrayPaths.config)
	if err != nil {
		return fmt.Errorf("read managed Xray config: %w", err)
	}
	var config bo.XrayConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("decode managed Xray config: %w", err)
	}
	if len(config.Inbounds) == 0 {
		return nil
	}
	for _, inbound := range config.Inbounds {
		if err := verifyManagedXrayListener(inbound.Listen, inbound.Port); err != nil {
			return fmt.Errorf("listener %s:%d: %w", inbound.Listen, inbound.Port, err)
		}
	}
	return nil
}

func verifyManagedXrayListener(listen string, port uint16) error {
	runtime := systemdXrayRuntime{}
	if !runtime.IsRunning() {
		return errors.New("h-ui-xray.service is not active")
	}
	output, err := xrayCommand("systemctl", "show", "--property", "MainPID", "--value", constant.XraySystemdUnitName)
	if err != nil {
		return fmt.Errorf("read managed Xray MainPID: %s", safeCommandOutput(output, err))
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil || pid <= 0 {
		return errors.New("managed Xray has no valid MainPID")
	}
	inodes, err := processSocketInodes(pid)
	if err != nil {
		return fmt.Errorf("inspect managed Xray sockets: %w", err)
	}
	table, err := os.ReadFile("/proc/net/tcp")
	if err != nil {
		return fmt.Errorf("read TCP socket table: %w", err)
	}
	if !tcpTableHasOwnedListener(table, listen, port, inodes) {
		return errors.New("SOCKS listener is not owned by h-ui-xray.service MainPID")
	}
	return probeSocks5(net.JoinHostPort(listen, strconv.Itoa(int(port))), time.Second)
}

func processSocketInodes(pid int) (map[string]struct{}, error) {
	entries, err := os.ReadDir(filepath.Join("/proc", strconv.Itoa(pid), "fd"))
	if err != nil {
		return nil, err
	}
	inodes := make(map[string]struct{})
	for _, entry := range entries {
		target, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(pid), "fd", entry.Name()))
		if err != nil {
			continue
		}
		if strings.HasPrefix(target, "socket:[") && strings.HasSuffix(target, "]") {
			inodes[strings.TrimSuffix(strings.TrimPrefix(target, "socket:["), "]")] = struct{}{}
		}
	}
	return inodes, nil
}

func tcpTableHasOwnedListener(table []byte, listenIP string, port uint16, ownedInodes map[string]struct{}) bool {
	ip := net.ParseIP(listenIP).To4()
	if ip == nil {
		return false
	}
	local := fmt.Sprintf("%02X%02X%02X%02X:%04X", ip[3], ip[2], ip[1], ip[0], port)
	for _, line := range strings.Split(string(table), "\n") {
		fields := strings.Fields(line)
		if len(fields) <= 9 || fields[1] != local || fields[3] != "0A" {
			continue
		}
		if _, ok := ownedInodes[fields[9]]; ok {
			return true
		}
	}
	return false
}

func probeSocks5(address string, timeout time.Duration) error {
	return probeSocks5Auth(address, "", "", timeout)
}

func probeSocks5Auth(address, username, password string, timeout time.Duration) error {
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	return socks5Authenticate(conn, username, password)
}

func socks5TCPConnect(proxyAddress, host string, port uint16, timeout time.Duration) (net.Conn, error) {
	return socks5TCPConnectAuth(proxyAddress, host, port, "", "", timeout)
}

func socks5TCPConnectAuth(proxyAddress, host string, port uint16, username, password string, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", proxyAddress, timeout)
	if err != nil {
		return nil, fmt.Errorf("connect local SOCKS5: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	if err = socks5Authenticate(conn, username, password); err != nil {
		conn.Close()
		return nil, err
	}
	request, err := socks5Request(1, host, port)
	if err != nil {
		conn.Close()
		return nil, err
	}
	if _, err = conn.Write(request); err != nil {
		conn.Close()
		return nil, err
	}
	if _, err = readSocks5Reply(conn); err != nil {
		conn.Close()
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	return conn, nil
}

func socks5UDPQuery(proxyAddress string, timeout time.Duration) error {
	return socks5UDPQueryAuth(proxyAddress, "", "", timeout)
}

func socks5UDPQueryAuth(proxyAddress, username, password string, timeout time.Duration) error {
	control, err := net.DialTimeout("tcp", proxyAddress, timeout)
	if err != nil {
		return fmt.Errorf("connect local SOCKS5: %w", err)
	}
	defer control.Close()
	_ = control.SetDeadline(time.Now().Add(timeout))
	if err = socks5Authenticate(control, username, password); err != nil {
		return err
	}
	request, _ := socks5Request(3, "0.0.0.0", 0)
	if _, err = control.Write(request); err != nil {
		return err
	}
	relay, err := readSocks5Reply(control)
	if err != nil {
		return err
	}
	if relay.IP == nil || relay.IP.IsUnspecified() {
		host, _, _ := net.SplitHostPort(proxyAddress)
		relay.IP = net.ParseIP(host)
	}
	udpConn, err := net.DialUDP("udp", nil, relay)
	if err != nil {
		return err
	}
	defer udpConn.Close()
	_ = udpConn.SetDeadline(time.Now().Add(timeout))
	id := uint16(time.Now().UnixNano())
	dns := buildDNSQuery(id)
	header, _ := socks5UDPHeader(xrayUDPTestHost, xrayUDPTestPort)
	if _, err = udpConn.Write(append(header, dns...)); err != nil {
		return err
	}
	response := make([]byte, 2048)
	n, err := udpConn.Read(response)
	if err != nil {
		return fmt.Errorf("UDP through SOCKS5 failed: %w", err)
	}
	payload, err := stripSocks5UDPHeader(response[:n])
	if err != nil {
		return err
	}
	if len(payload) < 12 || binary.BigEndian.Uint16(payload[:2]) != id || payload[2]&0x80 == 0 {
		return errors.New("invalid DNS response through SOCKS5")
	}
	return nil
}

func socks5Authenticate(conn net.Conn, username, password string) error {
	methods := []byte{0}
	if username != "" {
		methods = []byte{2}
	}
	if _, err := conn.Write(append([]byte{5, byte(len(methods))}, methods...)); err != nil {
		return err
	}
	reply := make([]byte, 2)
	if _, err := io.ReadFull(conn, reply); err != nil || reply[0] != 5 || reply[1] != methods[0] {
		return errors.New("SOCKS5 authentication negotiation failed")
	}
	if methods[0] == 2 {
		if len(username) > 255 || len(password) > 255 {
			return errors.New("SOCKS5 credentials exceed protocol limits")
		}
		request := append([]byte{1, byte(len(username))}, []byte(username)...)
		request = append(request, byte(len(password)))
		request = append(request, []byte(password)...)
		if _, err := conn.Write(request); err != nil {
			return err
		}
		authReply := make([]byte, 2)
		if _, err := io.ReadFull(conn, authReply); err != nil || authReply[1] != 0 {
			return errors.New("SOCKS5 username/password authentication failed")
		}
	}
	return nil
}

func socks5Request(command byte, host string, port uint16) ([]byte, error) {
	address, err := encodeSocks5Address(host, port)
	if err != nil {
		return nil, err
	}
	return append([]byte{5, command, 0}, address...), nil
}

func socks5UDPHeader(host string, port uint16) ([]byte, error) {
	address, err := encodeSocks5Address(host, port)
	if err != nil {
		return nil, err
	}
	return append([]byte{0, 0, 0}, address...), nil
}

func encodeSocks5Address(host string, port uint16) ([]byte, error) {
	var result []byte
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			result = append([]byte{1}, ip4...)
		} else {
			result = append([]byte{4}, ip.To16()...)
		}
	} else {
		if len(host) == 0 || len(host) > 255 {
			return nil, errors.New("invalid SOCKS5 destination")
		}
		result = append([]byte{3, byte(len(host))}, []byte(host)...)
	}
	return append(result, byte(port>>8), byte(port)), nil
}

func readSocks5Reply(r io.Reader) (*net.UDPAddr, error) {
	header := make([]byte, 4)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	if header[0] != 5 || header[1] != 0 {
		return nil, fmt.Errorf("SOCKS5 request rejected with code %d", header[1])
	}
	host, port, err := readSocks5Address(r, header[3])
	if err != nil {
		return nil, err
	}
	ip := net.ParseIP(host)
	if ip == nil {
		ips, lookupErr := net.LookupIP(host)
		if lookupErr != nil || len(ips) == 0 {
			return nil, errors.New("invalid SOCKS5 relay address")
		}
		ip = ips[0]
	}
	return &net.UDPAddr{IP: ip, Port: int(port)}, nil
}

func readSocks5Address(r io.Reader, addressType byte) (string, uint16, error) {
	var host string
	switch addressType {
	case 1:
		data := make([]byte, 4)
		if _, err := io.ReadFull(r, data); err != nil {
			return "", 0, err
		}
		host = net.IP(data).String()
	case 4:
		data := make([]byte, 16)
		if _, err := io.ReadFull(r, data); err != nil {
			return "", 0, err
		}
		host = net.IP(data).String()
	case 3:
		length := make([]byte, 1)
		if _, err := io.ReadFull(r, length); err != nil {
			return "", 0, err
		}
		data := make([]byte, int(length[0]))
		if _, err := io.ReadFull(r, data); err != nil {
			return "", 0, err
		}
		host = string(data)
	default:
		return "", 0, errors.New("unsupported SOCKS5 address type")
	}
	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(r, portBytes); err != nil {
		return "", 0, err
	}
	return host, binary.BigEndian.Uint16(portBytes), nil
}

func stripSocks5UDPHeader(packet []byte) ([]byte, error) {
	if len(packet) < 4 || packet[0] != 0 || packet[1] != 0 || packet[2] != 0 {
		return nil, errors.New("invalid SOCKS5 UDP response")
	}
	r := bytes.NewReader(packet[4:])
	if _, _, err := readSocks5Address(r, packet[3]); err != nil {
		return nil, err
	}
	consumed := len(packet[4:]) - r.Len()
	return packet[4+consumed:], nil
}

func buildDNSQuery(id uint16) []byte {
	query := make([]byte, 12)
	binary.BigEndian.PutUint16(query[0:2], id)
	binary.BigEndian.PutUint16(query[2:4], 0x0100)
	binary.BigEndian.PutUint16(query[4:6], 1)
	for _, label := range strings.Split("example.com", ".") {
		query = append(query, byte(len(label)))
		query = append(query, label...)
	}
	query = append(query, 0, 0, 1, 0, 1)
	return query
}

type systemdXrayRuntime struct{}

func (systemdXrayRuntime) Ensure() error { return installXraySystemdUnit(managedXrayPaths) }
func (systemdXrayRuntime) IsRunning() bool {
	if runtime.GOOS != "linux" {
		return false
	}
	_, err := xrayCommand("systemctl", "is-active", "--quiet", constant.XraySystemdUnitName)
	return err == nil
}
func (systemdXrayRuntime) Start() error                   { return runSystemctl("start") }
func (systemdXrayRuntime) Stop() error                    { return runSystemctl("stop") }
func (systemdXrayRuntime) Restart() error                 { return runSystemctl("restart") }
func (systemdXrayRuntime) Ready() error                   { return waitForManagedXray(5 * time.Second) }
func (systemdXrayRuntime) PrepareFiles(path string) error { return setXrayOwnership(path) }

func runSystemctl(action string) error {
	if runtime.GOOS != "linux" {
		return errors.New("Xray systemd management is available on Linux only")
	}
	allowed := map[string]bool{"start": true, "stop": true, "restart": true}
	if !allowed[action] {
		return errors.New("unsupported systemd action")
	}
	output, err := xrayCommand("systemctl", action, constant.XraySystemdUnitName)
	if err != nil {
		return fmt.Errorf("systemctl %s failed: %s", action, safeCommandOutput(output, err))
	}
	return nil
}

func installXraySystemdUnit(paths xrayPaths) error {
	if runtime.GOOS != "linux" {
		return errors.New("Xray systemd installation is available on Linux only")
	}
	if err := ensureXrayServiceAccount(); err != nil {
		return err
	}
	if err := ensureXrayDirectories(paths); err != nil {
		return err
	}
	if err := setXrayParentAccess(paths.dataDir); err != nil {
		return err
	}
	if err := writeXraySystemdUnit(paths); err != nil {
		return err
	}
	if err := setXrayOwnership(paths.dataDir); err != nil {
		return err
	}
	output, err := xrayCommand("systemctl", "enable", constant.XraySystemdUnitName)
	if err != nil {
		return fmt.Errorf("enable Xray systemd service: %s", safeCommandOutput(output, err))
	}
	return nil
}

func writeXraySystemdUnit(paths xrayPaths) error {
	unit := xraySystemdUnit(paths)
	current, _ := os.ReadFile(paths.systemdUnit)
	if string(current) != unit {
		if err := atomicWriteFile(paths.systemdUnit, []byte(unit), 0644); err != nil {
			return err
		}
		output, err := xrayCommand("systemctl", "daemon-reload")
		if err != nil {
			return fmt.Errorf("systemctl daemon-reload failed: %s", safeCommandOutput(output, err))
		}
	}
	return nil
}

func xraySystemdUnit(paths xrayPaths) string {
	return fmt.Sprintf(`[Unit]
Description=Better HY2 managed Xray outbound
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=%s
Group=%s
ExecStart=%s/xray run -c %s
Restart=on-failure
RestartSec=3s
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadOnlyPaths=%s
RestrictAddressFamilies=AF_INET AF_INET6 AF_UNIX
CapabilityBoundingSet=
AmbientCapabilities=

[Install]
WantedBy=multi-user.target
`, constant.XrayServiceUser, constant.XrayServiceUser, paths.currentLink, paths.config, paths.dataDir)
}

func ensureXrayServiceAccount() error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if _, err := user.Lookup(constant.XrayServiceUser); err == nil {
		return nil
	}
	output, err := xrayCommand("useradd", "--system", "--user-group", "--home-dir", constant.XrayDataDir, "--shell", "/usr/sbin/nologin", constant.XrayServiceUser)
	if err != nil {
		return fmt.Errorf("create Xray service account: %s", safeCommandOutput(output, err))
	}
	return nil
}

func ensureXrayDirectories(paths xrayPaths) error {
	for _, path := range []string{paths.dataDir, paths.versionsDir, paths.certDir} {
		if err := os.MkdirAll(path, 0750); err != nil {
			return err
		}
	}
	return nil
}

func setXrayOwnership(path string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	u, err := user.Lookup(constant.XrayServiceUser)
	if err != nil {
		return err
	}
	gid, _ := strconv.Atoi(u.Gid)
	return filepath.Walk(path, func(current string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		return os.Chown(current, 0, gid)
	})
}

func setXrayParentAccess(dataDir string) error {
	if runtime.GOOS != "linux" {
		return nil
	}
	u, err := user.Lookup(constant.XrayServiceUser)
	if err != nil {
		return err
	}
	gid, err := strconv.Atoi(u.Gid)
	if err != nil {
		return fmt.Errorf("parse Xray service group: %w", err)
	}
	return applyXrayParentAccess(dataDir, gid, os.Chown, os.Chmod)
}

func applyXrayParentAccess(dataDir string, gid int, chown func(string, int, int) error, chmod func(string, os.FileMode) error) error {
	parent := filepath.Dir(filepath.Clean(dataDir))
	if err := chown(parent, 0, gid); err != nil {
		return fmt.Errorf("set Xray parent directory group: %w", err)
	}
	if err := chmod(parent, 0710); err != nil {
		return fmt.Errorf("set Xray parent directory mode: %w", err)
	}
	return nil
}

func atomicWriteFile(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".hui-xray-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err = temp.Chmod(mode); err == nil {
		_, err = temp.Write(data)
	}
	if err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(path)
	}
	return os.Rename(tempName, path)
}

func replaceSymlink(link, target string) error {
	if _, err := os.Stat(target); err != nil {
		return fmt.Errorf("Xray version target missing: %w", err)
	}
	temp := link + ".new"
	_ = os.Remove(temp)
	if err := os.Symlink(target, temp); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(link)
	}
	if err := os.Rename(temp, link); err != nil {
		_ = os.Remove(temp)
		return err
	}
	return nil
}

func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }

func installedXrayVersions(paths xrayPaths) []string {
	entries, err := os.ReadDir(paths.versionsDir)
	if err != nil {
		return []string{}
	}
	versions := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() && verifyInstalledXrayVersion(paths, entry.Name()) == nil {
			versions = append(versions, entry.Name())
		}
	}
	sort.Strings(versions)
	return versions
}

func verifyInstalledXrayVersion(paths xrayPaths, version string) error {
	binaryPath, err := xrayVersionBinary(paths, version)
	if err != nil {
		return err
	}
	info, err := os.Lstat(binaryPath)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
		return errors.New("installed Xray binary is not a regular executable file")
	}
	manifestData, err := os.ReadFile(filepath.Join(filepath.Dir(binaryPath), xrayManifestName))
	if err != nil {
		return fmt.Errorf("read Xray install manifest: %w", err)
	}
	var manifest xrayInstallManifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return fmt.Errorf("decode Xray install manifest: %w", err)
	}
	if manifest.Version != version || manifest.BinarySHA256 == "" || manifest.ArchiveSHA256 == "" {
		return errors.New("Xray install manifest does not match the requested version")
	}
	binaryHash, err := fileSHA256(binaryPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(binaryHash, manifest.BinarySHA256) {
		return errors.New("installed Xray binary SHA-256 mismatch")
	}
	return verifyXrayExecutable(binaryPath, version, paths.dataDir)
}

func verifyXrayExecutable(binaryPath, version, tempParent string) error {
	output, err := xrayCommand(binaryPath, "version")
	if err != nil {
		return fmt.Errorf("execute Xray version check: %s", safeCommandOutput(output, err))
	}
	expected := strings.TrimPrefix(version, "v")
	versionPattern := regexp.MustCompile(`(?m)^Xray\s+` + regexp.QuoteMeta(expected) + `(?:\s|$)`)
	if !versionPattern.Match(output) {
		return fmt.Errorf("Xray executable version does not match %s", version)
	}
	probe, err := os.CreateTemp(tempParent, ".xray-probe-*.json")
	if err != nil {
		return err
	}
	probePath := probe.Name()
	defer os.Remove(probePath)
	if err = probe.Chmod(0600); err == nil {
		_, err = probe.Write([]byte(`{"log":{"loglevel":"none"}}`))
	}
	closeErr := probe.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	output, err = xrayCommand(binaryPath, "run", "-test", "-c", probePath)
	if err != nil {
		return fmt.Errorf("test Xray executable with a minimal config: %s", safeCommandOutput(output, err))
	}
	return nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func xrayVersionBinary(paths xrayPaths, version string) (string, error) {
	if !regexp.MustCompile(`^v[0-9]+(?:\.[0-9A-Za-z-]+)+$`).MatchString(version) {
		return "", errors.New("invalid Xray version")
	}
	return filepath.Join(paths.versionsDir, version, "xray"), nil
}

func InstallXrayVersion(version string) error {
	xrayOperationMutex.Lock()
	defer xrayOperationMutex.Unlock()

	if runtime.GOOS != "linux" || runtime.GOARCH != "amd64" {
		return errors.New("Xray installation currently supports Linux AMD64 only")
	}
	if err := ensureXrayServiceAccount(); err != nil {
		return err
	}
	binaryPath, err := xrayVersionBinary(managedXrayPaths, version)
	if err != nil {
		return err
	}
	existingUntrusted := false
	if fileExists(binaryPath) {
		if err := verifyInstalledXrayVersion(managedXrayPaths, version); err != nil {
			existingUntrusted = true
		} else {
			return finalizeInstalledXrayVersion(managedXrayPaths, version, xrayRuntimeFactory())
		}
	}
	if err := ensureXrayDirectories(managedXrayPaths); err != nil {
		return err
	}
	assetName := "Xray-linux-64.zip"
	assetURL, err := util.GetReleaseAssetURL("XTLS", "Xray-core", version, assetName)
	if err != nil {
		return err
	}
	digestURL, err := util.GetReleaseAssetURL("XTLS", "Xray-core", version, assetName+".dgst")
	if err != nil {
		return err
	}
	tempDir, err := os.MkdirTemp(managedXrayPaths.dataDir, ".install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	archivePath := filepath.Join(tempDir, assetName)
	if err = downloadXrayAsset(assetURL, archivePath); err != nil {
		return err
	}
	digestData, err := downloadXrayBytes(digestURL)
	if err != nil {
		return err
	}
	if err = verifyXrayDigest(archivePath, digestData); err != nil {
		return err
	}
	staging := filepath.Join(tempDir, "version")
	if err = os.MkdirAll(staging, 0750); err != nil {
		return err
	}
	if err = extractXrayBinary(archivePath, filepath.Join(staging, "xray")); err != nil {
		return err
	}
	stagedBinary := filepath.Join(staging, "xray")
	if err = verifyXrayExecutable(stagedBinary, version, tempDir); err != nil {
		return err
	}
	binaryHash, err := fileSHA256(stagedBinary)
	if err != nil {
		return err
	}
	archiveHash, err := fileSHA256(archivePath)
	if err != nil {
		return err
	}
	manifest, err := json.Marshal(xrayInstallManifest{Version: version, BinarySHA256: binaryHash, ArchiveSHA256: archiveHash})
	if err != nil {
		return err
	}
	if err = atomicWriteFile(filepath.Join(staging, xrayManifestName), manifest, 0640); err != nil {
		return err
	}
	if err = setXrayOwnership(staging); err != nil {
		return err
	}
	finalDir := filepath.Dir(binaryPath)
	if fileExists(finalDir) {
		if !existingUntrusted {
			return errors.New("Xray version directory already exists without an executable")
		}
		if err = attestExistingXrayVersion(managedXrayPaths, version, binaryHash, manifest, setXrayOwnership); err != nil {
			return err
		}
		return finalizeInstalledXrayVersion(managedXrayPaths, version, xrayRuntimeFactory())
	}
	if err = os.Rename(staging, finalDir); err != nil {
		return err
	}
	if err = finalizeInstalledXrayVersion(managedXrayPaths, version, xrayRuntimeFactory()); err != nil {
		_ = os.RemoveAll(finalDir)
		return err
	}
	return nil
}

func finalizeInstalledXrayVersion(paths xrayPaths, version string, runtime xrayRuntime) error {
	if err := verifyInstalledXrayVersion(paths, version); err != nil {
		return fmt.Errorf("verify installed Xray version: %w", err)
	}
	if err := runtime.Ensure(); err != nil {
		return fmt.Errorf("install Xray systemd service: %w", err)
	}
	return nil
}

func attestExistingXrayVersion(paths xrayPaths, version, officialBinaryHash string, manifest []byte, prepare func(string) error) error {
	binaryPath, err := xrayVersionBinary(paths, version)
	if err != nil {
		return err
	}
	existingHash, err := fileSHA256(binaryPath)
	if err != nil {
		return err
	}
	if !strings.EqualFold(existingHash, officialBinaryHash) {
		return errors.New("existing Xray binary does not match the verified official release")
	}
	if err = verifyXrayExecutable(binaryPath, version, paths.dataDir); err != nil {
		return fmt.Errorf("existing Xray executable failed verification: %w", err)
	}
	manifestPath := filepath.Join(filepath.Dir(binaryPath), xrayManifestName)
	if err = atomicWriteFile(manifestPath, manifest, 0640); err != nil {
		return err
	}
	if err = prepare(filepath.Dir(binaryPath)); err != nil {
		_ = os.Remove(manifestPath)
		return err
	}
	if err = verifyInstalledXrayVersion(paths, version); err != nil {
		_ = os.Remove(manifestPath)
		return fmt.Errorf("attest existing Xray version: %w", err)
	}
	return nil
}

func downloadXrayAsset(url, destination string) error {
	data, err := downloadXrayBytes(url)
	if err != nil {
		return err
	}
	return atomicWriteFile(destination, data, 0600)
}

func downloadXrayBytes(url string) ([]byte, error) {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	response, err := xrayHTTPClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with HTTP %d", response.StatusCode)
	}
	return io.ReadAll(io.LimitReader(response.Body, 200<<20))
}

func verifyXrayDigest(path string, digestData []byte) error {
	var expected string
	for _, line := range strings.Split(string(digestData), "\n") {
		upper := strings.ToUpper(line)
		if strings.Contains(upper, "SHA2-256") || strings.Contains(upper, "SHA256") {
			matches := regexp.MustCompile(`(?i)[a-f0-9]{64}`).FindAllString(line, -1)
			if len(matches) > 0 {
				expected = strings.ToLower(matches[len(matches)-1])
				break
			}
		}
	}
	if expected == "" {
		return errors.New("official Xray SHA-256 digest was not found")
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err = io.Copy(hash, file); err != nil {
		return err
	}
	actual := hex.EncodeToString(hash.Sum(nil))
	if actual != expected {
		return errors.New("Xray archive SHA-256 mismatch")
	}
	return nil
}

func extractXrayBinary(archivePath, destination string) error {
	archive, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer archive.Close()
	for _, file := range archive.File {
		if filepath.Base(file.Name) != "xray" {
			continue
		}
		source, err := file.Open()
		if err != nil {
			return err
		}
		defer source.Close()
		target, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0750)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(target, io.LimitReader(source, 200<<20))
		closeErr := target.Close()
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	return errors.New("xray binary not found in official archive")
}

func safeCommandOutput(output []byte, err error) string {
	message := strings.TrimSpace(string(output))
	if len(message) > 2048 {
		message = message[:2048]
	}
	if message == "" && err != nil {
		message = err.Error()
	}
	return message
}
