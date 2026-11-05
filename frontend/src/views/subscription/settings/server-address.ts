export type ServerAddressMode = "auto" | "custom" | "legacy";

const ipv4Part = (part: string) => /^\d{1,3}$/.test(part) && Number(part) <= 255;

export const isIPv4 = (value: string) => {
  const parts = value.split(".");
  return parts.length === 4 && parts.every(ipv4Part);
};

export const isIPv6 = (value: string) => {
  if (!value.includes(":")) return false;
  if (!/^[0-9a-fA-F:]+$/.test(value) || value.includes(":::")) return false;
  const compressed = value.includes("::");
  if (compressed && value.indexOf("::") !== value.lastIndexOf("::")) return false;
  const parts = value.split(":");
  const nonEmpty = parts.filter(Boolean);
  return nonEmpty.every(part => part.length <= 4) &&
    (compressed ? nonEmpty.length < 8 : nonEmpty.length === 8);
};

export const isLiteralIp = (value: string) => isIPv4(value.trim()) || isIPv6(value.trim());

export const classifyServerAddress = (value: string): ServerAddressMode => {
  if (value === "{SERVER_IP}") return "auto";
  if (isLiteralIp(value)) return "custom";
  return "legacy";
};

export const serverAddressValue = (mode: Exclude<ServerAddressMode, "legacy">, ip: string) =>
  mode === "auto" ? "{SERVER_IP}" : ip.trim();
