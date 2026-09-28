import assert from "node:assert/strict";
import { test } from "node:test";
import {
  elapsedPercent,
  foregroundColor,
  formatTime,
  phase,
  warningGauge,
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

test("warning precedence and disabled zero thresholds", () => {
  const state = {
    state: "running",
    warning1Seconds: 60,
    warning2Seconds: 60,
    colors: { normal: "n", warning1: "1", warning2: "2", overtime: "o" },
  } as TimerState;
  assert.equal(phase(state, 60000).color, "2");
  assert.equal(phase(state, 0).color, "o");
  assert.equal(
    phase({ ...state, warning1Seconds: 0, warning2Seconds: 0 }, 1).color,
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

test("warning gauge exposes regions and the next threshold", () => {
  assert.deepEqual(warningGauge(600, 180, 60, 600000), {
    warning1Start: 70,
    warning2Start: 90,
    next: { label: "第1警告まで", remainingMs: 420000 },
  });
  assert.deepEqual(warningGauge(600, 180, 60, 180000).next, {
    label: "第2警告まで",
    remainingMs: 120000,
  });
  assert.deepEqual(warningGauge(600, 180, 60, 60000).next, {
    label: "終了まで",
    remainingMs: 60000,
  });
  assert.deepEqual(warningGauge(600, 0, 0, 600000), {
    warning1Start: 100,
    warning2Start: 100,
    next: { label: "終了まで", remainingMs: 600000 },
  });
  assert.equal(warningGauge(600, 180, 60, 0).next, null);
});

test("stage foreground contrasts with its background", () => {
  assert.equal(foregroundColor("#f2cc60"), "#101218");
  assert.equal(foregroundColor("#4b5263"), "#ffffff");
  assert.equal(foregroundColor("invalid"), "#ffffff");
});

test("English localizes timer labels", () => {
  assert.equal(
    phase(
      {
        state: "idle",
        language: "en",
        warning1Seconds: 0,
        warning2Seconds: 0,
        colors: { normal: "n", warning1: "1", warning2: "2", overtime: "o" },
      } as TimerState,
      1000,
    ).label,
    "Idle",
  );
});
