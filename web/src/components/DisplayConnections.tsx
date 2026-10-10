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
              <li key={display.id}>
                <strong>
                  {t("displays.name", { id: display.id.slice(0, 8) })}
                </strong>
                {" · "}
                {t(display.online ? "displays.online" : "displays.offline")}
                <div className="hint">
                  {t("displays.lastSeen")}{" "}
                  <time dateTime={new Date(display.lastSeenMs).toISOString()}>
                    {new Date(display.lastSeenMs).toLocaleTimeString(
                      timer.state?.language,
                      { hour12: false },
                    )}
                  </time>
                </div>
              </li>
            ))}
          </ul>
        ) : (
          <p className="hint">{t("displays.empty")}</p>
        ))}
    </details>
  );
}
