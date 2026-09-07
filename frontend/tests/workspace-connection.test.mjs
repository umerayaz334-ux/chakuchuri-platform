import assert from "node:assert/strict";
import test from "node:test";
import { connectWorkspaceRealtime } from "../src/platform/workspace-realtime.ts";

test("connection-local revisions recover on reconnect without resyncing each update", () => {
  const originalWindow = globalThis.window;
  const originalSocket = globalThis.WebSocket;
  const sockets = [];
  const retries = new Map();
  let timerId = 0;
  let resyncs = 0;
  let deliveries = 0;
  class Socket {
    static OPEN = 1;
    static CLOSED = 3;
    readyState = 1;
    constructor() { sockets.push(this); }
    close() { this.readyState = 3; this.onclose?.(); }
    receive(type, revision, changes) {
      this.onmessage({ data: JSON.stringify({ type, revision, changes }) });
    }
  }
  globalThis.WebSocket = Socket;
  globalThis.window = {
    location: { protocol: "http:", host: "localhost:5170" },
    addEventListener() {}, removeEventListener() {},
    setTimeout(callback) { retries.set(++timerId, callback); return timerId; },
    clearTimeout(id) { retries.delete(id); },
    setInterval() { return ++timerId; }, clearInterval() {}
  };
  let stop;
  try {
    stop = connectWorkspaceRealtime("test-session", {
      onResync() { resyncs++; },
      onChanges() { deliveries++; }
    });
    const socket = sockets[0];
    socket.receive("ready", 0);
    assert.equal(resyncs, 1);
    socket.receive("workspace.changed", 1, [{ scope: "presence", operation: "upsert", data: { userId: "u" } }]);
    socket.receive("pong", 1);
    assert.equal(resyncs, 1);
    assert.equal(deliveries, 1);
    socket.receive("workspace.changed", 3, [{ scope: "users", operation: "invalidate" }]);
    assert.equal(resyncs, 2);
    socket.close();
    const retry = retries.values().next().value;
    assert.equal(typeof retry, "function");
    retry();
    sockets[1].receive("ready", 0);
    assert.equal(resyncs, 3);
    sockets[1].receive("pong", 0);
    assert.equal(resyncs, 3);
  } finally {
    stop?.();
    globalThis.window = originalWindow;
    globalThis.WebSocket = originalSocket;
  }
});
