import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import {
  buildAccountOutboundFormState,
  newAccountOutboundFields,
  hasUnavailableBinding,
} from "../src/views/account/list/outbound-form.ts";

const direct = { name: "DIRECT", type: "direct" };
const socks5 = { name: "TEST_VLESS", type: "socks5" };

test("new accounts start as unbound regular users", () => {
  assert.deepEqual(newAccountOutboundFields(), {
    outbound: "",
    role: "user",
  });
});

test("account dialog always renders the outbound field", () => {
  const component = readFileSync(
    new URL("../src/views/account/list/index.vue", import.meta.url),
    "utf8"
  );
  assert.match(
    component,
    /<el-form-item :label="\$t\('account\.outbound'\)" prop="outbound">/
  );
  assert.doesNotMatch(
    component,
    /<el-form-item[^>]*v-if="dataForm\.role !== 'admin'"[^>]*account\.outbound/
  );
  assert.match(component, /getAccountOutboundOptionsApi/);
  assert.doesNotMatch(component, /getXraySettingsApi|getHysteria2ConfigApi/);
  assert.match(component, /router\.push\(\{ name: "XrayOutbound" \}\)/);
});

test("edit form preserves an existing account binding", () => {
  const component = readFileSync(
    new URL("../src/views/account/list/index.vue", import.meta.url),
    "utf8"
  );
  assert.match(component, /v-model="dataForm\.outbound"/);
  assert.match(component, /Object\.assign\(state\.dataForm, data\)/);
});

test("new and edit forms stay visible without outbounds", () => {
  const state = buildAccountOutboundFormState([], "user");
  assert.equal(state.disabled, true);
  assert.equal(state.noOptions, true);
});

test("DIRECT is offered without creating an automatic binding", () => {
  const state = buildAccountOutboundFormState([direct], "user");
  assert.equal(state.disabled, false);
  assert.deepEqual(state.options, [direct]);
});

test("DIRECT and SOCKS5 are both offered", () => {
  const state = buildAccountOutboundFormState([direct, socks5], "user");
  assert.deepEqual(state.options, [direct, socks5]);
});

test("unsupported outbound types are not selectable", () => {
  const state = buildAccountOutboundFormState(
    [direct, { name: "HTTP", type: "http" }],
    "user"
  );
  assert.deepEqual(state.options, [direct]);
});

test("administrator form shows the field but cannot bind an outbound", () => {
  const state = buildAccountOutboundFormState([direct], "admin");
  assert.equal(state.admin, true);
  assert.equal(state.disabled, true);
});

test("deleted and renamed bindings warn without rewriting an account to DIRECT", () => {
  const account = { outbound: "OLD" };
  assert.equal(hasUnavailableBinding([direct, socks5], account.outbound), true);
  assert.equal(account.outbound, "OLD");
  assert.equal(hasUnavailableBinding([direct], "DIRECT"), false);
  assert.equal(hasUnavailableBinding([direct], ""), false);
  assert.equal(hasUnavailableBinding([direct, { name: "OLD", type: "socks5" }], account.outbound), false);
});

test("selector fetches authoritative options on each open and clears failed stale results", () => {
  const component = readFileSync(new URL("../src/views/account/list/index.vue", import.meta.url), "utf8");
  assert.match(component, /@visible-change="refreshOutboundOptions"/);
  assert.match(component, /if \(visible\) void loadOutboundOptions\(\)/);
  assert.match(component, /const handleAdd = async[\s\S]*await loadOutboundOptions\(\)/);
  assert.match(component, /const handleUpdate = async[\s\S]*await loadOutboundOptions\(\)/);
  assert.match(component, /catch \{\s*state\.outboundOptions = \[\]/);
  assert.match(component, /outboundBindingUnavailable/);
});
