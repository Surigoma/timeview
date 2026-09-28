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

export type Binding = {
  action: string;
  label: string;
  code: string;
  ctrl: boolean;
  shift: boolean;
  alt: boolean;
  meta: boolean;
  midi?: MidiBinding;
};
export type MidiBinding = { status: number; data1: number };
export type StoredBinding = Omit<Binding, "label">;
const bind = (action: string, code: string, ctrl = false): Binding => ({
  action,
  label: bindingLabel(action),
  code,
  ctrl,
  shift: false,
  alt: false,
  meta: false,
});
export const defaultBindings: Binding[] = [
  bind("start", "NumpadEnter"),
  bind("pause", "NumpadDecimal"),
  bind("add", "NumpadAdd"),
  bind("subtract", "NumpadSubtract"),
  bind("reset", "Numpad0", true),
  bind("hide", "Numpad0"),
  bind("show", "NumpadMultiply"),
  bind("clear", "NumpadDivide"),
  bind("blackout", "NumpadSubtract", true),
  bind("reveal", "NumpadAdd", true),
  ...Array.from({ length: 9 }, (_, i) => bind(`preset${i}`, `Numpad${i + 1}`)),
];
export function bindingLabel(action: string, language?: Language) {
  if (action.startsWith("preset"))
    return t(
      "actions.preset",
      { number: Number(action.slice(6)) + 1 },
      language,
    );
  return t(`actions.${action}`, undefined, language);
}
export function storeBindings(bindings: Binding[]): StoredBinding[] {
  return bindings.map((binding) => ({
    action: binding.action,
    code: binding.code,
    ctrl: binding.ctrl,
    shift: binding.shift,
    alt: binding.alt,
    meta: binding.meta,
    ...(binding.midi ? { midi: binding.midi } : {}),
  }));
}
export function restoreBindings(bindings: StoredBinding[]): Binding[] {
  return defaultBindings.map((fallback) => ({
    ...fallback,
    label: bindingLabel(fallback.action),
    ...bindings.find((binding) => binding.action === fallback.action),
  }));
}
export type KeyInput = {
  code: string;
  ctrlKey: boolean;
  shiftKey: boolean;
  altKey: boolean;
  metaKey: boolean;
  repeat: boolean;
  isComposing: boolean;
};
export function matches(b: Binding, e: KeyInput) {
  return (
    !e.repeat &&
    !e.isComposing &&
    b.code === e.code &&
    b.ctrl === e.ctrlKey &&
    b.shift === e.shiftKey &&
    b.alt === e.altKey &&
    b.meta === e.metaKey
  );
}
export function keyLabel(b: Binding) {
  return [
    b.ctrl ? "Ctrl" : "",
    b.shift ? "Shift" : "",
    b.alt ? "Alt" : "",
    b.meta ? "Meta" : "",
    b.code.replace("Numpad", "Num "),
  ]
    .filter(Boolean)
    .join(" + ");
}

export function midiInput(data: ArrayLike<number>) {
  if (data.length < 3) return null;
  let status = data[0];
  const kind = status & 0xf0;
  if (kind === 0x80) status = 0x90 | (status & 0x0f);
  else if (kind !== 0x90 && kind !== 0xb0) return null;
  return {
    binding: { status, data1: data[1] } satisfies MidiBinding,
    active: kind !== 0x80 && data[2] > 0,
  };
}

export function midiMatches(binding: Binding, midi: MidiBinding) {
  return (
    binding.midi?.status === midi.status && binding.midi.data1 === midi.data1
  );
}

export function midiLabel(midi?: MidiBinding) {
  if (!midi) return "MIDI Learn";
  const type = (midi.status & 0xf0) === 0x90 ? "Note" : "CC";
  return `${type} ${midi.data1} · Ch ${(midi.status & 0x0f) + 1}`;
}

// crypto.randomUUID is unavailable on plain HTTP LAN addresses in some browsers.
export function requestID() {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (x) => x.toString(16).padStart(2, "0")).join("");
}
