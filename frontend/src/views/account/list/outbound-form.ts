export interface AccountOutboundOption {
  name: string;
  type: string;
}

export interface AccountOutboundFormState {
  options: AccountOutboundOption[];
  disabled: boolean;
  noOptions: boolean;
  admin: boolean;
}

export function newAccountOutboundFields() {
  return { outbound: "", role: "user" as const };
}

export function buildAccountOutboundFormState(
  outbounds: AccountOutboundOption[],
  role: string | undefined
): AccountOutboundFormState {
  const options = outbounds.filter((item) => {
    const type = item.type.trim().toLowerCase();
    return type === "direct" || type === "socks5";
  });
  const admin = role === "admin";
  return {
    options,
    disabled: admin || options.length === 0,
    noOptions: options.length === 0,
    admin,
  };
}
