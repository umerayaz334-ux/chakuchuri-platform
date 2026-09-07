import assert from "node:assert/strict";
import test from "node:test";
import { applyCustomerPresence } from "../src/platform/customer-presence.ts";

test("presence aggregates customer sessions without changing account details", () => {
  const rows = [{ id: "c1", companyName: "Export", online: false, lastOnline: "", balanceDue: "123" }, { id: "c2", online: false, lastOnline: "" }];
  const result = applyCustomerPresence(rows, [
    { userId: "one", customerId: "c1", role: "Customer", online: true, lastOnline: "2026-09-05T12:00:00Z" },
    { userId: "two", customerId: "c1", role: "Customer", online: false, lastOnline: "2026-09-05T11:00:00Z" },
    { userId: "staff", customerId: "c2", role: "Staff", online: true, lastOnline: "2026-09-05T12:00:00Z" }
  ]);
  assert.equal(result[0].online, true);
  assert.equal(result[0].lastOnline, "2026-09-05T12:00:00Z");
  assert.equal(result[0].balanceDue, "123");
  assert.equal(result[1], rows[1]);
  assert.equal(rows[0].online, false);
});

test("offline changes preserve the newest activity timestamp", () => {
  const rows = [{ id: "c1", online: true, lastOnline: "2026-09-05T12:00:00Z" }];
  const result = applyCustomerPresence(rows, [{ customerId: "c1", role: "Customer", online: false, lastOnline: "2026-09-05T11:00:00Z" }]);
  assert.equal(result[0].online, false);
  assert.equal(result[0].lastOnline, rows[0].lastOnline);
  assert.equal(applyCustomerPresence(result, [])[0], result[0]);
});
