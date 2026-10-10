import type { Binding } from "../../input/bindings";
import { useState } from "react";
import type { TimerState } from "../../timer/timer";
import { t } from "../../i18n";
import type { TimerConnection } from "../../timer/useTimer";
import { BindingSettings } from "./BindingSettings";
import { IntegrationPanel } from "./IntegrationPanel";
import { PresetEditor } from "./PresetEditor";
import { BackupPanel } from "./BackupPanel";
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
  const [editing, setEditing] = useState<TimerState | null>(null);
  const [draft, setDraft] = useState<string[] | null>(null);
  return (
    <div className="settings-sections">
      <details className="settings-section" open>
        <summary>
          {t("settings.timerDisplay")}{" "}
          <span className="badge">
            {editing ? t("common.edit") : t("common.synced")}
          </span>
        </summary>
        <TimerSettings
          timer={timer}
          editing={editing}
          setEditing={setEditing}
        />
      </details>
      <details className="settings-section">
        <summary>
          {t("settings.presets")}{" "}
          <span className="badge">
            {draft ? t("common.edit") : t("settings.presetLimit")}
          </span>
        </summary>
        <PresetEditor
          timer={timer}
          presets={presets}
          draft={draft}
          setDraft={setDraft}
        />
      </details>
      <details className="settings-section">
        <summary>{t("settings.inputIntegration")}</summary>
        <div className="settings-grid">
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
      </details>
      <details className="settings-section">
        <summary>{t("backup.title")}</summary>
        <BackupPanel timer={timer} />
      </details>
    </div>
  );
}
