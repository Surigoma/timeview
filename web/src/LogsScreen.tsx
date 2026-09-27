import { useCallback, useEffect, useState } from "react";
import { t } from "./i18n";

type AuditEntry = {
  time: string;
  type: "process" | "operation";
  action: string;
  result: "success" | "failure";
  client?: string;
  statusCode?: number;
  errorCode?: string;
};

export function LogsScreen({ language }: { language: "ja" | "en" }) {
  const [entries, setEntries] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const response = await fetch("/api/v1/logs", {
        signal: AbortSignal.timeout(4000),
      });
      if (!response.ok) throw new Error();
      const data = (await response.json()) as { entries: AuditEntry[] };
      setEntries(data.entries);
    } catch {
      setError("操作ログを取得できませんでした");
    } finally {
      setLoading(false);
    }
  }, []);
  useEffect(() => void load(), [load]);
  return (
    <section className="panel logs-panel">
      <div className="panel-title">
        <h2>{t("logs.title")}</h2>
        <button
          className="small"
          disabled={loading}
          onClick={() => void load()}
        >
          {t("logs.refresh")}
        </button>
      </div>
      <p className="hint">{t("logs.hint")}</p>
      {error && <p className="warning">{error}</p>}
      {!error && !loading && entries.length === 0 && (
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
