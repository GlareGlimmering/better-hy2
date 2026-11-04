package constant

const (
	PanelBindAddress = "127.0.0.1"
	PanelPort        = "8000"

	LogDir        = "logs/"
	SqliteDBDir   = "data/"
	BinDir        = "bin/"
	ExportPathDir = "export/"

	SqliteDBPath = "data/h_ui.db"

	Hysteria2ConfigPath    = "bin/hysteria2.yaml"
	Hysteria2CustomVersion = "v2.13.0-hui.3"
	Hysteria2CustomOwner   = "GlareGlimmering"
	Hysteria2CustomRepo    = "hysteria"
	Hysteria2CustomSHA256  = "f8ef810c87e72fea631c659194222fae08477a042f71d2b27f7760247ac5dbea"

	SystemLogPath          = "logs/h-ui.log"
	Hysteria2LogPath       = "logs/hysteria2.log"
	Hysteria2AccessLogPath = "/var/lib/hui/hy2_access.log"

	XrayDataDir         = "/var/lib/h-ui/xray"
	XrayVersionsDir     = "/var/lib/h-ui/xray/versions"
	XrayCurrentLink     = "/var/lib/h-ui/xray/current"
	XrayConfigPath      = "/var/lib/h-ui/xray/config.json"
	XrayPreviousPath    = "/var/lib/h-ui/xray/config.previous.json"
	XrayCandidatePath   = "/var/lib/h-ui/xray/candidate.json"
	XrayCertDir         = "/var/lib/h-ui/xray/certs"
	XrayCAPath          = "/var/lib/h-ui/xray/certs/ca.crt"
	XraySystemdUnitPath = "/etc/systemd/system/h-ui-xray.service"
	XraySystemdUnitName = "h-ui-xray.service"
	XrayServiceUser     = "h-ui-xray"
	XrayPinnedVersion   = "v26.10.31"
	XraySOCKSListen     = "127.0.0.1"
	XraySOCKSPort       = 10808

	TokenType = "Bearer"
)

// Version is overridden for release builds with -ldflags -X. Keeping it a
// variable makes the binary self-identifying for verified upgrades.
var Version = "v0.0.25-dev"
