import { useState } from "react";
import type { FormEvent } from "react";
import { t } from "../../i18n";
import type { TimerState } from "../../timer/timer";
import type { TimerConnection } from "../../timer/useTimer";

export function TimerSettings({ timer }: { timer: TimerConnection }) {
  const [editing, setEditing] = useState<TimerState | null>(null);
  const state = editing ?? timer.state!;
  const update = (patch: Partial<TimerState>) =>
    setEditing({ ...state, ...patch });
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const body = {
      ...(state.state === "idle"
        ? { durationSeconds: state.durationSeconds }
        : {}),
      warning1Seconds: state.warning1Seconds,
      warning2Seconds: state.warning2Seconds,
      displayMode: state.displayMode,
      language: state.language,
      logLevel: state.logLevel,
      flash: state.flash,
      browserOnly: state.browserOnly,
      colors: state.colors,
    };
    const ok = await timer.send(
      "",
      body,
      "PATCH",
      `"${state.instanceId}:${state.version}"`,
    );
    if (ok) setEditing(null);
  };
  return (
    <section className="panel">
      <div className="panel-title">
        <h2>{t("settings.timerDisplay")}</h2>
        <span className="badge">
          {editing ? t("common.edit") : t("common.synced")}
        </span>
      </div>
      <form onSubmit={(event) => void submit(event)}>
        <fieldset
          disabled={!timer.connected || timer.busy}
          className="form-fields"
        >
          <div className="duration-fields">
            <span>{t("settings.duration")}</span>
            {(
              [
                t("settings.hour"),
                t("settings.minute"),
                t("settings.second"),
              ] as const
            ).map((label, index) => {
              const parts = [
                Math.floor(state.durationSeconds / 3600),
                Math.floor(state.durationSeconds / 60) % 60,
                state.durationSeconds % 60,
              ];
              return (
                <label key={label}>
                  {label}
                  <input
                    type="number"
                    min={0}
                    max={index === 0 ? 24 : 59}
                    required
                    disabled={timer.state?.state !== "idle"}
                    value={parts[index]}
                    onChange={(event) => {
                      parts[index] = Number(event.target.value);
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
              {t("settings.warning1")}
              <input
                type="number"
                min={0}
                max={state.durationSeconds}
                required
                value={state.warning1Seconds}
                onChange={(event) =>
                  update({ warning1Seconds: Number(event.target.value) })
                }
              />
            </label>
            <label>
              {t("settings.warning2")}
              <input
                type="number"
                min={0}
                max={state.warning1Seconds}
                required
                value={state.warning2Seconds}
                onChange={(event) =>
                  update({ warning2Seconds: Number(event.target.value) })
                }
              />
            </label>
          </div>
          <label>
            {t("settings.displayMode")}
            <select
              value={state.displayMode}
              onChange={(event) =>
                update({
                  displayMode: event.target.value as TimerState["displayMode"],
                })
              }
            >
              <option value="timer_and_message">
                {t("settings.timerMessage")}
              </option>
              <option value="timer">{t("settings.timerOnly")}</option>
              <option value="message">{t("settings.messageOnly")}</option>
            </select>
          </label>
          <label>
            {t("settings.language")}
            <select
              value={state.language}
              onChange={(event) =>
                update({
                  language: event.target.value as TimerState["language"],
                })
              }
            >
              <option value="ja">{t("settings.japanese")}</option>
              <option value="en">{t("settings.english")}</option>
            </select>
          </label>
          <label>
            {t("settings.logLevel")}
            <select
              value={state.logLevel}
              onChange={(event) =>
                update({
                  logLevel: event.target.value as TimerState["logLevel"],
                })
              }
            >
              <option value="debug">DEBUG</option>
              <option value="info">INFO</option>
              <option value="warn">WARN</option>
              <option value="error">ERROR</option>
            </select>
          </label>
          <div className="colors">
            {(["normal", "warning1", "warning2", "overtime"] as const).map(
              (key, index) => (
                <label key={key}>
                  {
                    [
                      t("settings.normal"),
                      t("timer.warning1"),
                      t("timer.warning2"),
                      t("settings.overtime"),
                    ][index]
                  }
                  <input
                    type="color"
                    value={state.colors[key]}
                    onChange={(event) =>
                      update({
                        colors: { ...state.colors, [key]: event.target.value },
                      })
                    }
                  />
                </label>
              ),
            )}
          </div>
          <label className="checkbox">
            <input
              type="checkbox"
              checked={state.flash}
              onChange={(event) => update({ flash: event.target.checked })}
            />
            {t("settings.flash")}
          </label>
          <label className="checkbox">
            <input
              type="checkbox"
              checked={state.browserOnly}
              onChange={(event) =>
                update({ browserOnly: event.target.checked })
              }
            />
            {t("settings.browserOnly")}
          </label>
          <p className="hint">{t("settings.browserOnlyHint")}</p>
          <div>
            <button type="submit" className="primary">
              {t("settings.apply")}
            </button>
            <button type="button" onClick={() => setEditing(null)}>
              {t("settings.reload")}
            </button>
          </div>
        </fieldset>
      </form>
      <p className="hint">{t("settings.conflict")}</p>
    </section>
  );
}
