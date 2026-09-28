import { useCallback, useEffect, useRef, useState } from "react";
import { Stage } from "./components/Stage";
import { setLanguage, t } from "./i18n";
import { ControlScreen } from "./screens/ControlScreen";
import { LogsScreen } from "./screens/LogsScreen";
import { SettingsScreen } from "./screens/SettingsScreen";
import { TouchScreen } from "./screens/TouchScreen";
import {
  bindingLabel,
  defaultBindings,
  matches,
  midiInput,
  midiMatches,
  restoreBindings,
  storeBindings,
} from "./timer/timer";
import type { Binding } from "./timer/timer";
import { useTimer } from "./timer/useTimer";
import type { TimerConnection } from "./timer/useTimer";

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

export default function App() {
  const timer = useTimer();
  const { busy, connected, send, setError } = timer;
  const display = location.pathname === "/display";
  const touch = location.pathname === "/touch";
  const [tab, setTab] = useState(
    location.pathname === "/settings"
      ? "settings"
      : location.pathname === "/logs"
        ? "logs"
        : "control",
  );
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
    if (display || !boot) return;
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
  }, [boot, version, display]);

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
    if (display) return;
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
    display,
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

  if (display) {
    return timer.state ? (
      <Stage
        state={timer.state}
        remaining={timer.remaining}
        connected={timer.connected}
      />
    ) : (
      <div className="stage blackout" />
    );
  }

  if (touch) return <TouchScreen timer={timer} presets={presets} />;

  return (
    <div className="app-shell">
      <header>
        <a className="brand" href="/">
          <span className="brand-icon">◷</span> TimeView{" "}
          <span className="brand-tag">SCREENING TIMER</span>
        </a>
        <div className="header-right">
          <span className={`connection ${timer.connected ? "online" : ""}`}>
            <i />
            {timer.connected ? t("common.connected") : t("common.connecting")}
          </span>
          <a className="button small" href="/touch">
            {t("app.touch")}
          </a>
          <a
            className="button small"
            href="/display"
            target="_blank"
            rel="noreferrer"
          >
            {t("app.openDisplay")}
          </a>
        </div>
      </header>
      <nav aria-label={t("app.mainNav")}>
        <button
          className={tab === "control" ? "selected" : ""}
          onClick={() => {
            setTab("control");
            setKeypad(false);
            setRecord(null);
            setMidiRecord(null);
          }}
        >
          {t("app.control")}
        </button>
        <button
          className={tab === "settings" ? "selected" : ""}
          onClick={() => {
            setTab("settings");
            setKeypad(false);
            setMidiRecord(null);
          }}
        >
          {t("app.settings")}
        </button>
        <button
          className={tab === "logs" ? "selected" : ""}
          onClick={() => {
            setTab("logs");
            setKeypad(false);
            setRecord(null);
            setMidiRecord(null);
          }}
        >
          {t("app.logs")}
        </button>
        <span>LOCAL / JSON CONFIG</span>
      </nav>
      <main>
        <div className="page-heading">
          <div>
            <p className="eyebrow">
              {tab === "control" ? "CONTROL ROOM" : "PREFERENCES"}
            </p>
            <h1>
              {tab === "control"
                ? t("app.controlHeading")
                : tab === "settings"
                  ? t("app.settingsHeading")
                  : t("app.logsHeading")}
            </h1>
          </div>
          <p>{t("app.environment")}</p>
        </div>
        {timer.notice && (
          <div role="status" className="notice">
            {timer.notice}
          </div>
        )}
        {timer.error && (
          <div role="alert" className="error">
            {timer.error}
            <button onClick={() => timer.setError("")}>
              {t("common.close")}
            </button>
          </div>
        )}
        {!timer.connected && (
          <div className="notice">{t("app.reconnecting")}</div>
        )}
        {timer.state && tab === "control" && (
          <ControlScreen
            timer={timer}
            presets={presets}
            bindings={bindings}
            keypad={keypad}
            setKeypad={setKeypad}
            lastKey={lastKey}
            midiEnabled={midiEnabled}
            midiSupported={"requestMIDIAccess" in navigator}
            toggleMidi={() => {
              if (midiEnabled) setMidiEnabled(false);
              else void connectMidi();
            }}
            lastMidi={lastMidi}
          />
        )}
        {timer.state && tab === "settings" && (
          <SettingsScreen
            timer={timer}
            presets={presets}
            bindings={bindings}
            record={record}
            setRecord={(action) => {
              if (action) setKeypad(false);
              setRecord(action);
            }}
            setBindings={setBindings}
            midiRecord={midiRecord}
            setMidiRecord={(action) => {
              setRecord(null);
              setMidiRecord(action);
            }}
            connectMidi={connectMidi}
          />
        )}
        {tab === "logs" && <LogsScreen language={language} />}
      </main>
      <footer>
        TIMEVIEW <span>{t("app.footer")}</span>
      </footer>
    </div>
  );
}
