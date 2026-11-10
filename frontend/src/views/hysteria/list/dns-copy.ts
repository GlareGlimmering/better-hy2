export async function copyConfirmedDNSValue(
  confirm: () => Promise<boolean>,
  read: () => Promise<string>,
  copy: (value: string) => Promise<void>
): Promise<boolean> {
  if (!(await confirm())) return false;
  await copy(await read());
  return true;
}
