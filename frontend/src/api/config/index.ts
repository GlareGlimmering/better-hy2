import { AxiosPromise } from "axios";
import request from "@/utils/request";
import {
  ConfigDto,
  ConfigsDto,
  ConfigUpdateDto,
  ConfigVo,
  Hysteria2AcmePathVo,
  Hysteria2ServerConfig,
  SubscriptionTemplate,
} from "@/api/config/types";

export function getSubscriptionTemplateApi(): AxiosPromise<SubscriptionTemplate> {
  return request({ url: "/config/getSubscriptionTemplate", method: "get" });
}

export function updateSubscriptionTemplateApi(
  data: SubscriptionTemplate
): AxiosPromise {
  return request({
    url: "/config/updateSubscriptionTemplate",
    method: "post",
    data,
  });
}

export function resetSubscriptionTemplateApi(): AxiosPromise {
  return request({ url: "/config/resetSubscriptionTemplate", method: "post" });
}

export function getHysteria2ConfigApi(): AxiosPromise<Hysteria2ServerConfig> {
  return request({
    url: "/config/getHysteria2Config",
    method: "get",
  });
}

export interface Hysteria2ApplyResult {
  id: string;
  state: "pending" | "succeeded" | "failed";
  error?: string;
}

export function beginHysteria2ConfigApplyApi(
  data: Hysteria2ServerConfig
): AxiosPromise<Hysteria2ApplyResult> {
  return request({ url: "/config/applyHysteria2Config", method: "post", data });
}

export function getHysteria2ConfigApplyStatusApi(id: string): AxiosPromise<Hysteria2ApplyResult> {
  return request({ url: "/config/applyHysteria2Config", method: "get", params: { id } });
}

export interface FixedUDPRelay {
  name: string;
  addr: string;
}

export function listFixedUDPRelaysApi(): AxiosPromise<{ supported: boolean; relays: FixedUDPRelay[] }> {
  return request({ url: "/config/fixedUDPRelays", method: "get" });
}

export function addFixedUDPRelayApi(data: FixedUDPRelay): AxiosPromise<Hysteria2ApplyResult> {
  return request({ url: "/config/fixedUDPRelays", method: "post", data });
}

export function updateFixedUDPRelayApi(previousName: string, relay: FixedUDPRelay): AxiosPromise<Hysteria2ApplyResult> {
  return request({ url: "/config/fixedUDPRelays", method: "put", data: { previousName, ...relay } });
}

export function deleteFixedUDPRelayApi(name: string): AxiosPromise<Hysteria2ApplyResult> {
  return request({ url: "/config/fixedUDPRelays", method: "delete", data: { name } });
}

export async function waitForHysteria2Job(initial: Hysteria2ApplyResult, pageClosed: () => boolean) {
  let result = initial;
  // Applying a configuration can include both a DNS-01 start and a rollback.
  const deadline = Date.now() + 14 * 60_000;
  while (!pageClosed() && result.state === "pending" && Date.now() < deadline) {
    await new Promise(resolve => window.setTimeout(resolve, 2_000));
    if (pageClosed()) break;
    ({ data: result } = await getHysteria2ConfigApplyStatusApi(result.id));
  }
  return result;
}

export function getConfigApi(data: ConfigDto): AxiosPromise<ConfigVo> {
  return request({
    url: "/config/getConfig",
    method: "get",
    params: data,
  });
}

export function listConfigApi(data: ConfigsDto): AxiosPromise<Array<ConfigVo>> {
  return request({
    url: "/config/listConfig",
    method: "post",
    data: data,
  });
}

export function updateConfigsApi(data: ConfigUpdateDto): AxiosPromise {
  return request({
    url: "/config/updateConfigs",
    method: "post",
    data: data,
  });
}

export function exportConfigApi(): AxiosPromise {
  return request({
    url: "/config/exportConfig",
    method: "post",
    responseType: "blob",
  });
}

export function importConfigApi(data: FormData): AxiosPromise {
  return request({
    url: "/config/importConfig",
    method: "post",
    headers: {
      "Content-Type": "multipart/form-data",
    },
    data: data,
  });
}

export function exportHysteria2ConfigApi(): AxiosPromise {
  return request({
    url: "/config/exportHysteria2Config",
    method: "post",
    responseType: "blob",
  });
}

export function importHysteria2ConfigApi(data: FormData): AxiosPromise {
  return request({
    url: "/config/importHysteria2Config",
    method: "post",
    headers: {
      "Content-Type": "multipart/form-data",
    },
    data: data,
  });
}

export function hysteria2AcmePathApi(): AxiosPromise<Hysteria2AcmePathVo> {
  return request({
    url: "/config/hysteria2AcmePath",
    method: "get",
  });
}

export function restartServerApi(): AxiosPromise {
  return request({
    url: "/config/restartServer",
    method: "post",
  });
}

export function uploadCertFileApi(data: FormData): AxiosPromise<string> {
  return request({
    url: "/config/uploadCertFile",
    method: "post",
    data,
    headers: { "Content-Type": "multipart/form-data" },
  });
}

export function readACMEDNSValueApi(provider: string, key: string): AxiosPromise<string> {
  return request({ url: "/config/readACMEDNSValue", method: "post", data: { provider, key } });
}
