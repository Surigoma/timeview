import type { Binding } from "../../input/bindings";
import type { TimerConnection } from "../../timer/useTimer";
import { InputPanel } from "./InputPanel";
import { MessagePanel } from "./MessagePanel";
import { PreviewPanel } from "./PreviewPanel";
import { TimerPanel } from "./TimerPanel";

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
  return (
    <div className="control-grid">
      <TimerPanel timer={timer} />
      <PreviewPanel timer={timer} />
      <MessagePanel timer={timer} presets={presets} />
      <InputPanel
        timer={timer}
        bindings={bindings}
        keypad={keypad}
        setKeypad={setKeypad}
        lastKey={lastKey}
        midiEnabled={midiEnabled}
        midiSupported={midiSupported}
        toggleMidi={toggleMidi}
        lastMidi={lastMidi}
      />
    </div>
  );
}
