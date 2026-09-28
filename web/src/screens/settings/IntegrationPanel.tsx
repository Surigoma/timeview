import { useState } from "react";
import { t } from "../../i18n";
import type { TimerConnection } from "../../timer/useTimer";

export function IntegrationPanel({ timer }: { timer: TimerConnection }) {
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
  );
}
