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
