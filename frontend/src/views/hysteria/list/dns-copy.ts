export async function copyConfirmedDNSValue(
  confirm: () => Promise<boolean>,
  read: () => Promise<string>,
  copy: (value: string) => Promise<void>
): Promise<boolean> {
  if (!(await confirm())) return false;
  await copy(await read());
  return true;
}

export function parseWorkerCredentials(value: string): Record<string, string> {
  const match = /^(\S+)(?:\s+([\s\S]*))?$/.exec(value.trim());
  return { worker_url: match?.[1] ?? "", worker_token: match?.[2] ?? "" };
}
