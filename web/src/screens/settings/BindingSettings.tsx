import { t } from "../../i18n";
import {
  bindingLabel,
  defaultBindings,
  keyLabel,
  midiLabel,
  storeBindings,
} from "../../input/bindings";
import type { Binding } from "../../input/bindings";
import type { TimerConnection } from "../../timer/useTimer";

export function BindingSettings({
  timer,
  bindings,
  record,
  setRecord,
  setBindings,
  midiRecord,
  setMidiRecord,
  connectMidi,
}: {
  timer: TimerConnection;
  bindings: Binding[];
  record: string | null;
  setRecord: (action: string | null) => void;
  setBindings: (bindings: Binding[]) => void;
  midiRecord: string | null;
  setMidiRecord: (action: string | null) => void;
  connectMidi: () => Promise<boolean>;
}) {
  return (
    <section className="panel">
      <div className="panel-title">
        <h2>{t("settings.bindings")}</h2>
        <button
          className="small"
          onClick={() => {
            const defaults = defaultBindings.map((binding) => ({ ...binding }));
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
  );
}
