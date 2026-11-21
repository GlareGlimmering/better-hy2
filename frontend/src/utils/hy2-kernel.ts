export type HY2KernelDisplay = { label: string; upstream?: string; channel?: "Stable" | "RC" };

const managedBuilds = new Map<string, HY2KernelDisplay>([
  ["v2.13.0-hui.6-dev", { label: "v1.1", upstream: "v2.13.0", channel: "Stable" }],
  ["v2.14.0-hui.1-dev", { label: "v2.14.0-hui.1-dev", upstream: "v2.14.0" }],
  ["v2.14.0-hui.2-dev", { label: "v1.2", upstream: "v2.14.0", channel: "Stable" }],
]);

export function describeHY2Kernel(version?: string): HY2KernelDisplay {
  return managedBuilds.get(version ?? "") ?? { label: version || "—" };
}
