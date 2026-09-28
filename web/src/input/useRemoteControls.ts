import { useCallback, useEffect, useRef, useState } from "react";
import { setLanguage, t } from "../i18n";
import type { TimerConnection } from "../timer/useTimer";
import {
  bindingLabel,
  defaultBindings,
  matches,
  midiInput,
  midiMatches,
  restoreBindings,
  storeBindings,
} from "./bindings";
import type { Binding } from "./bindings";

function sendAction(
  action: string,
  send: TimerConnection["send"],
  setError: TimerConnection["setError"],
  presets: string[],
) {
  if (action === "blackout" || action === "reveal")
    return send("/blackout", { enabled: action === "blackout" }, "PUT");
  if (action === "hide" || action === "show")
    return send("/message", { visible: action === "show" }, "PUT");
  if (action === "clear") return send("/message", undefined, "DELETE");
  if (action.startsWith("preset")) {
    const text = presets[Number(action.slice(6))];
    if (!text) {
      setError(t("app.unsetPreset"));
      return Promise.resolve(false);
    }
    return send("/message", { text }, "PUT");
  }
  if (action === "add" || action === "subtract")
    return send("/commands", {
      command: "adjust",
      deltaSeconds: action === "add" ? 60 : -60,
    });
  return send("/commands", { command: action });
}

