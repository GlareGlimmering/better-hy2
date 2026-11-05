export interface XraySettings {
  version: string;
  outbounds: XrayOutbound[];
}

export interface XrayOutbound {
  id: string;
  name: string;
  protocol: "vless" | "http" | "socks5";
  enabled: boolean;
  address: string;
  port: number;
  uuid: string;
  uuidSet: boolean;
  username: string;
  password?: string;
  passwordSet: boolean;
  tcpSupported: boolean;
  udpSupported: boolean;
  network: "tcp";
  tlsSni: string;
  useCustomCa: boolean;
  socksListen: "127.0.0.1";
  socksPort: number;
  outboundTag: string;
}

export interface XrayCAStatus {
  exists: boolean;
  valid: boolean;
  subject?: string;
  fingerprint?: string;
  error?: string;
}

export interface XrayStatus {
  installedVersions: string[];
  activeVersion?: string;
  versionInstalled: boolean;
  serviceInstalled: boolean;
  runtimeReady: boolean;
  runtimeState: "not_installed" | "service_missing" | "not_applied" | "running" | "stopped" | "not_required";
  running: boolean;
  configValid: boolean;
  pendingChanges: boolean;
  lastAppliedAt?: number;
  lastApplyError?: string;
  tcpAvailable?: boolean;
  udpAvailable?: boolean;
  ca: XrayCAStatus;
  error?: string;
}

export interface XrayValidation {
  valid: boolean;
  output?: string;
}

export interface XrayTestResult {
  protocol: "tcp" | "udp";
  supported: boolean;
  localProxyAvailable: boolean;
  remoteConnectivity: boolean;
  exitIdentityVerified: boolean;
  exitIp?: string;
  latencyMs: number;
  testedAt: number;
  error?: string;
}
