export interface AccountPageDto extends BaseDto {
  username?: string;
  deleted?: number;
  remark?: string;
}

export interface AccountUpdateDto extends IdDto {
  username: string;
  pass?: string;
  conPass: string;
  quota?: number;
  expireTime: number;
  deviceNo: number;
  uploadMbps?: number;
  downloadMbps?: number;
  deleted: number;
  remark: string;
  outbound: string;
}

export interface AccountUpdateResult {
  outboundChanged: boolean;
  hysteria2Running: boolean;
  runtimeApplied: boolean;
  bandwidthStatus?: "applied" | "stopped" | "unsupported" | "unconfirmed";
}

export interface AccountSaveDto {
  username: string;
  conPass: string;
  quota: number;
  expireTime: number;
  deviceNo: number;
  uploadMbps?: number;
  downloadMbps?: number;
  deleted: number;
  remark: string;
  outbound: string;
}

export interface AccountLoginDto {
  username: string;
  pass: string;
}

export interface AccountVo extends IdDto {
  username: string;
  quota: number;
  download: number;
  upload: number;
  expireTime: number;
  kickUtilTime: number;
  deviceNo: number;
  uploadMbps?: number;
  downloadMbps?: number;
  role: string;
  deleted: number;
  createTime: string;

  online: boolean;
  device: number;

  loginAt: number;
  conAt: number;
  remark: string;
  outbound: string;
}

export interface AccountLoginVo {
  accessToken: string;
  tokenType: string;
}

export interface AccountInfo {
  id: number;
  username: string;
  roles: string[];
}

export interface AccountForm extends IdDto {
  username: string;
  pass?: string;
  conPass: string;
  quota: number;
  expireTime: number;
  deviceNo: number;
  uploadMbps?: number;
  downloadMbps?: number;
  deleted: number;
  remark: string;
  outbound: string;
  role?: string;
}

export interface KickAccountForm {
  ids: number[];
  kickUtilTime: number;
}
