import { t } from "../i18n";
import { formatTime, phase } from "../timer/timer";
import type { TimerConnection } from "../timer/useTimer";

export function TouchScreen({
  timer,
  presets,
}: {
  timer: TimerConnection;
  presets: string[];
}) {
  const state = timer.state;
  if (!state)
    return <main className="touch-screen">{t("touch.connecting")}</main>;
  const disabled = !timer.connected || timer.busy;
  return (
    <main className="touch-screen">
      <header className="touch-header">
        <a className="brand" href="/">
          <span className="brand-icon">◷</span> TimeView
        </a>
        <span className={`connection ${timer.connected ? "online" : ""}`}>
          <i />{" "}
          {timer.connected ? t("common.connected") : t("common.connecting")}
        </span>
      </header>
      {timer.error && (
        <div role="alert" className="error">
          {timer.error}
          <button onClick={() => timer.setError("")}>
            {t("common.close")}
          </button>
        </div>
      )}
      <section className="touch-timer" aria-label={t("touch.timerControls")}>
        <div>
          <span className="badge">{t(`timer.${state.state}`)}</span>
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
            ▶{" "}
            {state.state === "paused"
              ? t("actions.resume")
              : t("actions.startOnly")}
          </button>
          <button
            onClick={() => void timer.send("/commands", { command: "pause" })}
          >
            Ⅱ {t("actions.pause")}
          </button>
          <button
            onClick={() =>
              void timer.send("/commands", {
                command: "adjust",
                deltaSeconds: -60,
              })
            }
          >
            {t("actions.subtract")}
          </button>
          <button
            onClick={() =>
              void timer.send("/commands", {
                command: "adjust",
                deltaSeconds: 60,
              })
            }
          >
            {t("actions.add")}
          </button>
          <button
            className="touch-reset"
            onClick={() => {
              if (
                state.state !== "running" ||
                confirm(t("control.resetConfirm"))
              )
                void timer.send("/commands", { command: "reset" });
            }}
          >
            ↺ {t("actions.reset")}
          </button>
        </fieldset>
      </section>
      <section className="touch-actions" aria-label={t("touch.stageControls")}>
        <fieldset disabled={disabled} className="touch-blackout">
          <button
            className={state.blackout ? "current" : ""}
            onClick={() =>
              void timer.send("/blackout", { enabled: true }, "PUT")
            }
          >
            ● {t("actions.blackout")}
          </button>
          <button
            className={!state.blackout ? "primary" : ""}
            onClick={() =>
              void timer.send("/blackout", { enabled: false }, "PUT")
            }
          >
            ○ {t("actions.reveal")}
          </button>
        </fieldset>
        <fieldset disabled={disabled} className="touch-message-actions">
          <button
            onClick={() =>
              void timer.send("/message", { visible: false }, "PUT")
            }
          >
            {t("actions.hide")}
          </button>
          <button
            disabled={disabled || !state.message.text}
            onClick={() =>
              void timer.send("/message", { visible: true }, "PUT")
            }
          >
            {t("actions.show")}
          </button>
        </fieldset>
      </section>
      {presets.some(Boolean) && (
        <section className="touch-presets" aria-label={t("settings.presets")}>
          <h2>{t("settings.presets")}</h2>
          <div>
            {presets.map(
              (text, index) =>
                text && (
                  <button
                    key={index}
                    disabled={disabled}
                    onClick={() => void timer.send("/message", { text }, "PUT")}
                  >
                    <kbd>{index + 1}</kbd> {text}
                  </button>
                ),
            )}
          </div>
        </section>
      )}
      <footer className="touch-footer">
        <a href="/">{t("touch.standardControl")}</a>
        <a href="/display" target="_blank" rel="noreferrer">
          {t("touch.display")}
        </a>
      </footer>
    </main>
  );
}
