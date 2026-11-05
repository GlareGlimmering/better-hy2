import type { Hysteria2ServerConfig } from "../../../api/config/types";

function copyConfig<T>(source: T): T {
  if (source === null || typeof source !== "object") return source;
  if (Array.isArray(source)) return source.map(copyConfig) as T;
  const copy = {} as T;
  for (const key of Object.keys(source as object)) {
    copy[key as keyof T] = copyConfig(source[key as keyof T]);
  }
  return copy;
}

export interface Hysteria2OptionalFeatures {
  ech: boolean;
  obfs: boolean;
  quic: boolean;
  bandwidth: boolean;
  speedTest: boolean;
  udp: boolean;
  resolver: boolean;
  sniff: boolean;
  acl: boolean;
  outbounds: boolean;
  masquerade: boolean;
}

export function buildHysteria2ConfigPayload(
  source: Hysteria2ServerConfig,
  enabled: Hysteria2OptionalFeatures,
  tlsType: string,
  aclType: string
): Hysteria2ServerConfig {
  const payload = copyConfig(source);

  if (payload.obfs?.type === "salamander") delete payload.obfs.gecko;
  if (payload.obfs?.type === "gecko") delete payload.obfs.salamander;

  if (tlsType === "tls") {
    delete payload.acme;
  } else {
    delete payload.tls;
    const acmeType = payload.acme?.type;
    if (acmeType !== "http") delete payload.acme?.http;
    if (acmeType !== "tls") delete payload.acme?.tls;
    if (acmeType !== "dns") delete payload.acme?.dns;
  }

  if (payload.acl) {
    if (aclType === "inline") delete payload.acl.file;
    if (aclType === "file") delete payload.acl.inline;
  }

  if (payload.resolver) {
    for (const type of ["tcp", "udp", "tls", "https"] as const) {
      if (payload.resolver.type !== type) delete payload.resolver[type];
    }
  }

  if (payload.masquerade) {
    for (const type of ["file", "proxy", "string"] as const) {
      if (payload.masquerade.type !== type) delete payload.masquerade[type];
    }
  }

  if (!enabled.ech) delete payload.ech;
  if (!enabled.obfs) delete payload.obfs;
  if (!enabled.quic) delete payload.quic;
  if (!enabled.bandwidth) {
    delete payload.bandwidth;
    delete payload.ignoreClientBandwidth;
    delete payload.congestion;
  }
  if (!enabled.speedTest) delete payload.speedTest;
  if (!enabled.udp) {
    delete payload.disableUDP;
    delete payload.udpIdleTimeout;
  }
  if (!enabled.resolver) delete payload.resolver;
  if (!enabled.sniff) delete payload.sniff;
  if (!enabled.acl) delete payload.acl;
  if (!enabled.outbounds) delete payload.outbounds;
  if (!enabled.masquerade) delete payload.masquerade;

  if (payload.udpIdleTimeout === "") payload.udpIdleTimeout = "60s";
  if (payload.quic?.maxIdleTimeout === "") payload.quic.maxIdleTimeout = "30s";
  if (payload.resolver?.tcp?.timeout === "") payload.resolver.tcp.timeout = "4s";
  if (payload.resolver?.udp?.timeout === "") payload.resolver.udp.timeout = "4s";
  if (payload.resolver?.tls?.timeout === "") payload.resolver.tls.timeout = "10s";
  if (payload.resolver?.https?.timeout === "") payload.resolver.https.timeout = "10s";
  if (payload.sniff?.timeout === "") payload.sniff.timeout = "2s";
  if (payload.acl?.geoUpdateInterval === "") payload.acl.geoUpdateInterval = "168h";

  return payload;
}