export function useRemoteControls(timer: TimerConnection, disabled: boolean) {
  const { busy, connected, send, setError } = timer;
  const [bindings, setBindings] = useState<Binding[]>(() =>
    defaultBindings.map((binding) => ({ ...binding })),
  );
  const [keypad, setKeypad] = useState(false);
  const [lastKey, setLastKey] = useState(t("app.inputWaiting"));
  const [midiEnabled, setMidiEnabled] = useState(false);
  const [midiRecord, setMidiRecord] = useState<string | null>(null);
  const [lastMidi, setLastMidi] = useState(t("app.inputWaiting"));
  const [presets, setPresets] = useState<string[]>([]);
  const [record, setRecord] = useState<string | null>(null);
  const lastInput = useRef({ action: "", at: 0 });
  const midiAccess = useRef<MIDIAccess | null>(null);
  const activeMidi = useRef(new Set<string>());
  const configGeneration = useRef(0);
  const boot = timer.state?.instanceId;
  const version = timer.state?.version;
  const language = timer.state?.language ?? "ja";

  useEffect(() => {
    setLanguage(language);
    setLastKey(t("app.inputWaiting"));
    setLastMidi(t("app.inputWaiting"));
  }, [language]);

  const connectMidi = useCallback(async () => {
    if (!("requestMIDIAccess" in navigator)) {
      setError(t("errors.midiUnsupported"));
      return false;
    }
    try {
      midiAccess.current ??= await navigator.requestMIDIAccess();
      setMidiEnabled(true);
      return true;
    } catch {
      setError(t("errors.midiDenied"));
      return false;
    }
  }, [setError]);

  const executeAction = useCallback(
    (action: string, midi = false) => {
      const now = performance.now();
      if (
        busy ||
        (lastInput.current.action === action &&
          now - lastInput.current.at < 300)
      )
        return;
      lastInput.current = { action, at: now };
      void sendAction(action, send, setError, presets).then((ok) =>
        (midi ? setLastMidi : setLastKey)(
          `${bindingLabel(action)} · ${ok ? t("app.executed") : t("app.notExecuted")}`,
        ),
      );
    },
    [busy, presets, send, setError],
  );

  useEffect(() => {
    if (disabled || !boot) return;
    let cancelled = false;
    const sequence = ++configGeneration.current;
    const load = async (path: string) => {
      const response = await fetch(path, { signal: AbortSignal.timeout(4000) });
      if (!response.ok) throw Error();
      return response.json();
    };
    Promise.all([load("/api/v1/timer/presets"), load("/api/v1/timer/bindings")])
      .then(([presetData, bindingData]) => {
        if (!cancelled && sequence === configGeneration.current) {
          setPresets(presetData.presets);
          setBindings(restoreBindings(bindingData.bindings));
        }
      })
      .catch(() => {});
    return () => {
      cancelled = true;
    };
  }, [boot, version, disabled]);

  useEffect(() => {
    const off = () => {
      setKeypad(false);
      setRecord(null);
      setMidiRecord(null);
    };
    window.addEventListener("blur", off);
    document.addEventListener("visibilitychange", off);
    return () => {
      window.removeEventListener("blur", off);
      document.removeEventListener("visibilitychange", off);
    };
  }, []);

  useEffect(() => {
    if (!connected) {
      setKeypad(false);
      setMidiEnabled(false);
    }
  }, [connected]);

  useEffect(() => {
    if (disabled) return;
    const handler = (event: KeyboardEvent) => {
      if (record) {
        event.preventDefault();
        if (event.key === "Escape") {
          setRecord(null);
          return;
        }
        if (
          event.repeat ||
          event.isComposing ||
          ["Control", "Shift", "Alt", "Meta"].includes(event.key)
        )
          return;
        const binding: Binding = {
          ...bindings.find((item) => item.action === record)!,
          code: event.code,
          ctrl: event.ctrlKey,
          shift: event.shiftKey,
          alt: event.altKey,
          meta: event.metaKey,
        };
        if (
          binding.action === "reset" &&
          !binding.ctrl &&
          !binding.shift &&
          !binding.alt &&
          !binding.meta
        ) {
          setError("リセットには修飾キーが必要です");
          return;
        }
        if (
          bindings.some(
            (other) => other.action !== record && matches(other, event),
          )
        ) {
          setError("このキーはすでに割り当てられています");
          return;
        }
        const next = bindings.map((other) =>
          other.action === record ? binding : other,
        );
        setRecord(null);
        setError("");
        void send("/bindings", { bindings: storeBindings(next) }, "PUT").then(
          (ok) => {
            if (ok) setBindings(next);
          },
        );
        return;
      }
      if (!keypad || !connected || !document.hasFocus() || document.hidden)
        return;
      const target = event.target;
      if (
        target instanceof HTMLElement &&
        (target.closest("input,textarea,select") || target.isContentEditable)
      )
        return;
      const binding = bindings.find((item) => matches(item, event));
      if (!binding) return;
      event.preventDefault();
      executeAction(binding.action);
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [
    disabled,
    record,
    bindings,
    keypad,
    connected,
    setError,
    send,
    executeAction,
  ]);

  useEffect(() => {
    const access = midiAccess.current;
    if (!access) return;
    const handler = (event: MIDIMessageEvent) => {
      if (!event.data) return;
      const input = midiInput(event.data);
      if (!input) return;
      const id = `${input.binding.status}:${input.binding.data1}`;
      if (!input.active) {
        activeMidi.current.delete(id);
        return;
      }
      if (activeMidi.current.has(id)) return;
      activeMidi.current.add(id);
      if (midiRecord) {
        if (
          bindings.some(
            (binding) =>
              binding.action !== midiRecord &&
              midiMatches(binding, input.binding),
          )
        ) {
          setError(t("errors.duplicateMidi"));
          return;
        }
        const next = bindings.map((binding) =>
          binding.action === midiRecord
            ? { ...binding, midi: input.binding }
            : binding,
        );
        setMidiRecord(null);
        setError("");
        void send("/bindings", { bindings: storeBindings(next) }, "PUT").then(
          (ok) => {
            if (ok) setBindings(next);
          },
        );
        return;
      }
      if (!midiEnabled || !connected) return;
      const binding = bindings.find((item) => midiMatches(item, input.binding));
      if (binding) executeAction(binding.action, true);
    };
    const attach = () =>
      access.inputs.forEach((input) => {
        input.onmidimessage = handler;
      });
    const stateChange = () => {
      activeMidi.current.clear();
      attach();
    };
    attach();
    access.addEventListener("statechange", stateChange);
    return () => {
      access.removeEventListener("statechange", stateChange);
      access.inputs.forEach((input) => {
        if (input.onmidimessage === handler) input.onmidimessage = null;
      });
    };
  }, [
    bindings,
    executeAction,
    midiEnabled,
    midiRecord,
    connected,
    send,
    setError,
  ]);

  return {
    bindings,
    setBindings,
    keypad,
    setKeypad,
    lastKey,
    midiEnabled,
    setMidiEnabled,
    midiRecord,
    setMidiRecord,
    lastMidi,
    presets,
    record,
    setRecord,
    connectMidi,
  };
}
