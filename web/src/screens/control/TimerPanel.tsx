import { t } from "../../i18n";
import { formatTime, phase } from "../../timer/timer";
import type { TimerConnection } from "../../timer/useTimer";

export function TimerPanel({ timer }: { timer: TimerConnection }) {
  const state = timer.state!;
  return (
    <section className="panel timer-panel">
      <div className="panel-title">
        <h2>{t("control.timer")}</h2>
        <span className="badge">{t(`timer.${state.state}`)}</span>
      </div>
      <div
        className="operator-time"
        style={{ color: phase(state, timer.remaining).color }}
      >
        {formatTime(timer.remaining)}
      </div>
      <div className="timer-meta">
        <span>
          {t("control.duration", {
            time: formatTime(state.durationSeconds * 1000),
          })}
        </span>
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
      <fieldset disabled={!timer.connected || timer.busy} className="transport">
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
          onClick={() => {
            if (state.state !== "running" || confirm(t("control.resetConfirm")))
              void timer.send("/commands", { command: "reset" });
          }}
        >
          ↺ {t("actions.reset")}
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
      </fieldset>
      <div className={`blackout-control ${state.blackout ? "active" : ""}`}>
        <div>
          <strong>
            {state.blackout ? t("control.blackoutOn") : t("control.displayOn")}
          </strong>
          <p>{t("control.blackoutContinues")}</p>
        </div>
        <fieldset disabled={!timer.connected || timer.busy}>
          <button
            className={state.blackout ? "current" : ""}
            onClick={() =>
              void timer.send("/blackout", { enabled: true }, "PUT")
            }
          >
            {t("actions.blackout")}
          </button>
          <button
            onClick={() =>
              void timer.send("/blackout", { enabled: false }, "PUT")
            }
          >
            {t("actions.reveal")}
          </button>
        </fieldset>
      </div>
    </section>
  );
}
