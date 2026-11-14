import { test } from "node:test";
import assert from "node:assert/strict";
import { formatTrafficG, quotaInput, quotaInputBytes } from "../src/utils/account-quota.ts";

test("traffic uses decimal G and distinguishes zero from small positive values", () => {
  assert.equal(formatTrafficG(0), "0G");
  assert.equal(formatTrafficG(1), "<0.1G");
  assert.equal(formatTrafficG(99_999_999), "<0.1G");
  assert.equal(formatTrafficG(100_000_000), "0.1G");
  assert.equal(formatTrafficG(1_000_000_000), "1G");
  assert.equal(formatTrafficG(1_073_741_824), "1.1G");
});

test("quota inputs accept tenths of G and preserve unlimited and zero", () => {
  assert.equal(quotaInputBytes(0), 0);
  assert.equal(quotaInputBytes(0.1), 100_000_000);
  assert.equal(quotaInputBytes(1.2), 1_200_000_000);
  assert.equal(quotaInputBytes(-1), -1);
  for (const input of [undefined, -2, 0.01, 1.25, Infinity, 9007199.3]) {
    assert.equal(quotaInputBytes(input), undefined);
  }
});

test("legacy bytes can map to a quantized editor without changing stored quota", () => {
  assert.equal(quotaInput(1), 0.1);
  assert.equal(quotaInput(1_073_741_824), 1.1);
  assert.equal(quotaInput(-7), -1);
});
