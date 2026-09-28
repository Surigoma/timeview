import { useState } from "react";
import type { FormEvent } from "react";
import { t } from "../i18n";
import {
  bindingLabel,
  defaultBindings,
  keyLabel,
  midiLabel,
  storeBindings,
} from "../timer/timer";
import type { Binding, TimerState } from "../timer/timer";
import type { TimerConnection } from "../timer/useTimer";

export function SettingsScreen({
  timer,
  presets,
  bindings,
  record,
  setRecord,
  setBindings,
  midiRecord,
  setMidiRecord,
  connectMidi,
}: {
  timer: TimerConnection;
  presets: string[];
  bindings: Binding[];
  record: string | null;
  setRecord: (action: string | null) => void;
  setBindings: (bindings: Binding[]) => void;
  midiRecord: string | null;
  setMidiRecord: (action: string | null) => void;
  connectMidi: () => Promise<boolean>;
}) {
  const [toast, setToast] = useState("");
  const state = timer.state!;
  const copy = async (text: string) => {
    // LAN HTTP may not provide navigator.clipboard; the manual field remains usable.
    try {
      await navigator.clipboard.writeText(text);
      setToast(t("settings.copied"));
    } catch {
      setToast(t("settings.copyFallback"));
    }
    setTimeout(() => setToast(""), 3000);
  };
  return (
    <div className="settings-grid">
      <SettingsPanel timer={timer} />
      <PresetEditor timer={timer} presets={presets} />
      <section className="panel">
        <div className="panel-title">
          <h2>{t("settings.bindings")}</h2>
          <button
            className="small"
            onClick={() => {
              const defaults = defaultBindings.map((binding) => ({
                ...binding,
              }));
              void timer
                .send("/bindings", { bindings: storeBindings(defaults) }, "PUT")
                .then((ok) => {
                  if (ok) setBindings(defaults);
                });
              setRecord(null);
              setMidiRecord(null);
            }}
          >
            {t("settings.defaults")}
          </button>
        </div>
        <p className="hint">{t("settings.bindingHint")}</p>
        <div className="bindings">
          {bindings.map((binding) => (
            <div key={binding.action}>
              <span>{bindingLabel(binding.action)}</span>
              <div className="binding-actions">
                <button
                  className="small"
                  onClick={() => {
                    setMidiRecord(null);
                    setRecord(binding.action);
                  }}
                >
                  {record === binding.action
                    ? t("settings.enterKey")
                    : keyLabel(binding)}
                </button>
                <button
                  className="small"
                  onClick={() => {
                    void connectMidi().then((ok) => {
                      if (ok) setMidiRecord(binding.action);
                    });
                  }}
                >
                  {midiRecord === binding.action
                    ? t("settings.enterMidi")
                    : midiLabel(binding.midi)}
                </button>
                {binding.midi && (
                  <button
                    className="small"
                    aria-label={t("settings.clearMidi")}
                    onClick={() => {
                      const next = bindings.map((item) =>
                        item.action === binding.action
                          ? { ...item, midi: undefined }
                          : item,
                      );
                      void timer
                        .send(
                          "/bindings",
                          { bindings: storeBindings(next) },
                          "PUT",
                        )
                        .then((ok) => {
                          if (ok) setBindings(next);
                        });
                    }}
                  >
                    ×
                  </button>
                )}
              </div>
            </div>
          ))}
        </div>
      </section>
      <section className="panel">
        <div className="panel-title">
          <h2>{t("settings.integration")}</h2>
          <span className="badge">
            {state.browserOnly ? t("settings.readOnly") : "HTTP"}
          </span>
        </div>
        <p className="hint">
          {state.browserOnly
            ? t("settings.integrationReadOnly")
            : t("settings.integrationOpen")}
        </p>
        <label>
          {t("settings.displayURL")}
          <input
            readOnly
            value={`${location.origin}/display`}
            onFocus={(event) => event.target.select()}
          />
        </label>
        <button
          className="small"
          onClick={() => void copy(`${location.origin}/display`)}
        >
          {t("settings.copyURL")}
        </button>
        <label>
          {t("settings.timerAPI")}
          <input
            readOnly
            value={`${location.origin}/api/v1/timer`}
            onFocus={(event) => event.target.select()}
          />
        </label>
        <p aria-live="polite">{toast}</p>
        <pre>
          {state.browserOnly
            ? t("settings.readOnlyExample")
            : 'PUT /api/v1/timer/blackout\nContent-Type: application/json\n\n{"enabled": true}'}
        </pre>
        <p className="hint">{t("settings.persistence")}</p>
      </section>
    </div>
  );
}

function SettingsPanel({ timer }: { timer: TimerConnection }) {
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

function PresetEditor({
  timer,
  presets,
}: {
  timer: TimerConnection;
  presets: string[];
}) {
  const [draft, setDraft] = useState<string[] | null>(null);
  const values =
    draft ?? Array.from({ length: 9 }, (_, index) => presets[index] ?? "");
  return (
    <section className="panel">
      <div className="panel-title">
        <h2>{t("settings.presets")}</h2>
        <span className="badge">{t("settings.presetLimit")}</span>
      </div>
      <p className="hint">{t("settings.presetHint")}</p>
      <div className="preset-editor">
        {values.map((text, index) => (
          <label key={index}>
            <kbd>{index + 1}</kbd>
            <input
              value={text}
              placeholder={t("actions.preset", { number: index + 1 })}
              onChange={(event) => {
                const next = [...values];
                next[index] = event.target.value;
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
          values.some((value) => Array.from(value).length > 500)
        }
        onClick={() => {
          void timer.send("/presets", { presets: values }, "PUT").then((ok) => {
            if (ok) setDraft(null);
          });
        }}
      >
        {t("settings.applyPresets")}
      </button>
      <button onClick={() => setDraft(null)}>{t("settings.undo")}</button>
    </section>
  );
}
