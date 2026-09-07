import assert from "node:assert/strict";
import test from "node:test";
import { createBlobLoader } from "../src/platform/blob-loader.ts";

test("simultaneous same-session downloads share a request, completed downloads revalidate", async () => {
  let count = 0;
  const load = createBlobLoader(async (_, options) => {
    count++;
    assert.equal(options.cache, "no-cache");
    assert.equal(options.headers.Authorization, "Bearer session-a");
    return new Response("image");
  });
  const one = load("/api/files/photo/thumbnail", "session-a");
  const two = load("/api/files/photo/thumbnail", "session-a");
  assert.equal(one, two);
  assert.equal(await (await one).text(), "image");
  assert.equal(count, 1);
  await load("/api/files/photo/thumbnail", "session-a");
  assert.equal(count, 2);
});

test("different sessions and representations never share a download", async () => {
  let count = 0;
  const load = createBlobLoader(async () => { count++; return new Response("image"); });
  await Promise.all([
    load("/photo/thumbnail", "a"),
    load("/photo/thumbnail", "b"),
    load("/photo/content", "a")
  ]);
  assert.equal(count, 3);
});

test("failed downloads can be retried", async () => {
  let count = 0;
  const load = createBlobLoader(async () => {
    count++;
    return new Response("image", { status: count === 1 ? 403 : 200 });
  });
  await assert.rejects(load("/photo", "a"), /File could not be loaded/);
  assert.equal(await (await load("/photo", "a")).text(), "image");
  assert.equal(count, 2);
});
