import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { createDashboardPolling } from "../src/views/info/account/polling.ts";
import type { DashboardPollingScheduler } from "../src/views/info/account/polling.ts";

function fakeScheduler() {
  let nextHandle = 1;
  const callbacks = new Map<number, () => void>();
  const scheduler: DashboardPollingScheduler = {
    setInterval(handler) {
      const handle = nextHandle++;
      callbacks.set(handle, handler);
      return handle;
    },
    clearInterval(handle) {
      callbacks.delete(handle as number);
    },
  };
  return { scheduler, callbacks };
}

test("dashboard polling pauses, resumes and stops without duplicate timers", async () => {
  const { scheduler, callbacks } = fakeScheduler();
  let runs = 0;
  const polling = createDashboardPolling(
    [{ interval: 8000, run: () => void runs++ }],
    scheduler
  );

  polling.start();
  await Promise.resolve();
  assert.equal(runs, 1);
  assert.equal(callbacks.size, 1);

  polling.start();
  assert.equal(callbacks.size, 1);

  polling.pause();
  assert.equal(callbacks.size, 0);
  assert.equal(polling.isRunning(), false);

  polling.resume();
  await Promise.resolve();
  assert.equal(runs, 2);
  assert.equal(callbacks.size, 1);

  polling.stop();
  assert.equal(callbacks.size, 0);
});

test("dashboard polling does not overlap the same request", async () => {
  const { scheduler, callbacks } = fakeScheduler();
  let runs = 0;
  let release: (() => void) | undefined;
  const polling = createDashboardPolling(
    [
      {
        interval: 30000,
        run: () => {
          runs++;
          return new Promise<void>((resolve) => {
            release = resolve;
          });
        },
      },
    ],
    scheduler
  );

  polling.start();
  const tick = [...callbacks.values()][0];
  tick();
  assert.equal(runs, 1);
  release?.();
  await Promise.resolve();
  tick();
  assert.equal(runs, 2);
  polling.stop();
});

test("dashboard route keeps the legacy monitor URL as a redirect", () => {
  const router = readFileSync(
    new URL("../src/router/index.ts", import.meta.url),
    "utf8"
  );
  assert.match(router, /path: "\/monitor\/system"/);
  assert.match(router, /redirect: "\/info\/account"/);
  assert.doesNotMatch(router, /name: "MonitorSystem"/);
});
