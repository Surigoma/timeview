import { useState } from "react";
import { Stage } from "../../components/Stage";
import { t } from "../../i18n";
import type { TimerConnection } from "../../timer/useTimer";

export function PreviewPanel({ timer }: { timer: TimerConnection }) {
  const [overflow, setOverflow] = useState(false);
  const state = timer.state!;
  return (
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
  );
}
