import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

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

test("account actions use a compact fixed operation column", () => {
  assert.match(accountPage, /fixed="right"/);
  assert.match(accountPage, /handleAccountCommand\(\$event, scope\.row\)/);
  assert.match(accountPage, /command="reset-traffic"/);
  assert.match(accountPage, /command="delete"/);
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
