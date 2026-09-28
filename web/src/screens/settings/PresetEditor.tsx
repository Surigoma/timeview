import { useState } from "react";
import { t } from "../../i18n";
import type { TimerConnection } from "../../timer/useTimer";

export function PresetEditor({
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
