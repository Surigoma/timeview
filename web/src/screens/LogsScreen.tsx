import { t } from "../i18n";

import type { TimerConnection } from "../timer/useTimer";

export function LogsScreen({ timer }: { timer: TimerConnection }) {
  const snapshot = timer.logs;
  const entries = snapshot?.entries ?? [];
  const language = timer.state?.language ?? "ja";
  const ready = timer.connected && snapshot !== null;
  const error = snapshot?.error;
  return (
    <section className="panel logs-panel">
      <div className="panel-title">
        <h2>{t("logs.title")}</h2>
        <span className="badge">
          {timer.connected ? t("logs.live") : t("common.connecting")}
        </span>
      </div>
      <p className="hint">{t("logs.hint")}</p>
      {!ready && (
        <p role="status" className="hint">
          {t("logs.waiting")}
        </p>
      )}
      {error && (
        <p role="alert" className="warning">
          {error}
        </p>
      )}
      {ready && !error && entries.length === 0 && (
        <p className="hint">{t("logs.empty")}</p>
      )}
      {entries.length > 0 && (
        <div className="logs-table-wrap">
          <table className="logs-table">
            <thead>
              <tr>
                <th>{t("logs.time")}</th>
                <th>{t("logs.type")}</th>
                <th>{t("logs.action")}</th>
                <th>{t("logs.result")}</th>
                <th>{t("logs.client")}</th>
              </tr>
            </thead>
            <tbody>
              {entries.map((entry, index) => (
                <tr key={`${entry.time}-${index}`}>
                  <td>{new Date(entry.time).toLocaleString(language)}</td>
                  <td>{t(`logs.${entry.type}`)}</td>
                  <td>
                    <code>{entry.action}</code>
                  </td>
                  <td className={entry.result}>
                    {t(`logs.${entry.result}`)}
                    {entry.errorCode ? ` (${entry.errorCode})` : ""}
                  </td>
                  <td>{entry.client ?? "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </section>
  );
}
