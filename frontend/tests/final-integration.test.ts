import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

const read = (path: string) => readFileSync(new URL(path, import.meta.url), "utf8");
const routes = read("../src/router/index.ts");
const hy2 = read("../src/views/hysteria/list/index.vue");
const hy2Api = read("../src/api/hysteria2/index.ts");
const xray = read("../src/views/xray/index.vue");

test("login and primary pages retain routable components", () => {
  assert.match(routes, /path: "\/login"[\s\S]*views\/login\/index\.vue/);
  assert.match(routes, /path: "account"[\s\S]*component: "info\/account\/dashboard"/);
  assert.match(routes, /path: "list"[\s\S]*component: "hysteria\/list\/index"/);
  assert.match(routes, /path: "outbound"[\s\S]*component: "xray\/index"/);
});

test("HY2 controls expose state-specific actions, confirmation and status refresh", () => {
  assert.match(hy2, /v-if="hysteria2Monitor\.running"/);
  assert.match(hy2, /handleLifecycleAction\('start'\)/);
  assert.match(hy2, /handleLifecycleAction\('restart'\)/);
  assert.match(hy2, /handleLifecycleAction\('stop'\)/);
  assert.match(hy2, /ElMessageBox\.confirm/);
  assert.match(hy2, /finally \{[\s\S]*setHysteria2Monitor\(\)/);
  assert.match(hy2, /:disabled="lifecycleLoading"/);
  assert.match(hy2Api, /\/hysteria2\/start/);
  assert.match(hy2Api, /\/hysteria2\/stop/);
  assert.match(hy2Api, /\/hysteria2\/restart/);
});

test("outbound editor remains open until apply succeeds and has responsive UI", () => {
  assert.match(xray, /await saveAndApply\(\); editorVisible\.value = false/);
  assert.match(xray, /validateOutboundEditor/);
  assert.match(xray, /template #empty/);
  assert.match(xray, /:xs="24" :sm="16"/);
  assert.match(xray, /@media \(max-width: 768px\)/);
  assert.match(xray, /type="password" show-password/);
});

test("all supported languages include D6 and D7 labels", () => {
  for (const language of ["zh-cn", "en", "ru"]) {
    const source = read(`../src/lang/package/${language}.ts`);
    assert.match(source, /restartConfirm:/);
    assert.match(source, /lifecycleNotice:/);
    assert.match(source, /emptyNodes:/);
    assert.match(source, /duplicateName:/);
  }
});

test("frontend embedding keeps underscore-prefixed Vite chunks", () => {
  const embed = read("../embed.go");
  const embedTest = read("../embed_test.go");
  assert.match(embed, /go:embed all:dist/);
  assert.match(embedTest, /TestEmbeddedUnderscoreChunksAreServed/);
  assert.match(embedTest, /TestEmbeddedProductionAssetReferences/);
});
