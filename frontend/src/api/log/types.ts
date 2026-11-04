export interface LogDto {
  numLine: number;
}

export interface LogExportDto {
  option: number;
}

export type LogType = "system" | "hysteria2" | "access";

export interface LogStreamDto {
  type: LogType;
  cursor?: string;
  limit?: number;
}

export interface LogStreamVo {
  lines: string[];
  cursor: string;
  reset: boolean;
  exists: boolean;
}

export interface LogUsageVo {
  type: LogType;
  bytes: number;
  human: string;
  files: number;
  exists: boolean;
  error?: string;
}

export interface LogUsageSummaryVo {
  logs: LogUsageVo[];
  bytes: number;
  human: string;
  complete: boolean;
}

export interface LogSystemVo {
  clientIp: string;
  latencyTime: string;
  level: string;
  msg: string;
  reqMethod: string;
  reqUri: string;
  statusCode: string;
  time: string;
}

export interface LogHysteria2Vo {
  level: string;
  msg: string;
  time: string;
}
