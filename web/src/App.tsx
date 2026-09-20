import { useEffect, useRef, useState } from "react";
import type { CSSProperties, FormEvent } from "react";
import {
  defaultBindings,
  elapsedPercent,
  foregroundColor,
  formatTime,
  keyLabel,
  matches,
  phase,
  restoreBindings,
  storeBindings,
} from "./timer";
import type { Binding, TimerState } from "./timer";
import { useTimer } from "./useTimer";

type Connection = ReturnType<typeof useTimer>;
const names = { idle: "待機中", running: "計測中", paused: "一時停止" };

function Stage({
  state,
  remaining,
  connected,
  preview = false,
  onOverflow,
}: {
  state: TimerState;
  remaining: number;
  connected: boolean;
  preview?: boolean;
  onOverflow?: (v: boolean) => void;
}) {
  const container = useRef<HTMLDivElement>(null);
  const message = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = message.current,
      box = container.current;
    if (!el || !box) {
      onOverflow?.(false);
      return;
    }
    const fit = () => {
      const minimum = preview ? 10 : 24;
      let size = Math.min(120, Math.max(minimum, box.clientWidth / 18));
      el.style.fontSize = `${size}px`;
      while (
        size > minimum &&
        (el.scrollHeight > el.clientHeight + 1 ||
          el.scrollWidth > el.clientWidth + 1)
      ) {
        size -= 1;
        el.style.fontSize = `${size}px`;
      }
      onOverflow?.(
        el.scrollHeight > el.clientHeight + 1 ||
          el.scrollWidth > el.clientWidth + 1,
      );
    };
    const observer = new ResizeObserver(fit);
    observer.observe(box);
    fit();
    return () => observer.disconnect();
  }, [
    state.message.text,
    state.message.visible,
    state.displayMode,
    state.blackout,
    preview,
    onOverflow,
  ]);
  const p = phase(state, remaining);
  const elapsed = elapsedPercent(state.durationSeconds, remaining);
  return (
    <div
      ref={container}
      className={`stage ${preview ? "preview" : ""} ${state.blackout ? "blackout" : ""} ${state.displayMode === "message" ? "message-only" : ""}`}
      style={
        {
          "--stage-background": p.color,
          "--stage-foreground": foregroundColor(p.color),
        } as CSSProperties
      }
    >
      {!state.blackout && (
        <>
          {state.displayMode !== "message" && (
            <div className="time-block">
              <div
                className={`stage-time ${state.flash && remaining <= 0 ? "pulse" : ""}`}
                style={{
                  fontSize: `${Math.min(26, 145 / formatTime(remaining).length)}cqw`,
                }}
              >
                {formatTime(remaining)}
              </div>
            </div>
          )}
          {state.displayMode !== "timer" && state.message.visible && (
            <div ref={message} className="stage-message">
              {state.message.text}
            </div>
          )}
          {!connected && (
            <span className="stage-offline">接続待ち · 推定表示</span>
          )}
          <div
            className="elapsed-gauge"
            role="progressbar"
            aria-label="経過時間"
            aria-valuemin={0}
            aria-valuemax={100}
            aria-valuenow={Math.round(elapsed)}
          >
            <span style={{ width: `${elapsed}%` }} />
          </div>
          {!preview && (
            <button
              className="fullscreen"
              onClick={() => {
                void document.documentElement
                  .requestFullscreen()
                  .catch(() => {});
              }}
            >
              全画面表示
            </button>
          )}
        </>
      )}
    </div>
  );
}

