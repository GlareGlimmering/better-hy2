import type { AccountForm, AccountVo } from "../../../api/account/types";

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
