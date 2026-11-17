import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { editAccountForm, newAccountForm } from "../src/views/account/list/account-form.ts";

import {
  timestampToCompactDateTime,
  timestampToFullDateTimeWithZone,
} from "../src/utils/time.ts";

const accountPage = readFileSync(
  new URL("../src/views/account/list/index.vue", import.meta.url),
  "utf8"
);

test("account timestamps use compact cells with full timezone tooltips", () => {
  const timestamp = new Date(2026, 10, 2, 6, 14, 34).getTime();
  assert.match(timestampToCompactDateTime(timestamp), /^2026-11-02 06:14$/);
  assert.match(
    timestampToFullDateTimeWithZone(timestamp),
    /^2026-11-02 06:14:34 UTC[+-]\d{2}:\d{2} · .+$/
  );
  assert.match(accountPage, /timestampToFullDateTimeWithZone/);
  assert.match(accountPage, /timestampToCompactDateTime/);
});

test("account dialogs remain precise and adapt to narrow screens", () => {
  assert.match(accountPage, /format="YYYY-MM-DD HH:mm:ss"/);
  assert.match(accountPage, /:fullscreen="useFullscreenDialog"/);
  assert.match(accountPage, /:label-position="isMobile \? 'top' : 'right'"/);
});

test("account row opens the editor and three accessible links stop row clicks", () => {
  assert.match(accountPage, /@row-click="handleUpdate"/);
  assert.match(accountPage, /fixed="right"/);
  for (const action of ["handleSubscribe", "handleNodeUrl", "handleQrCode"]) {
    assert.match(accountPage, new RegExp('@click\\.stop="'+action+'\\(row\\)"'));
  }
  assert.match(accountPage, /:aria-label="\$t\('common.nodeQrCode'\)"/);
  assert.match(accountPage, /class="account-qr-icon"/);
  assert.doesNotMatch(accountPage, /drawer-secondary-actions|handleAccountCommand|<el-dropdown/);
  for (const action of ["confirmResetTraffic", "handleKick", "confirmReleaseKickWithPrompt", "handleDelete"]) {
    assert.ok(accountPage.includes(action+"(drawerAccount)"));
  }
});

test("account password is read-only and can be regenerated before saving", () => {
  const start = accountPage.indexOf('prop="conPass"');
  const password = accountPage.slice(start, accountPage.indexOf('prop="outbound"', start));
  assert.match(password, /<template #append>/);
  assert.match(password, /@click="generatePassword"/);
  assert.match(password, /:model-value="dataForm\.conPass"/);
  assert.match(password, /\breadonly\b/);
  assert.doesNotMatch(password, /v-model|clearable|@input/);
  assert.match(password, /:loading="passwordGenerating"/);
  assert.match(accountPage, /state\.dataForm = newAccountForm\(getMonthLater\(\)\);[\s\S]*void generatePassword\(\)/);
  assert.match(accountPage, /request === passwordRequest && dialog\.value\.visible && state\.dataForm\.id === accountID/);
  assert.match(accountPage, /:disabled="accountSaving \|\| passwordGenerating \|\| !creationBindingsComplete" @click="submitForm"/);
});

test("account drawer uses tenths of G and omits unchanged legacy quota", () => {
  assert.match(accountPage, /:step="0\.1"/);
  assert.match(accountPage, /:precision="1"/);
  assert.match(accountPage, /unchangedQuota = [^\n]+quotaInput\(state\.dataForm\.quota\)/);
  assert.match(accountPage, /unchangedQuota \? undefined : quotaInputBytes/);
  assert.match(accountPage, /\{2,32\}/);
});

test("navbar keeps language and account actions in narrow and wide layouts", () => {
  const navbar = readFileSync(new URL("../src/layout/components/Navbar.vue", import.meta.url), "utf8");
  assert.match(navbar, /<lang-select/);
  assert.match(navbar, /navbar.logout/);
  assert.match(navbar, /\.navbar-actions\s*\{\s*flex: none/);
  assert.doesNotMatch(navbar, /[Ff]ullscreen|size-select|device !== 'mobile'/);
});

test("account management has no import or export control", () => {
  assert.doesNotMatch(accountPage, /handleImport|handleExport|<el-upload/);
});

test("pending password change stays on login without storing a session", () => {
  const login = readFileSync(new URL("../src/views/login/index.vue", import.meta.url), "utf8");
  const store = readFileSync(new URL("../src/store/modules/account.ts", import.meta.url), "utf8");
  assert.match(login, /if \(!signedIn\) \{\s*passwordChangeRequired\.value = true;\s*changingPassword\.value = true;\s*return;/);
  assert.match(store, /if \(data\.passwordChangeRequired\) \{\s*resetToken\(\);\s*return false;/);
});

test("strict routing setup requires an explicit selection for every unassigned user", () => {
  assert.match(accountPage, /v-for="username in routingBindingUsers"/);
  assert.match(accountPage, /routingBindings\.clear\(\);/);
  assert.match(accountPage, /routingBindingUsers\.value\.every\(username => routingBindings\.has\(username\)\)/);
  assert.match(accountPage, /:disabled="!routingBindingsComplete"/);
  assert.match(accountPage, /await bindUnassignedOutboundsApi\(Object\.fromEntries\(routingBindings\)\)/);
});

test("edit then delete and add starts with a fresh account form", () => {
  const old = editAccountForm({
    id: 13, username: "old-user", remark: "old remark", conPass: "unexpected-secret",
    quota: 200000000, uploadMbps: 3, downloadMbps: 7, expireTime: 123,
    deviceNo: 2, deleted: 0, role: "user", outbound: "OLD-SOCKS",
  } as Parameters<typeof editAccountForm>[0]);
  assert.equal(old.conPass, "");
  assert.equal(old.outbound, "OLD-SOCKS");
  old.conPass = "unsaved-generated-password";
  const fresh = newAccountForm(456);
  assert.deepEqual(fresh, {
    username: "", remark: "", conPass: "", quota: 0, uploadMbps: 0, downloadMbps: 0,
    expireTime: 456, deviceNo: 6, deleted: 0, role: "user", outbound: "",
  });
  assert.equal(fresh.id, undefined);
  assert.match(accountPage, /if \(request !== accountEditorRequest\) return;\s*state\.dataForm = editAccountForm\(data\)/);
});

test("first bound account creation submits all explicit legacy bindings with the account", () => {
  assert.match(accountPage, /v-for="username in creationBindingUsers"/);
  assert.match(accountPage, /creationBindingUsers\.value\.every\(username => !!creationBindings\.get\(username\)\)/);
  assert.match(accountPage, /await saveAccountApi\(\{[\s\S]*existingBindings: Object\.fromEntries\(creationBindingUsers\.value\.map/);
});
