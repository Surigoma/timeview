import { t } from "../i18n.ts";
import type { Language } from "../i18n.ts";

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
