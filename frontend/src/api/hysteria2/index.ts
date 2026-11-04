import { AxiosPromise } from "axios";
import { Hysteria2ServerConfig } from "@/api/config/types";
import request from "@/utils/request";
import {
  Hysteria2KickDto,
  Hysteria2SubscribeVo,
  Hysteria2SubscribeUrlDto,
  Hysteria2UrlDto,
  Hysteria2UrlVo,
  Hysteria2VersionDto,
  Hysteria2RoutingStatus,
} from "@/api/hysteria2/types";

export function hysteria2KickApi(
  data: Hysteria2KickDto
): AxiosPromise<Hysteria2ServerConfig> {
  return request({
    url: "/hysteria2/hysteria2Kick",
    method: "post",
    data: data,
  });
}

export function hysteria2ChangeVersionApi(
  data: Hysteria2VersionDto
): AxiosPromise {
  return request({
    url: "/hysteria2/hysteria2ChangeVersion",
    method: "post",
    data: data,
  });
}

export function listReleaseApi(): AxiosPromise<string[]> {
  return request({
    url: "/hysteria2/listRelease",
    method: "get",
  });
}

export function hysteria2RoutingStatusApi(): AxiosPromise<Hysteria2RoutingStatus> {
  return request({
    url: "/hysteria2/routingStatus",
    method: "get",
  });
}

export function startHysteria2Api(): AxiosPromise {
  return request({ url: "/hysteria2/start", method: "post" });
}

export function stopHysteria2Api(): AxiosPromise {
  return request({ url: "/hysteria2/stop", method: "post" });
}

export function restartHysteria2Api(): AxiosPromise {
  return request({ url: "/hysteria2/restart", method: "post" });
}

export function hysteria2SubscribeUrlApi(
  dto: Hysteria2SubscribeUrlDto
): AxiosPromise<Hysteria2SubscribeVo> {
  return request({
    url: "/hysteria2/hysteria2SubscribeUrl",
    method: "get",
    params: dto,
  });
}

export function hysteria2UrlApi(
  dto: Hysteria2UrlDto
): AxiosPromise<Hysteria2UrlVo> {
  return request({
    url: "/hysteria2/hysteria2Url",
    method: "get",
    params: dto,
  });
}
