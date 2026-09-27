import { formatTime, phase } from "./timer";
import type { TimerConnection } from "./useTimer";

const stateNames = {
  idle: "待機中",
  running: "計測中",
  paused: "一時停止",
};

export function TouchScreen({
  timer,
  presets,
}: {
  timer: TimerConnection;
  presets: string[];
}) {
  const state = timer.state;
  if (!state) return <main className="touch-screen">接続しています…</main>;
  const disabled = !timer.connected || timer.busy;
  return (
    <main className="touch-screen">
      <header className="touch-header">
        <a className="brand" href="/">
          <span className="brand-icon">◷</span> TimeView
        </a>
        <span className={`connection ${timer.connected ? "online" : ""}`}>
          <i /> {timer.connected ? "接続中" : "接続待ち"}
        </span>
      </header>
      {timer.error && (
        <div role="alert" className="error">
          {timer.error}
          <button onClick={() => timer.setError("")}>閉じる</button>
        </div>
      )}
      <section className="touch-timer" aria-label="タイマー操作">
        <div>
          <span className="badge">{stateNames[state.state]}</span>
          <div
            className="touch-time"
            style={{ color: phase(state, timer.remaining).color }}
          >
            {formatTime(timer.remaining)}
          </div>
          <p>{phase(state, timer.remaining).label}</p>
        </div>
        <fieldset disabled={disabled} className="touch-transport">
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
          <button
            className="touch-reset"
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
        </fieldset>
      </section>
      <section className="touch-actions" aria-label="演台操作">
        <fieldset disabled={disabled} className="touch-blackout">
          <button
            className={state.blackout ? "current" : ""}
            onClick={() =>
              void timer.send("/blackout", { enabled: true }, "PUT")
            }
          >
            ● 暗転
          </button>
          <button
            className={!state.blackout ? "primary" : ""}
            onClick={() =>
              void timer.send("/blackout", { enabled: false }, "PUT")
            }
          >
            ○ 暗転解除
          </button>
        </fieldset>
        <fieldset disabled={disabled} className="touch-message-actions">
          <button
            onClick={() =>
              void timer.send("/message", { visible: false }, "PUT")
            }
          >
            カンペ非表示
          </button>
          <button
            disabled={disabled || !state.message.text}
            onClick={() =>
              void timer.send("/message", { visible: true }, "PUT")
            }
          >
            カンペ再表示
          </button>
        </fieldset>
      </section>
      {presets.some(Boolean) && (
        <section className="touch-presets" aria-label="定型文">
          <h2>定型文</h2>
          <div>
            {presets.map(
              (text, index) =>
                text && (
                  <button
                    key={index}
                    disabled={disabled}
                    onClick={() =>
                      void timer.send("/message", { text }, "PUT")
                    }
                  >
                    <kbd>{index + 1}</kbd> {text}
                  </button>
                ),
            )}
          </div>
        </section>
      )}
      <footer className="touch-footer">
        <a href="/">通常の操作画面</a>
        <a href="/display" target="_blank" rel="noreferrer">
          演台画面 ↗
        </a>
      </footer>
    </main>
  );
}
