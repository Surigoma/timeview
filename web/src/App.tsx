import { useEffect, useRef, useState } from "react";
import { ControlScreen } from "./ControlScreen";
import { SettingsScreen } from "./SettingsScreen";
import { Stage } from "./Stage";
import {
  defaultBindings,
  matches,
  restoreBindings,
  storeBindings,
} from "./timer";
import type { Binding } from "./timer";
import { useTimer } from "./useTimer";

export default function App() {
  const timer = useTimer();
  const display = location.pathname === "/display";
  const [tab, setTab] = useState(
    location.pathname === "/settings" ? "settings" : "control",
  );
  const [bindings, setBindings] = useState<Binding[]>(() =>
    defaultBindings.map((binding) => ({ ...binding })),
  );
  const [keypad, setKeypad] = useState(false);
  const [lastKey, setLastKey] = useState("入力待ち");
  const [presets, setPresets] = useState<string[]>([]);
  const [record, setRecord] = useState<string | null>(null);
  const lastInput = useRef({ action: "", at: 0 });
  const configGeneration = useRef(0);
  const boot = timer.state?.instanceId;
  const version = timer.state?.version;

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
    };
    window.addEventListener("blur", off);
    document.addEventListener("visibilitychange", off);
    return () => {
      window.removeEventListener("blur", off);
      document.removeEventListener("visibilitychange", off);
    };
  }, []);
  useEffect(() => {
    if (!timer.connected) setKeypad(false);
  }, [timer.connected]);

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
          timer.setError("リセットには修飾キーが必要です");
          return;
        }
        if (
          bindings.some(
            (other) => other.action !== record && matches(other, event),
          )
        ) {
          timer.setError("このキーはすでに割り当てられています");
          return;
        }
        const next = bindings.map((other) =>
          other.action === record ? binding : other,
        );
        setRecord(null);
        timer.setError("");
        void timer
          .send("/bindings", { bindings: storeBindings(next) }, "PUT")
          .then((ok) => {
            if (ok) setBindings(next);
          });
        return;
      }
      if (
        !keypad ||
        !timer.connected ||
        !document.hasFocus() ||
        document.hidden
      )
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
      const now = performance.now();
      if (
        timer.busy ||
        (lastInput.current.action === binding.action &&
          now - lastInput.current.at < 300)
      )
        return;
      lastInput.current = { action: binding.action, at: now };
      const sendAction = () => {
        const action = binding.action;
        if (action === "blackout" || action === "reveal")
          return timer.send(
            "/blackout",
            { enabled: action === "blackout" },
            "PUT",
          );
        if (action === "hide" || action === "show")
          return timer.send("/message", { visible: action === "show" }, "PUT");
        if (action === "clear")
          return timer.send("/message", undefined, "DELETE");
        if (action.startsWith("preset")) {
          const text = presets[Number(action.slice(6))];
          if (!text) {
            timer.setError("この定型文は未設定です");
            return Promise.resolve(false);
          }
          return timer.send("/message", { text }, "PUT");
        }
        if (action === "add" || action === "subtract")
          return timer.send("/commands", {
            command: "adjust",
            deltaSeconds: action === "add" ? 60 : -60,
          });
        return timer.send("/commands", { command: action });
      };
      void sendAction().then((ok) =>
        setLastKey(`${binding.label} · ${ok ? "実行済み" : "未実行"}`),
      );
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [display, record, bindings, keypad, timer, presets]);

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
            {timer.connected ? "接続中" : "接続待ち"}
          </span>
          <a
            className="button small"
            href="/display"
            target="_blank"
            rel="noreferrer"
          >
            演台画面を開く ↗
          </a>
        </div>
      </header>
      <nav aria-label="メイン">
        <button
          className={tab === "control" ? "selected" : ""}
          onClick={() => {
            setTab("control");
            setKeypad(false);
            setRecord(null);
          }}
        >
          タイマー操作
        </button>
        <button
          className={tab === "settings" ? "selected" : ""}
          onClick={() => {
            setTab("settings");
            setKeypad(false);
          }}
        >
          設定・外部連携
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
                ? "上映を、静かに支える。"
                : "操作環境を整える。"}
            </h1>
          </div>
          <p>単一タイマー · 分離ネットワーク</p>
        </div>
        {timer.notice && (
          <div role="status" className="notice">
            {timer.notice}
          </div>
        )}
        {timer.error && (
          <div role="alert" className="error">
            {timer.error}
            <button onClick={() => timer.setError("")}>閉じる</button>
          </div>
        )}
        {!timer.connected && (
          <div className="notice">
            サーバーに接続しています。接続が戻るまで操作できません。
          </div>
        )}
        {timer.state && tab === "control" && (
          <ControlScreen
            timer={timer}
            presets={presets}
            bindings={bindings}
            keypad={keypad}
            setKeypad={setKeypad}
            lastKey={lastKey}
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
          />
        )}
      </main>
      <footer>
        TIMEVIEW <span>HTTP · オフライン運用 / 設定はJSON保存</span>
      </footer>
    </div>
  );
}
