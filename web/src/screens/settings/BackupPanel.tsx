import { useState } from "react";
import { t } from "../../i18n";
import type { TimerConnection } from "../../timer/useTimer";

type Backup = {
  version: number;
  timer: Record<string, unknown>;
  presets: string[];
  keypadBindings: unknown[];
};

function comparable(value: unknown): string {
  return JSON.stringify(value, (_key, item) =>
    item && typeof item === "object" && !Array.isArray(item)
      ? Object.fromEntries(
          Object.keys(item)
            .sort()
            .map((key) => [key, item[key]]),
        )
      : item,
  );
}

export function BackupPanel({ timer }: { timer: TimerConnection }) {
  const [preview, setPreview] = useState<{
    before: Backup;
    after: Backup;
    etag: string;
  } | null>(null);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");
  const state = timer.state!;
  const canImport =
    timer.connected && !timer.busy && !loading && state.state === "idle";
  async function currentConfig() {
    const response = await fetch("/api/v1/timer/config", {
      signal: AbortSignal.timeout(5000),
    });
    if (!response.ok) throw new Error();
    return {
      config: (await response.json()) as Backup,
      etag: response.headers.get("ETag")!,
    };
  }
  async function download() {
    setLoading(true);
    setMessage("");
    try {
      const { config } = await currentConfig();
      const url = URL.createObjectURL(
        new Blob([JSON.stringify(config, null, 2) + "\n"], {
          type: "application/json",
        }),
      );
      const link = document.createElement("a");
      link.href = url;
      link.download = "timeview-settings.json";
      link.click();
      setTimeout(() => URL.revokeObjectURL(url), 1000);
      setMessage(t("backup.exported"));
    } catch {
      timer.setError("設定を書き出せませんでした。接続を確認してください");
    } finally {
      setLoading(false);
    }
  }
  async function read(file: File) {
    setPreview(null);
    setMessage("");
    setLoading(true);
    try {
      if (file.size > 64 * 1024) {
        timer.setError("設定ファイルは64KiB以内にしてください");
        return;
      }
      const after = JSON.parse(await file.text()) as Backup;
      const { config: before, etag } = await currentConfig();
      if (await timer.send("/config/validate", after, "POST", etag)) {
        after.timer.language ||= "ja";
        after.timer.logLevel ||= "info";
        after.presets ||= [];
        setPreview({ before, after, etag });
      }
    } catch {
      timer.setError(
        "設定ファイルを読み込めませんでした。JSONと接続を確認してください",
      );
    } finally {
      setLoading(false);
    }
  }
  const changes = preview
    ? [
        ...Object.keys(preview.before.timer)
          .filter(
            (key) =>
              comparable(preview.before.timer[key]) !==
              comparable(preview.after.timer[key]),
          )
          .map((key) => ({
            label: t(`backup.fields.${key}`),
            before: preview.before.timer[key],
            after: preview.after.timer[key],
          })),
        ...(["presets", "keypadBindings"] as const)
          .filter(
            (key) =>
              comparable(preview.before[key]) !==
              comparable(preview.after[key]),
          )
          .map((key) => ({
            label: t(`backup.fields.${key}`),
            before: preview.before[key],
            after: preview.after[key],
          })),
      ]
    : [];
  return (
    <section className="panel backup-panel">
      <div className="panel-title">
        <h2>{t("backup.title")}</h2>
      </div>
      <p className="hint">{t("backup.hint")}</p>
      <button
        disabled={!timer.connected || loading}
        onClick={() => void download()}
      >
        {t("backup.export")}
      </button>
      <label>
        {t("backup.import")}
        <input
          type="file"
          accept=".json,application/json"
          disabled={!canImport}
          onChange={(event) => {
            const file = event.target.files?.[0];
            event.target.value = "";
            if (file) void read(file);
          }}
        />
      </label>
      <p className="hint">{t("backup.idleOnly")}</p>
      <p role="status">{loading ? t("backup.loading") : message}</p>
      {preview && (
        <div>
          <h3>{t("backup.review")}</h3>
          {changes.length ? (
            <ul>
              {changes.map((change) => (
                <li key={change.label}>
                  <details>
                    <summary>{change.label}</summary>
                    <p>{t("backup.before")}</p>
                    <pre>{JSON.stringify(change.before, null, 2)}</pre>
                    <p>{t("backup.after")}</p>
                    <pre>{JSON.stringify(change.after, null, 2)}</pre>
                  </details>
                </li>
              ))}
            </ul>
          ) : (
            <p>{t("backup.noChanges")}</p>
          )}
          <button
            className="primary"
            disabled={!canImport || !changes.length}
            onClick={() => {
              void timer
                .send("/config", preview.after, "PATCH", preview.etag)
                .then((ok) => {
                  if (ok) {
                    setPreview(null);
                    setMessage(t("backup.imported"));
                  }
                });
            }}
          >
            {t("backup.apply")}
          </button>
          <button onClick={() => setPreview(null)}>{t("backup.cancel")}</button>
        </div>
      )}
    </section>
  );
}
