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
  blackout: boolean;
  flash: boolean;
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
  if (remaining <= 0) return { label: "時間超過", color: s.colors.overtime };
  if (s.warning2Seconds > 0 && remaining <= s.warning2Seconds * 1000)
    return { label: "第2警告", color: s.colors.warning2 };
  if (s.warning1Seconds > 0 && remaining <= s.warning1Seconds * 1000)
    return { label: "第1警告", color: s.colors.warning1 };
  return {
    label:
      s.state === "running"
        ? "計測中"
        : s.state === "paused"
          ? "一時停止"
          : "待機中",
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
};
const bind = (
  action: string,
  label: string,
  code: string,
  ctrl = false,
): Binding => ({
  action,
  label,
  code,
  ctrl,
  shift: false,
  alt: false,
  meta: false,
});
export const defaultBindings: Binding[] = [
  bind("start", "開始 / 再開", "NumpadEnter"),
  bind("pause", "一時停止", "NumpadDecimal"),
  bind("add", "+1分", "NumpadAdd"),
  bind("subtract", "−1分", "NumpadSubtract"),
  bind("reset", "リセット", "Numpad0", true),
  bind("hide", "カンペ非表示", "Numpad0"),
  bind("show", "カンペ再表示", "NumpadMultiply"),
  bind("clear", "カンペ消去", "NumpadDivide"),
  bind("blackout", "暗転", "NumpadSubtract", true),
  bind("reveal", "暗転解除", "NumpadAdd", true),
  ...Array.from({ length: 9 }, (_, i) =>
    bind(`preset${i}`, `定型文 ${i + 1}`, `Numpad${i + 1}`),
  ),
];
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

// crypto.randomUUID is unavailable on plain HTTP LAN addresses in some browsers.
export function requestID() {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (x) => x.toString(16).padStart(2, "0")).join("");
}
