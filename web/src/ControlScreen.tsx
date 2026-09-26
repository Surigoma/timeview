import { useState } from "react";
import { Stage } from "./Stage";
import { formatTime, keyLabel, phase } from "./timer";
import type { Binding } from "./timer";
import type { TimerConnection } from "./useTimer";

const stateNames = {
  idle: "待機中",
  running: "計測中",
  paused: "一時停止",
};

export function ControlScreen({
  timer,
  presets,
  bindings,
  keypad,
  setKeypad,
  lastKey,
}: {
  timer: TimerConnection;
  presets: string[];
  bindings: Binding[];
  keypad: boolean;
  setKeypad: (enabled: boolean) => void;
  lastKey: string;
}) {
  const [overflow, setOverflow] = useState(false);
  const state = timer.state!;
  return (
    <div className="control-grid">
      <section className="panel timer-panel">
        <div className="panel-title">
          <h2>タイマー</h2>
          <span className="badge">{stateNames[state.state]}</span>
        </div>
        <div
          className="operator-time"
          style={{ color: phase(state, timer.remaining).color }}
        >
          {formatTime(timer.remaining)}
        </div>
        <div className="timer-meta">
          <span>持ち時間 {formatTime(state.durationSeconds * 1000)}</span>
          <span>{phase(state, timer.remaining).label}</span>
        </div>
        <div className="progress">
          <span
            style={{
              width: `${Math.max(0, Math.min(100, timer.remaining / (state.durationSeconds * 10)))}%`,
              background: phase(state, timer.remaining).color,
            }}
          />
        </div>
        <fieldset
          disabled={!timer.connected || timer.busy}
          className="transport"
        >
          <button
            className="primary"
            onClick={() => void timer.send("/commands", { command: "start" })}
          >
            ▶ {state.state === "paused" ? "再開" : "開始"}
          </button>
          <button
            onClick={() => void timer.send("/commands", { command: "pause" })}
          >
            Ⅱ 一時停止
          </button>
          <button
            onClick={() => {
              if (
                state.state !== "running" ||
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
        <div className={`blackout-control ${state.blackout ? "active" : ""}`}>
          <div>
            <strong>{state.blackout ? "● 暗転中" : "○ 演台表示中"}</strong>
            <p>暗転中も計測を継続します</p>
          </div>
          <fieldset disabled={!timer.connected || timer.busy}>
            <button
              className={state.blackout ? "current" : ""}
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
          <span className="badge">{state.blackout ? "BLACKOUT" : "LIVE"}</span>
        </div>
        <Stage
          state={state}
          remaining={timer.remaining}
          connected={timer.connected}
          preview
          onOverflow={setOverflow}
        />
        <p className="hint">
          {state.blackout
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
          {bindings.slice(0, 4).map((binding) => (
            <div key={binding.action}>
              <span>{binding.label}</span>
              <kbd>{keyLabel(binding)}</kbd>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}

function MessagePanel({
  timer,
  presets,
}: {
  timer: TimerConnection;
  presets: string[];
}) {
  const [draft, setDraft] = useState("");
  const count = Array.from(draft).length;
  const state = timer.state!;
  return (
    <section className="panel message-panel">
      <div className="panel-title">
        <h2>カンペ</h2>
        <span className="badge">
          {state.message.visible ? "表示設定 ON" : "非表示"}
        </span>
      </div>
      <label htmlFor="message">送信するメッセージ</label>
      <textarea
        id="message"
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
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
        <p>{state.message.text || "まだ送信していません"}</p>
      </div>
      <fieldset disabled={!timer.connected || timer.busy}>
        <button
          onClick={() => void timer.send("/message", { visible: false }, "PUT")}
        >
          非表示
        </button>
        <button
          disabled={!state.message.text}
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
            (text, index) =>
              text && (
                <button
                  key={index}
                  disabled={!timer.connected || timer.busy}
                  onClick={() => void timer.send("/message", { text }, "PUT")}
                >
                  <kbd>{index + 1}</kbd>
                  {text}
                </button>
              ),
          )}
        </div>
      )}
    </section>
  );
}
