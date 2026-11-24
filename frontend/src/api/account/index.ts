import request from "@/utils/request";

export function getAccountOutboundOptionsApi(): AxiosPromise<{
  options: { name: string; type: string; source: string }[];
  unavailable: { name: string; type: string; source: string; reason: string }[];
}> {
  return request({ url: "/account/outboundOptions", method: "get" });
}
import { AxiosPromise } from "axios";
import {
  AccountSaveDto,
  AccountInfo,
  AccountLoginDto,
  AccountLoginVo,
  AccountPageDto,
  AccountUpdateDto,
  AccountUpdateResult,
  AccountVo,
} from "./types";

export function bindUnassignedOutboundsApi(bindings: Record<string, string>): AxiosPromise<AccountUpdateResult> {
  return request({ url: "/account/bindUnassignedOutbounds", method: "post", data: { bindings } });
}

/**
 * 查询
 */
export function getAccountApi(data: IdDto): AxiosPromise<AccountVo> {
  return request({
    url: "/account/getAccount",
    method: "get",
    params: data,
  });
}

/**
 * 保存
 *
 * @param data
 */
export function saveAccountApi(
  data: AccountSaveDto
): AxiosPromise<AccountUpdateResult> {
  return request({
    url: "/account/saveAccount",
    method: "post",
    data: data,
  });
}

/**
 * 查询当前
 */
export function getAccountInfoApi(): AxiosPromise<AccountInfo> {
  return request({
    url: "/account/getAccountInfo",
    method: "get",
  });
}

/**
 * 分页
 * @param data
 */
export function pageAccountApi(
  data: AccountPageDto
): AxiosPromise<PageVo<AccountVo>> {
  return request({
    url: "/account/pageAccount",
    method: "get",
    params: data,
  });
}

/**
 * 删除
 *
 * @param data
 */
export function deleteAccountApi(
  data: IdDto
): AxiosPromise<AccountUpdateResult> {
  return request({
    url: "/account/deleteAccount",
    method: "post",
    data: data,
  });
}

/**
 * 修改
 * @param data
 */
export function updateAccountApi(
  data: AccountUpdateDto
): AxiosPromise<AccountUpdateResult> {
  return request({
    url: "/account/updateAccount",
    method: "post",
    data: data,
  });
}

/**
 * 重设流量
 * @param data
 */
export function resetTrafficApi(data: IdDto): AxiosPromise {
  return request({
    url: "/account/resetTraffic",
    method: "post",
    data: data,
  });
}

/**
 * 登录
 * @param data
 */
export function loginApi(data: AccountLoginDto): AxiosPromise<AccountLoginVo> {
  return request({
    url: "/auth/login",
    method: "post",
    data: data,
  });
}

/**
 * 解除下线状态
 */
export function releaseKickAccountApi(data: IdDto): AxiosPromise {
  return request({
    url: "/account/releaseKickAccount",
    method: "post",
    data: data,
  });
}

export function getConnectionPasswordApi(data: IdDto): AxiosPromise<string> {
  return request({ url: "/account/getConnectionPassword", method: "get", params: data });
}

export function generateConnectionPasswordApi(): AxiosPromise<string> {
  return request({ url: "/account/generateConnectionPassword", method: "post" });
}
