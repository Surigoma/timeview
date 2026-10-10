import { DisplayConnectionRow } from "./DisplayConnectionRow";
import { t } from "../i18n";
import type { TimerConnection } from "../timer/useTimer";

export function DisplayConnections({ timer }: { timer: TimerConnection }) {
  const reports = timer.displays;
  const known =
    timer.connected &&
    reports !== null &&
    reports.instanceId === timer.state?.instanceId;
  return (
    <details className="display-connections">
      <summary>
        {known
          ? t("displays.count", {
              count: reports.displays.filter((display) => display.online)
                .length,
            })
          : t("displays.unknown")}
        {known && reports.displays.some((display) => !display.online) && (
          <span className="warning">
            {" "}
            ·{" "}
            {t("displays.noResponse", {
              count: reports.displays.filter((display) => !display.online)
                .length,
            })}
          </span>
        )}
      </summary>
      <p className="hint">{t("displays.hint")}</p>
      {known &&
        (reports.displays.length ? (
          <ul>
            {reports.displays.map((display) => (
              <DisplayConnectionRow
                key={display.id}
                display={display}
                timer={timer}
              />
            ))}
          </ul>
        ) : (
          <p className="hint">{t("displays.empty")}</p>
        ))}
    </details>
  );
}
