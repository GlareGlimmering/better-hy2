import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import type { Hysteria2ServerConfig } from "../src/api/config/types.ts";
import {
  buildHysteria2ConfigPayload,
  type Hysteria2OptionalFeatures,
} from "../src/views/hysteria/list/config-form.ts";

const allEnabled: Hysteria2OptionalFeatures = {
  ech: true,
  obfs: true,
  quic: true,
  bandwidth: true,
  speedTest: true,
  udp: true,
  resolver: true,
  sniff: true,
  acl: true,
  outbounds: true,
  masquerade: true,
};

function config(): Hysteria2ServerConfig {
  return {
    listen: ":443",
    tls: { cert: "/cert.crt", key: "/cert.key" },
    acme: {
      domains: ["example.com"], email: "a@example.com", ca: "letsencrypt",
      listenHost: "0.0.0.0", dir: "acme", type: "dns",
      http: { altPort: 80 }, tls: { altPort: 443 },
      dns: { name: "cloudflare", config: { token: "secret" } },
      disableHTTP: false, disableTLSALPN: false,
      altHTTPPort: 80, altTLSALPNPort: 443,
    },
    obfs: {
      type: "salamander",
      salamander: { password: "sensitive" },
      gecko: { password: "draft", minPacketSize: 512, maxPacketSize: 1200 },
    },
    resolver: {
      type: "udp",
      tcp: { addr: "1.1.1.1:53", timeout: "4s" },
      udp: { addr: "8.8.8.8:53", timeout: "4s" },
    },
    acl: { file: "/acl.txt", inline: ["reject(all)"], geoUpdateInterval: "168h" },
    outbounds: [{ name: "DIRECT", type: "direct", direct: { mode: "auto" } }],
    trafficStats: { listen: ":9999" },
    masquerade: {
      type: "proxy",
      file: { dir: "/www" },
      proxy: { url: "https://example.com", rewriteHost: true, insecure: false, xForwarded: false },
      string: { content: "hello" },
    },
  };
}

test("optional features are omitted without destroying the editable draft", () => {
  const source = config();
  const disabled = { ...allEnabled, obfs: false, resolver: false, outbounds: false };
  const payload = buildHysteria2ConfigPayload(source, disabled, "tls", "inline");

  assert.equal(payload.obfs, undefined);
  assert.equal(payload.resolver, undefined);
  assert.equal(payload.outbounds, undefined);
  assert.equal(source.obfs?.salamander?.password, "sensitive");
  assert.equal(source.resolver?.udp?.addr, "8.8.8.8:53");
  assert.equal(source.outbounds?.[0].name, "DIRECT");

  const restored = buildHysteria2ConfigPayload(source, allEnabled, "tls", "inline");
  assert.equal(restored.obfs?.salamander?.password, "sensitive");
  assert.equal(restored.resolver?.udp?.addr, "8.8.8.8:53");
});

test("mutually exclusive TLS, resolver, ACL and masquerade variants are pruned only in payload", () => {
  const source = config();
  const payload = buildHysteria2ConfigPayload(source, allEnabled, "acme", "file");

  assert.equal(payload.tls, undefined);
  assert.equal(payload.acme?.dns?.name, "cloudflare");
  assert.equal(payload.acme?.http, undefined);
  assert.equal(payload.acme?.tls, undefined);
  assert.equal(payload.resolver?.tcp, undefined);
  assert.equal(payload.resolver?.udp?.addr, "8.8.8.8:53");
  assert.equal(payload.acl?.inline, undefined);
  assert.equal(payload.acl?.file, "/acl.txt");
  assert.equal(payload.masquerade?.file, undefined);
  assert.equal(payload.masquerade?.string, undefined);
  assert.equal(source.tls?.cert, "/cert.crt");
  assert.deepEqual(source.acl?.inline, ["reject(all)"]);
});

test("unknown and backend-managed fields are not rewritten by form preparation", () => {
  const source = config() as Hysteria2ServerConfig & Record<string, unknown>;
  source.futureOption = { enabled: true };
  source.userOutbounds = { alice: "DIRECT" };
  const payload = buildHysteria2ConfigPayload(source, allEnabled, "tls", "inline") as Hysteria2ServerConfig & Record<string, unknown>;

  assert.deepEqual(payload.futureOption, { enabled: true });
  assert.deepEqual(payload.userOutbounds, { alice: "DIRECT" });
});

test("HY2 page uses fixed groups and does not expose managed userOutbounds", () => {
  const component = readFileSync(
    new URL("../src/views/hysteria/list/index.vue", import.meta.url),
    "utf8"
  );
  assert.doesNotMatch(component, /addConfigItem|handleDropdownClick|<el-tabs[^>]*closable/);
  for (const group of ["basic", "tls", "authTraffic", "performance", "network", "obfuscation", "advancedRules"]) {
    assert.match(component, new RegExp(`name="${group}"`));
  }
  assert.match(component, /expandedGroups: \["basic", "tls", "authTraffic", "performance"\]/);
  assert.match(component, /buildHysteria2ConfigPayload/);
  assert.doesNotMatch(component, /v-model="dataForm\.userOutbounds"/);
  assert.match(component, /catch \{[\s\S]*setConfig\(\);[\s\S]*setRoutingStatus\(\);/);
  assert.match(component, /type="password"[\s\S]*v-model="dataForm\.obfs\.salamander\.password"|v-model="dataForm\.obfs\.salamander\.password"[\s\S]*type="password"/);
});

test("ACME DNS copy requests a secret only after confirmation", async () => {
  const { copyConfirmedDNSValue } = await import("../src/views/hysteria/list/dns-copy.ts");
  const calls: string[] = [];
  const read = async () => { calls.push("read"); return "fixture"; };
  const copy = async (value: string) => { calls.push(value); };
  assert.equal(await copyConfirmedDNSValue(async () => false, read, copy), false);
  assert.deepEqual(calls, []);
  assert.equal(await copyConfirmedDNSValue(async () => true, read, copy), true);
  assert.deepEqual(calls, ["read", "fixture"]);
});
