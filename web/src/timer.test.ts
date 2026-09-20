import { test } from "node:test";
import assert from "node:assert/strict";
import {
  formatTime,
  matches,
  defaultBindings,
  elapsedPercent,
  phase,
  foregroundColor,
  requestID,
  restoreBindings,
  storeBindings,
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
test("elapsed gauge clamps adjustments and overtime", () => {
  assert.equal(elapsedPercent(600, 600000), 0);
  assert.equal(elapsedPercent(600, 300000), 50);
  assert.equal(elapsedPercent(600, 0), 100);
  assert.equal(elapsedPercent(600, -1000), 100);
  assert.equal(elapsedPercent(600, 660000), 0);
});
test("stage foreground contrasts with its background", () => {
  assert.equal(foregroundColor("#f2cc60"), "#101218");
  assert.equal(foregroundColor("#4b5263"), "#ffffff");
  assert.equal(foregroundColor("invalid"), "#ffffff");
});
test("request IDs work without secure-context randomUUID", () => {
  const a = requestID();
  assert.match(a, /^[0-9a-f]{32}$/);
  assert.notEqual(a, requestID());
});
test("key bindings round-trip without persisting presentation labels", () => {
  const changed = defaultBindings.map((binding) => ({ ...binding }));
  changed[0].code = "KeyS";
  const stored = storeBindings(changed);
  assert.equal("label" in stored[0], false);
  assert.equal(restoreBindings(stored)[0].code, "KeyS");
  assert.equal(restoreBindings(stored)[0].label, "開始 / 再開");
});
