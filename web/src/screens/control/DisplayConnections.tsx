import { useEffect, useState } from "react";
import { t } from "../../i18n";
import type { TimerConnection } from "../../timer/useTimer";

type Display = {
  id: string;
  lastSeenMs: number;
  version: number;
  online: boolean;
};
export function DisplayConnections({ timer }: { timer: TimerConnection }) {
  const [reports, setReports] = useState<{
    instanceId: string;
    displays: Display[];
  } | null>(null);
  const [available, setAvailable] = useState(false);
  const instance = timer.state?.instanceId;
  useEffect(() => {
    let stopped = false;
    let pending = false;
    const controller = new AbortController();
    const load = async () => {
      if (pending) return;
      pending = true;
      try {
        const response = await fetch("/api/v1/timer/displays", {
          signal: AbortSignal.any([
            controller.signal,
            AbortSignal.timeout(4000),
          ]),
        });
        if (!response.ok) throw new Error();
        const next = (await response.json()) as {
          instanceId: string;
          displays: Display[];
        };
        if (!stopped) {
          setReports(next);
          setAvailable(true);
        }
      } catch {
        if (!stopped) setAvailable(false);
      } finally {
        pending = false;
      }
    };
    void load();
    const interval = setInterval(() => void load(), 3000);
    return () => {
      stopped = true;
      controller.abort();
      clearInterval(interval);
    };
  }, [instance]);
  const known =
    timer.connected &&
    available &&
    reports !== null &&
    reports.instanceId === instance;
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
