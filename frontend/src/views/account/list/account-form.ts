import type { AccountForm, AccountVo } from "../../../api/account/types";

// Same cutoff as the backend's subscription template expiry calculation.
export const permanentAccountExpiry = 253370736000000;
export const accountExpiryMonths = [1, 2, 3, 6, 12];

export function accountExpiryAfterMonths(months: number, now = new Date()): number {
  const expiry = new Date(now);
  const day = expiry.getDate();
  expiry.setDate(1);
  expiry.setMonth(expiry.getMonth() + months);
  const lastDay = new Date(expiry.getFullYear(), expiry.getMonth() + 1, 0).getDate();
  expiry.setDate(Math.min(day, lastDay));
  return expiry.getTime();
}

export function newAccountForm(expireTime: number): AccountForm {
  return {
    username: "",
    conPass: "",
    remark: "",
    quota: 0,
    uploadMbps: 0,
    downloadMbps: 0,
    expireTime,
    deviceNo: 6,
    deleted: 0,
    outbound: "",
    role: "user",
  };
}

export function editAccountForm(account: AccountVo): AccountForm {
  const { id, username, remark, quota, uploadMbps, downloadMbps, expireTime, deviceNo, deleted, role, outbound } = account;
  return { id, username, remark, quota, uploadMbps, downloadMbps, expireTime, deviceNo, deleted, role, outbound, conPass: "" };
}
