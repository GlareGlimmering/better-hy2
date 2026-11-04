import type { XrayTestResult } from "@/api/xray/types";

export type OutboundTestState = {
  tcp?: XrayTestResult;
  udp?: XrayTestResult;
};

export const failedTestResult = (
  protocol: "tcp" | "udp",
  error: unknown,
  testedAt = Date.now()
): XrayTestResult => ({
  protocol,
  supported: true,
  localProxyAvailable: false,
  remoteConnectivity: false,
  exitIdentityVerified: false,
  latencyMs: 0,
  testedAt,
  error:
    error instanceof Error && error.message
      ? error.message
      : typeof error === "string" && error
        ? error
        : "request failed",
});

export const relativeTestTime = (testedAt: number, now = Date.now()) => {
  const seconds = Math.max(0, Math.floor((now - testedAt) / 1000));
  if (seconds < 60) return { value: Math.max(1, seconds), unit: "seconds" as const };
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return { value: minutes, unit: "minutes" as const };
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return { value: hours, unit: "hours" as const };
  return { value: Math.floor(hours / 24), unit: "days" as const };
};

export const visibleExitIp = (result?: XrayTestResult) =>
  result?.exitIdentityVerified && result.exitIp ? result.exitIp : "";
