import { t } from "../../i18n";
import { bindingLabel, keyLabel } from "../../input/bindings";
import type { Binding } from "../../input/bindings";
import type { TimerConnection } from "../../timer/useTimer";

export function InputPanel({
  timer,
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
  bindings: Binding[];
  keypad: boolean;
  setKeypad: (enabled: boolean) => void;
  lastKey: string;
  midiEnabled: boolean;
  midiSupported: boolean;
  toggleMidi: () => void;
  lastMidi: string;
}) {
  return (
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
  );
}
