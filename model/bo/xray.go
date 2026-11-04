package bo

import "h-ui/model/constant"

type XraySettings struct {
	Version   string                `json:"version"`
	Outbounds []XrayManagedOutbound `json:"outbounds,omitempty"`

	// Legacy single-outbound fields are read during the compatible migration
	// from releases before the managed outbound list was introduced.
	Name        string `json:"name,omitempty"`
	Address     string `json:"address,omitempty"`
	Port        uint16 `json:"port,omitempty"`
	UUID        string `json:"uuid,omitempty"`
	TLSSNI      string `json:"tlsSni,omitempty"`
	UseCustomCA bool   `json:"useCustomCa,omitempty"`
}

type XrayManagedOutbound struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Protocol    string `json:"protocol"`
	Enabled     bool   `json:"enabled"`
	Address     string `json:"address"`
	Port        uint16 `json:"port"`
	UUID        string `json:"uuid,omitempty"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`
	Network     string `json:"network"`
	TLSSNI      string `json:"tlsSni"`
	UseCustomCA bool   `json:"useCustomCa"`
	SOCKSListen string `json:"socksListen"`
	SOCKSPort   uint16 `json:"socksPort"`
	OutboundTag string `json:"outboundTag"`
}

func DefaultXraySettings() XraySettings {
	return XraySettings{Version: constant.XrayPinnedVersion, Outbounds: []XrayManagedOutbound{}}
}

type XrayConfig struct {
	Log       XrayLog        `json:"log"`
	Inbounds  []XrayInbound  `json:"inbounds"`
	Outbounds []XrayOutbound `json:"outbounds"`
	Routing   XrayRouting    `json:"routing"`
}

type XrayLog struct {
	LogLevel string `json:"loglevel"`
}

type XrayInbound struct {
	Tag      string              `json:"tag"`
	Listen   string              `json:"listen"`
	Port     uint16              `json:"port"`
	Protocol string              `json:"protocol"`
	Settings XrayInboundSettings `json:"settings"`
}

type XrayInboundSettings struct {
	Auth string `json:"auth"`
	UDP  bool   `json:"udp"`
	IP   string `json:"ip"`
}

type XrayOutbound struct {
	Tag            string              `json:"tag"`
	Protocol       string              `json:"protocol"`
	Settings       interface{}         `json:"settings"`
	StreamSettings *XrayStreamSettings `json:"streamSettings,omitempty"`
}

type XrayVLESSSettings struct {
	Address    string `json:"address"`
	Port       uint16 `json:"port"`
	ID         string `json:"id"`
	Encryption string `json:"encryption"`
}

type XrayProxySettings struct {
	Servers []XrayProxyServer `json:"servers"`
}

type XrayProxyServer struct {
	Address string          `json:"address"`
	Port    uint16          `json:"port"`
	Users   []XrayProxyUser `json:"users,omitempty"`
}

type XrayProxyUser struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

type XrayStreamSettings struct {
	Network     string          `json:"network"`
	Security    string          `json:"security"`
	TLSSettings XrayTLSSettings `json:"tlsSettings"`
}

type XrayTLSSettings struct {
	ServerName        string            `json:"serverName"`
	AllowInsecure     bool              `json:"allowInsecure"`
	DisableSystemRoot bool              `json:"disableSystemRoot"`
	Certificates      []XrayCertificate `json:"certificates"`
}

type XrayCertificate struct {
	CertificateFile string `json:"certificateFile"`
	Usage           string `json:"usage"`
}

type XrayRouting struct {
	Rules []XrayRoutingRule `json:"rules"`
}

type XrayRoutingRule struct {
	Type        string   `json:"type"`
	InboundTag  []string `json:"inboundTag,omitempty"`
	OutboundTag string   `json:"outboundTag"`
	Network     string   `json:"network,omitempty"`
}
