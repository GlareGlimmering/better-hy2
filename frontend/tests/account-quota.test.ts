import { test } from "node:test";
import assert from "node:assert/strict";
import { bytesToGB, parseQuotaGB, quotaInput } from "../src/utils/account-quota.ts";

test("quota uses decimal GB and can round-trip a single byte", () => {
  for (const bytes of [0, 1, 1_000_000_000, 1_073_741_824, 99_123_456_789]) {
    assert.equal(parseQuotaGB(quotaInput(bytes)), bytes);
  }
  assert.equal(bytesToGB(1_073_741_824), "1.073741824");
  assert.equal(parseQuotaGB("1.25"), 1_250_000_000);
});

test("legacy negative quotas remain distinguishable and invalid edits are rejected", () => {
  assert.equal(quotaInput(-7), "-7");
  assert.equal(parseQuotaGB("-1"), -1);
  for (const input of ["", "-2", "0.0000000001", "1e2", "Infinity", "9007199.254740992"]) {
    assert.equal(parseQuotaGB(input), undefined);
  }
});
