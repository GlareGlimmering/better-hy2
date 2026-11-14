const bytesPerG = 1_000_000_000;
const bytesPerTenthG = bytesPerG / 10;

export function formatTrafficG(bytes: number): string {
  if (bytes <= 0) return "0G";
  if (bytes < bytesPerTenthG) return "<0.1G";
  return `${(bytes / bytesPerG).toFixed(1).replace(/\.0$/, "")}G`;
}

export function quotaInput(bytes: number): number {
  if (bytes < 0) return -1;
  if (bytes === 0) return 0;
  return Math.max(1, Math.round(bytes / bytesPerTenthG)) / 10;
}

export function quotaInputBytes(input: number | undefined): number | undefined {
  if (input === -1) return -1;
  if (input === undefined || input < 0 || !Number.isFinite(input)) return undefined;
  const tenths = Math.round(input * 10);
  if (tenths / 10 !== input) return undefined;
  const bytes = tenths * bytesPerTenthG;
  return Number.isSafeInteger(bytes) ? bytes : undefined;
}
