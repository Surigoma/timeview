import { test } from "node:test";
import assert from "node:assert/strict";
import {
  formatTime,
  matches,
  defaultBindings,
  phase,
  requestID,
} from "./timer.ts";
import type { TimerState } from "./timer.ts";

test("rounding around zero and hour boundary", () => {
  assert.equal(formatTime(1), "00:01");
  assert.equal(formatTime(60001), "01:01");
  assert.equal(formatTime(-999), "00:00");
  assert.equal(formatTime(-1000), "−00:01");
  assert.equal(formatTime(3600000), "1:00:00");
  assert.equal(formatTime(-3661000), "−1:01:01");
});
test("blackout shortcut never also adjusts time; repeats ignored", () => {
  const event = {
    code: "NumpadSubtract",
    ctrlKey: true,
    shiftKey: false,
    altKey: false,
    metaKey: false,
    repeat: false,
    isComposing: false,
  };
  assert.deepEqual(
    defaultBindings.filter((b) => matches(b, event)).map((b) => b.action),
    ["blackout"],
  );
  assert.equal(
    defaultBindings.some((b) => matches(b, { ...event, repeat: true })),
    false,
  );
  assert.equal(
    defaultBindings.some((b) => matches(b, { ...event, isComposing: true })),
    false,
  );
});
test("warning precedence and disabled zero thresholds", () => {
  const s = {
    state: "running",
    warning1Seconds: 60,
    warning2Seconds: 60,
    colors: { normal: "n", warning1: "1", warning2: "2", overtime: "o" },
  } as TimerState;
  assert.equal(phase(s, 60000).color, "2");
  assert.equal(phase(s, 0).color, "o");
  assert.equal(
    phase({ ...s, warning1Seconds: 0, warning2Seconds: 0 }, 1).color,
    "n",
  );
});
test("request IDs work without secure-context randomUUID", () => {
  const a = requestID();
  assert.match(a, /^[0-9a-f]{32}$/);
  assert.notEqual(a, requestID());
});
