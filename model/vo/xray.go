package vo

type XraySettingsVo struct {
	Version   string                  `json:"version"`
	Outbounds []XrayManagedOutboundVo `json:"outbounds"`
}

type XrayManagedOutboundVo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Protocol     string `json:"protocol"`
	Enabled      bool   `json:"enabled"`
	Address      string `json:"address"`
	Port         uint16 `json:"port"`
	UUID         string `json:"uuid"`
	UUIDSet      bool   `json:"uuidSet"`
	Username     string `json:"username,omitempty"`
	PasswordSet  bool   `json:"passwordSet"`
	TCPSupported bool   `json:"tcpSupported"`
	UDPSupported bool   `json:"udpSupported"`
	Network      string `json:"network"`
	TLSSNI       string `json:"tlsSni"`
	UseCustomCA  bool   `json:"useCustomCa"`
	SOCKSListen  string `json:"socksListen"`
	SOCKSPort    uint16 `json:"socksPort"`
	OutboundTag  string `json:"outboundTag"`
}

type XrayCAStatusVo struct {
	Exists      bool   `json:"exists"`
	Valid       bool   `json:"valid"`
	Subject     string `json:"subject,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	Error       string `json:"error,omitempty"`
}

type XrayStatusVo struct {
	InstalledVersions []string       `json:"installedVersions"`
	ActiveVersion     string         `json:"activeVersion,omitempty"`
	VersionInstalled  bool           `json:"versionInstalled"`
	ServiceInstalled  bool           `json:"serviceInstalled"`
	RuntimeReady      bool           `json:"runtimeReady"`
	RuntimeState      string         `json:"runtimeState"`
	Running           bool           `json:"running"`
	ConfigValid       bool           `json:"configValid"`
	PendingChanges    bool           `json:"pendingChanges"`
	LastAppliedAt     int64          `json:"lastAppliedAt,omitempty"`
	LastApplyError    string         `json:"lastApplyError,omitempty"`
	TCPAvailable      *bool          `json:"tcpAvailable,omitempty"`
	UDPAvailable      *bool          `json:"udpAvailable,omitempty"`
	CA                XrayCAStatusVo `json:"ca"`
	Error             string         `json:"error,omitempty"`
}

type XrayValidationVo struct {
	Valid  bool   `json:"valid"`
	Output string `json:"output,omitempty"`
}

type XrayTestVo struct {
	Protocol             string `json:"protocol"`
	Supported            bool   `json:"supported"`
	LocalProxyAvailable  bool   `json:"localProxyAvailable"`
	RemoteConnectivity   bool   `json:"remoteConnectivity"`
	ExitIdentityVerified bool   `json:"exitIdentityVerified"`
	ExitIP               string `json:"exitIp,omitempty"`
	LatencyMs            int64  `json:"latencyMs"`
	TestedAt             int64  `json:"testedAt"`
	Error                string `json:"error,omitempty"`
}
