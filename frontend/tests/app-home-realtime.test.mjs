import assert from "node:assert/strict";
import { test } from "node:test";
import { applyWorkspaceChanges, changesFromApiMutation } from "../src/platform/workspace-realtime.ts";

const empty = () => ({
  products: [], quotations: [], manufacturing: [], rateSheets: [],
  shipping: [], payments: [], ledger: [], conversations: [], calls: [],
  presence: [], notices: [], featuredProducts: [], metrics: {}
});

for (const [scope, collection] of [["notices", "notices"], ["featured", "featuredProducts"]]) {
  test(scope + " creates, updates and withdraws without resetting other workspace data", () => {
    const initial = empty();
    const value = { id: "item-1", active: true, title: "Published", name: "Published" };
    const created = applyWorkspaceChanges(initial, changesFromApiMutation({
      path: "/api/workflow/" + scope, data: value
    }));
    assert.deepEqual(created[collection], [value]);
    assert.deepEqual(initial[collection], []);
    assert.equal(created.conversations, initial.conversations);
    const updated = applyWorkspaceChanges(created, [{
      scope, operation: "upsert", data: { ...value, title: "Changed" }
    }]);
    assert.equal(updated[collection].length, 1);
    assert.equal(updated[collection][0].title, "Changed");
    const removed = applyWorkspaceChanges(updated, [{
      scope, operation: "remove", data: { id: value.id }
    }]);
    assert.deepEqual(removed[collection], []);
    assert.equal(removed.quotations, initial.quotations);
    assert.equal(updated[collection].length, 1);
  });
}
