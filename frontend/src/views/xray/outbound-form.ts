import type { XrayOutbound } from "../../api/xray/types";

export type OutboundValidationError =
  | "nameRequired"
  | "duplicateName"
  | "addressRequired"
  | "portRequired"
  | "uuidRequired"
  | "sniRequired"
  | "socksPortRequired"
  | "duplicateSocksPort";

export function validateOutboundEditor(
  outbound: XrayOutbound,
  existing: XrayOutbound[],
  editingID: string
): OutboundValidationError | undefined {
  const name = outbound.name.trim();
  if (!name) return "nameRequired";
  if (
    existing.some(
      (item) =>
        item.id !== editingID && item.name.trim().toLowerCase() === name.toLowerCase()
    )
  ) {
    return "duplicateName";
  }
  if (!outbound.address.trim()) return "addressRequired";
  if (!Number.isInteger(outbound.port) || outbound.port < 1 || outbound.port > 65535) {
    return "portRequired";
  }
  if (outbound.protocol === "vless") {
    if (!outbound.uuidSet && !outbound.uuid.trim()) return "uuidRequired";
    if (!outbound.tlsSni.trim()) return "sniRequired";
  }
  if (outbound.protocol !== "socks5") {
    if (!Number.isInteger(outbound.socksPort) || outbound.socksPort < 1024 || outbound.socksPort > 65535) {
      return "socksPortRequired";
    }
    if (
      existing.some(
        (item) =>
          item.id !== editingID &&
          item.protocol !== "socks5" &&
          item.socksPort === outbound.socksPort
      )
    ) {
      return "duplicateSocksPort";
    }
  }
  return undefined;
}