export default function App() {
  const timer = useTimer();
  const display = location.pathname === "/display";
  const [tab, setTab] = useState(
    location.pathname === "/settings" ? "settings" : "control",
  );
  const [bindings, setBindings] = useState<Binding[]>(() =>
    defaultBindings.map((b) => ({ ...b })),
  );
  const [keypad, setKeypad] = useState(false);
  const [lastKey, setLastKey] = useState("入力待ち");
  const [presets, setPresets] = useState<string[]>([]);
  const [overflow, setOverflow] = useState(false);
  const [record, setRecord] = useState<string | null>(null);
  const [toast, setToast] = useState("");
  const lastInput = useRef({ key: "", at: 0 });
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
    const handler = (e: KeyboardEvent) => {
      if (record) {
        e.preventDefault();
        if (e.key === "Escape") {
          setRecord(null);
          return;
        }
        if (
          e.repeat ||
          e.isComposing ||
          ["Control", "Shift", "Alt", "Meta"].includes(e.key)
        )
          return;
        const b: Binding = {
          ...bindings.find((b) => b.action === record)!,
          code: e.code,
          ctrl: e.ctrlKey,
          shift: e.shiftKey,
          alt: e.altKey,
          meta: e.metaKey,
        };
        if (b.action === "reset" && !b.ctrl && !b.shift && !b.alt && !b.meta) {
          timer.setError("リセットには修飾キーが必要です");
          return;
        }
        if (
          bindings.some((other) => other.action !== record && matches(other, e))
        ) {
          timer.setError("このキーはすでに割り当てられています");
          return;
        }
        const next = bindings.map((other) =>
          other.action === record ? b : other,
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
      const target = e.target;
      if (
        target instanceof HTMLElement &&
        (target.closest("input,textarea,select") || target.isContentEditable)
      )
        return;
      const binding = bindings.find((b) => matches(b, e));
      if (!binding) return;
      e.preventDefault();
      const now = performance.now();
      if (
        timer.busy ||
        (lastInput.current.key === e.code && now - lastInput.current.at < 300)
      )
        return;
      lastInput.current = { key: e.code, at: now };
      const sendAction = () => {
        const a = binding.action;
        if (a === "blackout" || a === "reveal")
          return timer.send("/blackout", { enabled: a === "blackout" }, "PUT");
        if (a === "hide" || a === "show")
          return timer.send("/message", { visible: a === "show" }, "PUT");
        if (a === "clear") return timer.send("/message", undefined, "DELETE");
        if (a.startsWith("preset")) {
          const text = presets[Number(a.slice(6))];
          if (!text) {
            timer.setError("この定型文は未設定です");
            return Promise.resolve(false);
          }
          return timer.send("/message", { text }, "PUT");
        }
        if (a === "add" || a === "subtract")
          return timer.send("/commands", {
            command: "adjust",
            deltaSeconds: a === "add" ? 60 : -60,
          });
        return timer.send("/commands", { command: a });
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

  const copy = async (text: string) => {
    // LAN HTTP may not provide navigator.clipboard; the manual field remains usable.
    try {
      await navigator.clipboard.writeText(text);
      setToast("コピーしました");
    } catch {
      setToast("下のURLを選択してコピーしてください");
    }
    setTimeout(() => setToast(""), 3000);
  };
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
          <>
            <div className="control-grid">
              <section className="panel timer-panel">
                <div className="panel-title">
                  <h2>タイマー</h2>
                  <span className="badge">{names[timer.state.state]}</span>
                </div>
                <div
                  className="operator-time"
                  style={{ color: phase(timer.state, timer.remaining).color }}
                >
                  {formatTime(timer.remaining)}
                </div>
                <div className="timer-meta">
                  <span>
                    持ち時間 {formatTime(timer.state.durationSeconds * 1000)}
                  </span>
                  <span>{phase(timer.state, timer.remaining).label}</span>
                </div>
                <div className="progress">
                  <span
                    style={{
                      width: `${Math.max(0, Math.min(100, timer.remaining / (timer.state.durationSeconds * 10)))}%`,
                      background: phase(timer.state, timer.remaining).color,
                    }}
                  />
                </div>
                <fieldset
                  disabled={!timer.connected || timer.busy}
                  className="transport"
                >
                  <button
                    className="primary"
                    onClick={() =>
                      void timer.send("/commands", { command: "start" })
                    }
                  >
                    ▶ {timer.state.state === "paused" ? "再開" : "開始"}
                  </button>
                  <button
                    onClick={() =>
                      void timer.send("/commands", { command: "pause" })
                    }
                  >
                    Ⅱ 一時停止
                  </button>
                  <button
                    onClick={() => {
                      if (
                        timer.state?.state !== "running" ||
                        confirm("タイマーを停止し、持ち時間に戻しますか？")
                      )
                        void timer.send("/commands", { command: "reset" });
                    }}
                  >
                    ↺ リセット
                  </button>
                  <button
                    onClick={() =>
                      void timer.send("/commands", {
                        command: "adjust",
                        deltaSeconds: -60,
                      })
                    }
                  >
                    −1分
                  </button>
                  <button
                    onClick={() =>
                      void timer.send("/commands", {
                        command: "adjust",
                        deltaSeconds: 60,
                      })
                    }
                  >
                    ＋1分
                  </button>
                </fieldset>
                <div
                  className={`blackout-control ${timer.state.blackout ? "active" : ""}`}
                >
                  <div>
                    <strong>
                      {timer.state.blackout ? "● 暗転中" : "○ 演台表示中"}
                    </strong>
                    <p>暗転中も計測を継続します</p>
                  </div>
                  <fieldset disabled={!timer.connected || timer.busy}>
                    <button
                      className={timer.state.blackout ? "current" : ""}
                      onClick={() =>
                        void timer.send("/blackout", { enabled: true }, "PUT")
                      }
                    >
                      暗転
                    </button>
                    <button
                      onClick={() =>
                        void timer.send("/blackout", { enabled: false }, "PUT")
                      }
                    >
                      暗転解除
                    </button>
                  </fieldset>
                </div>
              </section>
              <section className="panel preview-panel">
                <div className="panel-title">
                  <h2>演台プレビュー</h2>
                  <span className="badge">
                    {timer.state.blackout ? "BLACKOUT" : "LIVE"}
                  </span>
                </div>
                <Stage
                  state={timer.state}
                  remaining={timer.remaining}
                  connected={timer.connected}
                  preview
                  onOverflow={setOverflow}
                />
                <p className="hint">
                  {timer.state.blackout
                    ? "演台は黒一色です。暗転解除で現在の内容を表示します。"
                    : "すべての演台画面に同じ内容を表示しています。"}
                </p>
                {overflow && (
                  <p className="warning">
                    カンペが領域に収まりません。短く編集してください。
                  </p>
                )}
              </section>
              <MessagePanel timer={timer} presets={presets} />
              <section className="panel">
                <div className="panel-title">
                  <h2>Keypad</h2>
                  <span className={`badge ${keypad ? "accent" : ""}`}>
                    {keypad ? "有効" : "OFF"}
                  </span>
                </div>
                <p className="hint">
                  この画面を前面にして操作します。フォーカスが外れるとOFFになります。
                </p>
                <button
                  disabled={!timer.connected}
                  className={keypad ? "primary" : ""}
                  onClick={() => setKeypad(!keypad)}
                >
                  {keypad ? "Keypadを無効にする" : "Keypadを有効にする"}
                </button>
                <p aria-live="polite" className="last-key">
                  {lastKey}
                </p>
                <div className="key-summary">
                  {bindings.slice(0, 4).map((b) => (
                    <div key={b.action}>
                      <span>{b.label}</span>
                      <kbd>{keyLabel(b)}</kbd>
                    </div>
                  ))}
                </div>
              </section>
            </div>
          </>
        )}
        {timer.state && tab === "settings" && (
          <div className="settings-grid">
            <SettingsPanel timer={timer} />
            <PresetEditor timer={timer} presets={presets} />
            <section className="panel">
              <div className="panel-title">
                <h2>キー割り当て</h2>
                <button
                  className="small"
                  onClick={() => {
                    const defaults = defaultBindings.map((b) => ({ ...b }));
                    void timer
                      .send(
                        "/bindings",
                        { bindings: storeBindings(defaults) },
                        "PUT",
                      )
                      .then((ok) => {
                        if (ok) setBindings(defaults);
                      });
                    setRecord(null);
                  }}
                >
                  既定に戻す
                </button>
              </div>
              <p className="hint">
                変更を押してキーを入力。Escでキャンセル。変更はJSON設定へ保存します。
              </p>
              <div className="bindings">
                {bindings.map((b) => (
                  <div key={b.action}>
                    <span>{b.label}</span>
                    <button
                      className="small"
                      onClick={() => {
                        setKeypad(false);
                        setRecord(b.action);
                      }}
                    >
                      {record === b.action ? "キーを入力…" : keyLabel(b)}
                    </button>
                  </div>
                ))}
              </div>
            </section>
            <section className="panel">
              <div className="panel-title">
                <h2>外部連携</h2>
                <span className="badge">HTTP</span>
              </div>
              <p className="hint">
                認証不要。同じ分離ネットワークから操作できます。
              </p>
              <label>
                演台URL
                <input
                  readOnly
                  value={`${location.origin}/display`}
                  onFocus={(e) => e.target.select()}
                />
              </label>
              <button
                className="small"
                onClick={() => void copy(`${location.origin}/display`)}
              >
                URLをコピー
              </button>
              <label>
                タイマーAPI
                <input
                  readOnly
                  value={`${location.origin}/api/v1/timer`}
                  onFocus={(e) => e.target.select()}
                />
              </label>
              <p aria-live="polite">{toast}</p>
              <pre>
                {
                  'PUT /api/v1/timer/blackout\nContent-Type: application/json\n\n{"enabled": true}'
                }
              </pre>
              <p className="hint">
                設定・定型文・キー割り当てはJSONへ保存します。再起動時も暗転はONです。
              </p>
            </section>
          </div>
        )}
      </main>
      <footer>
        TIMEVIEW <span>HTTP · オフライン運用 / 設定はJSON保存</span>
      </footer>
    </div>
  );
}

function MessagePanel({
  timer,
  presets,
}: {
  timer: Connection;
  presets: string[];
}) {
  const [draft, setDraft] = useState("");
  const count = Array.from(draft).length;
  const s = timer.state!;
  return (
    <section className="panel message-panel">
      <div className="panel-title">
        <h2>カンペ</h2>
        <span className="badge">
          {s.message.visible ? "表示設定 ON" : "非表示"}
        </span>
      </div>
      <label htmlFor="message">送信するメッセージ</label>
      <textarea
        id="message"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        rows={3}
        placeholder="登壇者へのメッセージを入力"
      />
      <div className="message-toolbar">
        <span className={count > 500 ? "warning" : "hint"}>
          {count} / 500文字 · 未送信
        </span>
        <button
          className="primary"
          disabled={!timer.connected || timer.busy || count > 500}
          onClick={() => void timer.send("/message", { text: draft }, "PUT")}
        >
          送信 ↗
        </button>
      </div>
      <div className="sent-message">
        <span>送信済み</span>
        <p>{s.message.text || "まだ送信していません"}</p>
      </div>
      <fieldset disabled={!timer.connected || timer.busy}>
        <button
          onClick={() => void timer.send("/message", { visible: false }, "PUT")}
        >
          非表示
        </button>
        <button
          disabled={!s.message.text}
          onClick={() => void timer.send("/message", { visible: true }, "PUT")}
        >
          再表示
        </button>
        <button
          onClick={() => void timer.send("/message", undefined, "DELETE")}
        >
          消去
        </button>
      </fieldset>
      {presets.some(Boolean) && (
        <div className="presets">
          {presets.map(
            (text, i) =>
              text && (
                <button
                  key={i}
                  disabled={!timer.connected || timer.busy}
                  onClick={() => void timer.send("/message", { text }, "PUT")}
                >
                  <kbd>{i + 1}</kbd>
                  {text}
                </button>
              ),
          )}
        </div>
      )}
    </section>
  );
}

function SettingsPanel({ timer }: { timer: Connection }) {
  const [editing, setEditing] = useState<TimerState | null>(null);
  const s = editing ?? timer.state!;
  const update = (p: Partial<TimerState>) => setEditing({ ...s, ...p });
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    const body = {
      ...(s.state === "idle" ? { durationSeconds: s.durationSeconds } : {}),
      warning1Seconds: s.warning1Seconds,
      warning2Seconds: s.warning2Seconds,
      displayMode: s.displayMode,
      flash: s.flash,
      colors: s.colors,
    };
    const ok = await timer.send(
      "",
      body,
      "PATCH",
      `"${s.instanceId}:${s.version}"`,
    );
    if (ok) setEditing(null);
  };
  return (
    <section className="panel">
      <div className="panel-title">
        <h2>タイマー・表示設定</h2>
        <span className="badge">{editing ? "編集中" : "同期済み"}</span>
      </div>
      <form onSubmit={(e) => void submit(e)}>
        <fieldset
          disabled={!timer.connected || timer.busy}
          className="form-fields"
        >
          <div className="duration-fields">
            <span>持ち時間（待機中のみ変更可能）</span>
            {(["時", "分", "秒"] as const).map((label, i) => {
              const parts = [
                Math.floor(s.durationSeconds / 3600),
                Math.floor(s.durationSeconds / 60) % 60,
                s.durationSeconds % 60,
              ];
              return (
                <label key={label}>
                  {label}
                  <input
                    type="number"
                    min={0}
                    max={i === 0 ? 24 : 59}
                    required
                    disabled={timer.state?.state !== "idle"}
                    value={parts[i]}
                    onChange={(e) => {
                      parts[i] = Number(e.target.value);
                      update({
                        durationSeconds:
                          parts[0] * 3600 + parts[1] * 60 + parts[2],
                      });
                    }}
                  />
                </label>
              );
            })}
          </div>
          <div className="two-columns">
            <label>
              第1警告（残り秒）
              <input
                type="number"
                min={0}
                max={s.durationSeconds}
                required
                value={s.warning1Seconds}
                onChange={(e) =>
                  update({ warning1Seconds: Number(e.target.value) })
                }
              />
            </label>
            <label>
              第2警告（残り秒）
              <input
                type="number"
                min={0}
                max={s.warning1Seconds}
                required
                value={s.warning2Seconds}
                onChange={(e) =>
                  update({ warning2Seconds: Number(e.target.value) })
                }
              />
            </label>
          </div>
          <label>
            表示モード
            <select
              value={s.displayMode}
              onChange={(e) =>
                update({
                  displayMode: e.target.value as TimerState["displayMode"],
                })
              }
            >
              <option value="timer_and_message">タイマー＋カンペ</option>
              <option value="timer">タイマーのみ</option>
              <option value="message">カンペのみ</option>
            </select>
          </label>
          <div className="colors">
            {(["normal", "warning1", "warning2", "overtime"] as const).map(
              (key, i) => (
                <label key={key}>
                  {["通常", "第1警告", "第2警告", "時間超過"][i]}
                  <input
                    type="color"
                    value={s.colors[key]}
                    onChange={(e) =>
                      update({ colors: { ...s.colors, [key]: e.target.value } })
                    }
                  />
                </label>
              ),
            )}
          </div>
          <label className="checkbox">
            <input
              type="checkbox"
              checked={s.flash}
              onChange={(e) => update({ flash: e.target.checked })}
            />
            時間超過時にゆっくり点滅
          </label>
          <div>
            <button type="submit" className="primary">
              設定を反映
            </button>
            <button type="button" onClick={() => setEditing(null)}>
              最新の設定を読み直す
            </button>
          </div>
        </fieldset>
      </form>
      <p className="hint">
        設定が競合した場合は読み直してから再編集してください。
      </p>
    </section>
  );
}

