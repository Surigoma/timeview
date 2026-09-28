import assert from "node:assert/strict";
import { test } from "node:test";
import { requestID } from "./useTimer.ts";

test("request IDs work without secure-context randomUUID", () => {
  const first = requestID();
  assert.match(first, /^[0-9a-f]{32}$/);
  assert.notEqual(first, requestID());
});
