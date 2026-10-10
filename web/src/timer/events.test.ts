import assert from "node:assert/strict";
import { test } from "node:test";
import { belongsToInstance, eventURL } from "./events.ts";
import type { TimerState } from "./timer.ts";

test("views share one stream and display pages do not subscribe to operator data", () => {
  assert.equal(
    eventURL("control"),
    "/api/v1/timer/events?topics=displays,logs",
  );
  assert.equal(eventURL("touch"), "/api/v1/timer/events?topics=displays");
  assert.equal(eventURL("display"), "/api/v1/timer/events");
});
test("snapshots from a previous server instance are rejected", () => {
  const current = { instanceId: "new" } as TimerState;
  assert.equal(belongsToInstance({ instanceId: "old" }, current), false);
  assert.equal(belongsToInstance({ instanceId: "new" }, current), true);
  assert.equal(belongsToInstance({ instanceId: "new" }, null), false);
});
