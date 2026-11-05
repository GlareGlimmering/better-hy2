import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { failedTestResult, relativeTestTime, visibleExitIp } from "../src/views/xray/test-result.ts";
import { classifyServerAddress, isLiteralIp, serverAddressValue } from "../src/views/subscription/settings/server-address.ts";

const read = (path: string) => readFileSync(new URL(path, import.meta.url), "utf8");
const xray = read("../src/views/xray/index.vue");
const accounts = read("../src/views/account/list/index.vue");
const settingsPage = read("../src/views/config/list/index.vue");
const settingsStore = read("../src/store/modules/settings.ts");
const subscription = read("../src/views/subscription/settings/index.vue");
const locales = ["zh-cn", "en", "ru"].map(language =>
  read(`../src/lang/package/${language}.ts`)
);

test("outbound tests render inline and retain explicit failure state", () => {
  assert.doesNotMatch(xray, /v-model="testVisible"/);
  assert.match(xray, /testing\[scope\.row\.id\]/);
  assert.match(xray, /protocolTestSummary\(scope\.row\.id, "tcp"\)/);
  assert.match(xray, /protocolTestSummary\(scope\.row\.id, "udp"\)/);
  const failure = failedTestResult("tcp", new Error("timeout"), 1000);
  assert.equal(failure.remoteConnectivity, false);
  assert.equal(failure.error, "timeout");
});

test("exit IP is shown only when its identity was verified", () => {
  const base = failedTestResult("tcp", "timeout");
  assert.equal(visibleExitIp({ ...base, remoteConnectivity: true, exitIp: "1.2.3.4" }), "");
  assert.equal(visibleExitIp({ ...base, remoteConnectivity: true, exitIdentityVerified: true, exitIp: "1.2.3.4" }), "1.2.3.4");
  assert.deepEqual(relativeTestTime(0, 65_000), { value: 1, unit: "minutes" });
});

test("account create and edit share a responsive drawer", () => {
  assert.match(accounts, /<el-drawer/);
  assert.match(accounts, /:size="isMobile \? '100%' : '500px'"/);
  assert.match(accounts, /handleAdd/);
  assert.match(accounts, /handleUpdate/);
  assert.match(accounts, /account\.dangerZone/);
  assert.match(accounts, /const submitForm = async/);
  assert.match(accounts, /await updateAccountApi/);
  assert.match(accounts, /closeDialog\(\);[\s\S]*await Promise\.all/);
  assert.match(accounts, /:loading="accountSaving"/);
  assert.doesNotMatch(accounts, /class="account-dialog"/);
});

test("account more button cannot open editor and drawer dismisses without saving", () => {
  assert.doesNotMatch(accounts, /@row-click="handleUpdate"/);
  assert.match(accounts, /@click\.stop="handleUpdate\(scope\.row\)"/);
  assert.match(accounts, /<el-button type="primary" link @click\.stop>\s*\{\{ \$t\("common\.more"\)/);
  assert.match(accounts, /:close-on-click-modal="true"/);
  assert.match(accounts, /@close="closeDialog"/);
  const close = accounts.slice(accounts.indexOf("const closeDialog ="), accounts.indexOf("const closeDialogKick ="));
  assert.match(close, /resetFields/);
  assert.doesNotMatch(close, /saveAccountApi|updateAccountApi|submitForm/);
});

test("theme initializes globally and settings route has no color-mode mount side effects", () => {
  assert.match(read("../src/App.vue"), /const settingsStore = useSettingsStore\(\)/);
  assert.match(settingsStore, /useColorMode\(\{ emitAuto: true \}\)/);
  assert.match(settingsStore, /colorMode\.value = dark \? "dark" : "light"/);
  assert.match(settingsPage, /v-model="settingsStore\.isDark"/);
  assert.doesNotMatch(settingsPage, /useDark\(|useColorMode\(/);
});

test("subscription server address supports only automatic server IP or literal IP", () => {
  assert.equal(classifyServerAddress("{SERVER_IP}"), "auto");
  assert.equal(classifyServerAddress("203.0.113.8"), "custom");
  assert.equal(classifyServerAddress("2001:db8::1"), "custom");
  assert.equal(classifyServerAddress("legacy.example.com"), "legacy");
  assert.equal(isLiteralIp("999.1.1.1"), false);
  assert.equal(serverAddressValue("auto", ""), "{SERVER_IP}");
  assert.equal(serverAddressValue("custom", " 203.0.113.8 "), "203.0.113.8");
  assert.match(subscription, /subscriptionTemplate\.addressModeAuto/);
  assert.match(subscription, /subscriptionTemplate\.legacyAddressWarning/);
});

test("D9 labels exist in Chinese English and Russian", () => {
  for (const locale of locales) {
    assert.match(locale, /lastTestOnly:/);
    assert.match(locale, /trafficUsed:/);
    assert.match(locale, /dangerZone:/);
    assert.match(locale, /addressModeAuto:/);
    assert.match(locale, /legacyAddressWarning:/);
  }
});
