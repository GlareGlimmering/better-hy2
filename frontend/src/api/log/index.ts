import { AxiosPromise } from "axios";
import request from "@/utils/request";
import {
  LogDto,
  LogExportDto,
  LogHysteria2Vo,
  LogStreamDto,
  LogStreamVo,
  LogSystemVo,
  LogType,
  LogUsageSummaryVo,
} from "@/api/log/types";

export function logSystemApi(data: LogDto): AxiosPromise<PageVo<LogSystemVo>> {
  return request({
    url: "/log/logSystem",
    method: "get",
    params: data,
  });
}

export function logHysteria2Api(
  data: LogDto
): AxiosPromise<PageVo<LogHysteria2Vo>> {
  return request({
    url: "/log/logHysteria2",
    method: "get",
    params: data,
  });
}

export function exportLogApi(data: LogExportDto): AxiosPromise {
  return request({
    url: "/log/exportLog",
    method: "post",
    data: data,
    responseType: "blob",
  });
}

export function logStreamApi(data: LogStreamDto): AxiosPromise<LogStreamVo> {
  return request({
    url: "/log/stream",
    method: "get",
    params: data,
  });
}

export function logUsageApi(): AxiosPromise<LogUsageSummaryVo> {
  return request({
    url: "/log/usage",
    method: "get",
  });
}

export function clearLogApi(
  type: LogType | "all"
): AxiosPromise<LogUsageSummaryVo> {
  return request({
    url: "/log/clear",
    method: "post",
    data: { type },
  });
}