function PresetEditor({
  timer,
  presets,
}: {
  timer: Connection;
  presets: string[];
}) {
  const [draft, setDraft] = useState<string[] | null>(null);
  const values = draft ?? Array.from({ length: 9 }, (_, i) => presets[i] ?? "");
  return (
    <section className="panel">
      <div className="panel-title">
        <h2>定型文</h2>
        <span className="badge">最大9件</span>
      </div>
      <p className="hint">ボタン・テンキーから即時送信するメッセージです。</p>
      <div className="preset-editor">
        {values.map((text, i) => (
          <label key={i}>
            <kbd>{i + 1}</kbd>
            <input
              value={text}
              placeholder={`定型文 ${i + 1}`}
              onChange={(e) => {
                const next = [...values];
                next[i] = e.target.value;
                setDraft(next);
              }}
            />
          </label>
        ))}
      </div>
      <button
        className="primary"
        disabled={
          !timer.connected ||
          timer.busy ||
          values.some((v) => Array.from(v).length > 500)
        }
        onClick={() => {
          void timer.send("/presets", { presets: values }, "PUT").then((ok) => {
            if (ok) setDraft(null);
          });
        }}
      >
        定型文を反映
      </button>
      <button onClick={() => setDraft(null)}>編集を戻す</button>
    </section>
  );
}
