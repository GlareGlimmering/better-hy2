import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { validateOutboundEditor } from "../src/views/xray/outbound-form.ts";

const component = readFileSync(
  new URL("../src/views/xray/index.vue", import.meta.url),
  "utf8"
);
const api = readFileSync(
  new URL("../src/api/xray/index.ts", import.meta.url),
  "utf8"
);

test("Xray page groups service, outbound, and configuration actions", () => {
  assert.match(component, /xray\.serviceTitle/);
  assert.match(component, /xray\.outboundList/);
  assert.match(component, /xray\.configurationState/);
  assert.match(component, /v-if="!status\.running"/);
  assert.doesNotMatch(component, /xray\.advancedOperations/);
  assert.match(component, /xray\.installKernel/);
  assert.match(component, /xray\.saveDraft/);
  assert.match(component, /xray\.checkConfig/);
});

test("missing Xray runtime is explicit and cannot be started", () => {
  assert.match(component, /status\.runtimeReady \? \$t\('xray\.tipStart'\)/);
  assert.match(component, /:disabled="!status\.runtimeReady"/);
  assert.match(component, /status\.runtimeState === "not_installed"/);
  assert.match(component, /xray\.serviceMissingState/);
  assert.match(component, /xray\.notAppliedState/);
});

test("each outbound exposes test edit delete and explanatory tooltips", () => {
  assert.match(component, /testOutbound\(scope\.row\)/);
  assert.match(component, /openEdit\(scope\.row\)/);
  assert.match(component, /removeOutbound\(scope\.row\)/);
  assert.match(component, /<el-tooltip/g);
});

test("per-outbound tests send stable ID and protocol to the backend", () => {
  assert.match(api, /testXrayOutboundApi/);
  assert.match(api, /data: \{ id, protocol \}/);
  assert.match(component, /testXrayOutboundApi\(outbound\.id, "tcp"\)/);
  assert.match(component, /testXrayOutboundApi\(outbound\.id, "udp"\)/);
});

test("ordinary editor saves and applies without four manual steps", () => {
  assert.match(component, /await saveXrayDraftApi\(settings\); await applyXrayApi\(\)/);
  assert.match(component, /await saveAndApply\(\)/);
});

test("unified node editor exposes VLESS HTTP and SOCKS5 without leaking backend details", () => {
  assert.match(component, /value="vless"/);
  assert.match(component, /value="http"/);
  assert.match(component, /value="socks5"/);
  assert.match(component, /editor\.protocol === 'vless'/);
  assert.match(component, /editor\.protocol !== 'socks5'/);
  assert.match(component, /xray\.totalNodes/);
});

test("outbound editor validates protocol fields and case-insensitive duplicates", () => {
  const base = {
    id: "one", name: "TEST_VLESS", protocol: "vless" as const, enabled: true,
    address: "example.com", port: 443, uuid: "uuid", uuidSet: false,
    username: "", password: "", passwordSet: false, tcpSupported: true,
    udpSupported: true, network: "tcp", tlsSni: "example.com",
    useCustomCa: false, socksListen: "127.0.0.1", socksPort: 10808,
    outboundTag: "",
  };
  assert.equal(validateOutboundEditor({ ...base, id: "", name: "test_vless" }, [base], ""), "duplicateName");
  assert.equal(validateOutboundEditor({ ...base, id: "", name: "TM_2", socksPort: 10808 }, [base], ""), "duplicateSocksPort");
  assert.equal(validateOutboundEditor({ ...base, protocol: "vless", uuid: "", uuidSet: false }, [], "one"), "uuidRequired");
  assert.equal(validateOutboundEditor({ ...base, protocol: "http", tlsSni: "", uuid: "" }, [], "one"), undefined);
  assert.equal(validateOutboundEditor({ ...base, protocol: "socks5", socksPort: 0, tlsSni: "", uuid: "" }, [], "one"), undefined);
});

test("failed node apply keeps the editor open and restores the local row", () => {
  assert.match(component, /const previous = index >= 0/);
  assert.match(component, /await saveAndApply\(\); editorVisible\.value = false/);
  assert.match(component, /catch \(error\)[\s\S]*settings\.outbounds\.splice\(index, 1, previous\)/);
});
