import { useState } from "react";
import { t } from "./i18n";
import { Stage } from "./Stage";
import { bindingLabel, formatTime, keyLabel, phase } from "./timer";
import type { Binding } from "./timer";
import type { TimerConnection } from "./useTimer";

export function ControlScreen({
  timer,
  presets,
  bindings,
  keypad,
  setKeypad,
  lastKey,
  midiEnabled,
  midiSupported,
  toggleMidi,
  lastMidi,
}: {
  timer: TimerConnection;
  presets: string[];
  bindings: Binding[];
  keypad: boolean;
  setKeypad: (enabled: boolean) => void;
  lastKey: string;
  midiEnabled: boolean;
  midiSupported: boolean;
  toggleMidi: () => void;
  lastMidi: string;
}) {
  const [overflow, setOverflow] = useState(false);
  const state = timer.state!;
  return (
    <div className="control-grid">
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
        <fieldset
          disabled={!timer.connected || timer.busy}
          className="transport"
        >
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
              if (
                state.state !== "running" ||
                confirm(t("control.resetConfirm"))
              )
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
              {state.blackout
                ? t("control.blackoutOn")
                : t("control.displayOn")}
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
      <section className="panel preview-panel">
        <div className="panel-title">
          <h2>{t("control.preview")}</h2>
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
            ? t("control.previewBlackout")
            : t("control.previewLive")}
        </p>
        {overflow && <p className="warning">{t("control.overflow")}</p>}
      </section>
      <MessagePanel timer={timer} presets={presets} />
      <section className="panel">
        <div className="panel-title">
          <h2>Keypad</h2>
          <span className={`badge ${keypad ? "accent" : ""}`}>
            {keypad ? t("common.enabled") : t("common.off")}
          </span>
        </div>
        <p className="hint">{t("control.keypadHint")}</p>
        <button
          disabled={!timer.connected}
          className={keypad ? "primary" : ""}
          onClick={() => setKeypad(!keypad)}
        >
          {keypad ? t("control.disableKeypad") : t("control.enableKeypad")}
        </button>
        <p aria-live="polite" className="last-key">
          {lastKey}
        </p>
        <div className="midi-control">
          <div className="panel-title">
            <strong>MIDI</strong>
            <span className={`badge ${midiEnabled ? "accent" : ""}`}>
              {midiEnabled ? t("common.enabled") : t("common.off")}
            </span>
          </div>
          <p className="hint">{t("control.midiHint")}</p>
          <button
            disabled={!timer.connected || !midiSupported}
            className={midiEnabled ? "primary" : ""}
            onClick={toggleMidi}
          >
            {midiEnabled ? t("control.disableMidi") : t("control.enableMidi")}
          </button>
          {!midiSupported && (
            <p className="warning">{t("control.midiUnsupported")}</p>
          )}
          <p aria-live="polite" className="last-key">
            {lastMidi}
          </p>
        </div>
        <div className="key-summary">
          {bindings.slice(0, 4).map((binding) => (
            <div key={binding.action}>
              <span>{bindingLabel(binding.action)}</span>
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
        <h2>{t("control.message")}</h2>
        <span className="badge">
          {state.message.visible
            ? t("control.messageVisible")
            : t("control.hidden")}
        </span>
      </div>
      <label htmlFor="message">{t("control.messageLabel")}</label>
      <textarea
        id="message"
        value={draft}
        onChange={(event) => setDraft(event.target.value)}
        rows={3}
        placeholder={t("control.messagePlaceholder")}
      />
      <div className="message-toolbar">
        <span className={count > 500 ? "warning" : "hint"}>
          {t("control.draftCount", { count })}
        </span>
        <button
          className="primary"
          disabled={!timer.connected || timer.busy || count > 500}
          onClick={() => void timer.send("/message", { text: draft }, "PUT")}
        >
          {t("control.send")}
        </button>
      </div>
      <div className="sent-message">
        <span>{t("control.sent")}</span>
        <p>{state.message.text || t("control.notSent")}</p>
      </div>
      <fieldset disabled={!timer.connected || timer.busy}>
        <button
          onClick={() => void timer.send("/message", { visible: false }, "PUT")}
        >
          {t("control.hidden")}
        </button>
        <button
          disabled={!state.message.text}
          onClick={() => void timer.send("/message", { visible: true }, "PUT")}
        >
          {t("control.showAgain")}
        </button>
        <button
          onClick={() => void timer.send("/message", undefined, "DELETE")}
        >
          {t("actions.clear")}
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
