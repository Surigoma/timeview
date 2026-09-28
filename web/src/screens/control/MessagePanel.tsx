import { useState } from "react";
import { t } from "../../i18n";
import type { TimerConnection } from "../../timer/useTimer";

export function MessagePanel({
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
