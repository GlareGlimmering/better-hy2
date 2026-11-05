import { AxiosPromise } from "axios";
import request from "@/utils/request";
import {
  Hysteria2MonitorVo,
  SystemMonitorVo,
  TrafficMonitorVo,
} from "@/api/monitor/types";

export function monitorSystemApi(): AxiosPromise<SystemMonitorVo> {
  return request({
    url: "/monitor/monitorSystem",
    method: "get",
  });
}

export function monitorHysteria2Api(): AxiosPromise<Hysteria2MonitorVo> {
  return request({
    url: "/monitor/monitorHysteria2",
    method: "get",
  });
}

export function monitorTrafficApi(): AxiosPromise<TrafficMonitorVo> {
  return request({
    url: "/monitor/monitorTraffic",
    method: "get",
  });
}
