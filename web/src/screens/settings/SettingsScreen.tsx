import type { Binding } from "../../input/bindings";
import type { TimerConnection } from "../../timer/useTimer";
import { BindingSettings } from "./BindingSettings";
import { IntegrationPanel } from "./IntegrationPanel";
import { PresetEditor } from "./PresetEditor";
import { TimerSettings } from "./TimerSettings";

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
  return (
    <div className="settings-grid">
      <TimerSettings timer={timer} />
      <PresetEditor timer={timer} presets={presets} />
      <BindingSettings
        timer={timer}
        bindings={bindings}
        record={record}
        setRecord={setRecord}
        setBindings={setBindings}
        midiRecord={midiRecord}
        setMidiRecord={setMidiRecord}
        connectMidi={connectMidi}
      />
      <IntegrationPanel timer={timer} />
    </div>
  );
}
