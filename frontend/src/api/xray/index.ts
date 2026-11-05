import { AxiosPromise } from "axios";
import request from "@/utils/request";
import type {
  XrayCAStatus,
  XraySettings,
  XrayStatus,
  XrayTestResult,
  XrayValidation,
} from "./types";

export const getXraySettingsApi = (): AxiosPromise<XraySettings> =>
  request({ url: "/xray/settings", method: "get" });

export const saveXrayDraftApi = (data: XraySettings): AxiosPromise =>
  request({ url: "/xray/draft", method: "post", data });

export const installXrayApi = (version: string): AxiosPromise =>
  request({ url: "/xray/install", method: "post", data: { version } });

export const validateXrayApi = (): AxiosPromise<XrayValidation> =>
  request({ url: "/xray/validate", method: "post" });

export const applyXrayApi = (): AxiosPromise =>
  request({ url: "/xray/apply", method: "post" });

export const getXrayStatusApi = (): AxiosPromise<XrayStatus> =>
  request({ url: "/xray/status", method: "get" });

export const uploadXrayCAApi = (data: FormData): AxiosPromise<XrayCAStatus> =>
  request({
    url: "/xray/ca",
    method: "post",
    headers: { "Content-Type": "multipart/form-data" },
    data,
  });

export const startXrayApi = (): AxiosPromise =>
  request({ url: "/xray/start", method: "post" });
export const stopXrayApi = (): AxiosPromise =>
  request({ url: "/xray/stop", method: "post" });
export const restartXrayApi = (): AxiosPromise =>
  request({ url: "/xray/restart", method: "post" });
export const testXrayOutboundApi = (
  id: string,
  protocol: "tcp" | "udp"
): AxiosPromise<XrayTestResult> =>
  request({ url: "/xray/test", method: "post", data: { id, protocol } });
