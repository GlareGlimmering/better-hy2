const bytesPerGB = 1_000_000_000;

export function bytesToGB(bytes: number): string {
  return (bytes / bytesPerGB).toFixed(9).replace(/\.?0+$/, "");
}

// Preserve the existing API sentinels as literal values, even in a GB field.
export function quotaInput(bytes: number): string {
  return bytes < 0 ? String(bytes) : bytesToGB(bytes);
}

export function parseQuotaGB(input: string): number | undefined {
  const value = input.trim();
  if (value === "-1") return -1;
  const match = /^(\d+)(?:\.(\d{1,9}))?$/.exec(value);
  if (!match) return undefined;
  const bytes = Number(match[1]) * bytesPerGB + Number((match[2] ?? "").padEnd(9, "0"));
  return Number.isSafeInteger(bytes) ? bytes : undefined;
}
