import { t } from "../i18n.ts";
import type { Language } from "../i18n.ts";

export type TimerState = {
  instanceId: string;
  version: number;
  state: "idle" | "running" | "paused";
  durationSeconds: number;
  remainingMs: number;
  serverTimeMs: number;
  warning1Seconds: number;
  warning2Seconds: number;
  message: { text: string; visible: boolean };
  displayMode: "timer" | "timer_and_message" | "message";
  language: "ja" | "en";
  logLevel: "debug" | "info" | "warn" | "error";
  blackout: boolean;
  flash: boolean;
  browserOnly: boolean;
  colors: {
    normal: string;
    warning1: string;
    warning2: string;
    overtime: string;
  };
};

export function formatTime(ms: number): string {
  const sec = ms > 0 ? Math.ceil(ms / 1000) : Math.floor(Math.abs(ms) / 1000);
  const hours = Math.floor(sec / 3600);
  const minutes = Math.floor(sec / 60) % 60;
  const seconds = sec % 60;
  return `${ms <= -1000 ? "−" : ""}${hours ? `${hours}:` : ""}${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}

export function phase(s: TimerState, remaining: number) {
  const language = s.language ?? "ja";
  if (remaining <= 0)
    return {
      label: t("timer.overtime", undefined, language),
      color: s.colors.overtime,
    };
  if (s.warning2Seconds > 0 && remaining <= s.warning2Seconds * 1000)
    return {
      label: t("timer.warning2", undefined, language),
      color: s.colors.warning2,
    };
  if (s.warning1Seconds > 0 && remaining <= s.warning1Seconds * 1000)
    return {
      label: t("timer.warning1", undefined, language),
      color: s.colors.warning1,
    };
  return {
    label:
      s.state === "running"
        ? t("timer.running", undefined, language)
        : s.state === "paused"
          ? t("timer.paused", undefined, language)
          : t("timer.idle", undefined, language),
    color: s.colors.normal,
  };
}

export function elapsedPercent(durationSeconds: number, remainingMs: number) {
  if (durationSeconds <= 0) return 0;
  return Math.max(
    0,
    Math.min(
      100,
      ((durationSeconds * 1000 - remainingMs) / (durationSeconds * 1000)) * 100,
    ),
  );
}

export function warningGauge(
  durationSeconds: number,
  warning1Seconds: number,
  warning2Seconds: number,
  remainingMs: number,
  language: Language = "ja",
) {
  const warning1Start =
    warning1Seconds > 0
      ? elapsedPercent(durationSeconds, warning1Seconds * 1000)
      : 100;
  const warning2Start =
    warning2Seconds > 0
      ? elapsedPercent(durationSeconds, warning2Seconds * 1000)
      : 100;
  const thresholds = [
    ...(warning1Seconds > warning2Seconds
      ? [
          {
            label: t("timer.untilWarning1", undefined, language),
            remainingMs: warning1Seconds * 1000,
          },
        ]
      : []),
    ...(warning2Seconds > 0
      ? [
          {
            label: t("timer.untilWarning2", undefined, language),
            remainingMs: warning2Seconds * 1000,
          },
        ]
      : []),
    { label: t("timer.untilEnd", undefined, language), remainingMs: 0 },
  ];
  const next = thresholds.find(
    (threshold) => remainingMs > threshold.remainingMs,
  );
  return {
    warning1Start,
    warning2Start,
    next: next
      ? {
          label: next.label,
          remainingMs: remainingMs - next.remainingMs,
        }
      : null,
  };
}

export function foregroundColor(background: string) {
  const value = background.match(/^#([0-9a-f]{6})$/i)?.[1];
  if (!value) return "#ffffff";
  const [r, g, b] = [0, 2, 4].map((i) =>
    Number.parseInt(value.slice(i, i + 2), 16),
  );
  return (r * 299 + g * 587 + b * 114) / 1000 >= 150 ? "#101218" : "#ffffff";
}
