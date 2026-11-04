import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import {
  buildAccountOutboundFormState,
  newAccountOutboundFields,
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
  assert.match(component, /getXraySettingsApi/);
  assert.match(component, /name: "DIRECT", type: "direct"/);
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
