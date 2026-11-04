export interface SystemMonitorVo {
  huiVersion: string;
  cpuPercent: number;
  diskPercent: number;
  memPercent: number;
}

export interface Hysteria2MonitorVo {
  userTotal: number;
  deviceTotal: number;
  version: string;
  running: boolean;
}

export interface AccountTrafficVo {
  username: string;
  upload: number;
  download: number;
  total: number;
}

export interface TrafficMonitorVo {
  upload: number;
  download: number;
  total: number;
  topUsers: AccountTrafficVo[];
}
