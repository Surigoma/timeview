import { t } from "../i18n";
import { formatEndTime } from "../timer/timer";
import type { TimerConnection } from "../timer/useTimer";

export function EndTime({
  timer,
  className = "hint",
}: {
  timer: TimerConnection;
  className?: string;
}) {
  const state = timer.state;
  if (!state) return null;
  return (
    <p className={className}>
      {t(
        state.state === "running"
          ? "control.endTime"
          : state.state === "paused"
            ? "control.resumeEndTime"
            : "control.startEndTime",
      )}{" "}
      {timer.connected ? formatEndTime(state) : "—"}
    </p>
  );
}
