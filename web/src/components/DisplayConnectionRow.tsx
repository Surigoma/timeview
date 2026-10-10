import { useRef, useState } from "react";
import { t } from "../i18n";
import type { DisplayReport } from "../timer/events";
import type { TimerConnection } from "../timer/useTimer";

export function DisplayConnectionRow({
  display,
  timer,
}: {
  display: DisplayReport;
  timer: TimerConnection;
}) {
  const [draft, setDraft] = useState<string | null>(null);
  const editButton = useRef<HTMLButtonElement>(null);
  const finishEditing = () => {
    setDraft(null);
    requestAnimationFrame(() => editButton.current?.focus());
  };
  const label = display.name
    ? `${display.name} (${display.id.slice(0, 8)})`
    : t("displays.name", { id: display.id.slice(0, 8) });
  const disabled = !timer.connected || timer.busy;
  return (
    <li>
      <div className="display-row-header">
        <div className="display-row-identity">
          <strong>{label}</strong>
          {draft === null && (
            <button
              ref={editButton}
              className="display-icon"
              disabled={disabled}
              title={t("displays.editName")}
              aria-label={t("displays.editNameLabel", { name: label })}
              onClick={() => setDraft(display.name ?? "")}
            >
              <svg aria-hidden="true" viewBox="0 0 24 24">
                <path d="m15 5 4 4M4 20l4-1L20 7a2.8 2.8 0 0 0-4-4L4 15z" />
              </svg>
            </button>
          )}
          <span className="display-status">
            {t(display.online ? "displays.online" : "displays.offline")}
          </span>
        </div>
        {draft === null && !display.online && (
          <button
            className="display-icon display-remove"
            disabled={disabled}
            title={t("displays.remove")}
            aria-label={t("displays.removeLabel", { name: label })}
            onClick={() => {
              if (confirm(t("displays.removeConfirm", { name: label })))
                void timer.send(`/displays/${display.id}`, undefined, "DELETE");
            }}
          >
            <svg aria-hidden="true" viewBox="0 0 24 24">
              <path d="M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7M14 10v7" />
            </svg>
          </button>
        )}
      </div>
      <div className="hint">
        {t("displays.lastSeen")}{" "}
        <time dateTime={new Date(display.lastSeenMs).toISOString()}>
          {new Date(display.lastSeenMs).toLocaleTimeString(
            timer.state?.language,
            { hour12: false },
          )}
        </time>
      </div>
      {draft !== null ? (
        <form
          className="display-name-form"
          onSubmit={(event) => {
            event.preventDefault();
            void timer
              .send(`/displays/${display.id}/name`, { name: draft }, "PUT")
              .then((ok) => {
                if (ok) finishEditing();
              });
          }}
        >
          <label>
            {t("displays.nameLabel", { id: display.id.slice(0, 8) })}
            <input
              autoFocus
              value={draft}
              disabled={disabled}
              onChange={(event) => setDraft(event.target.value)}
            />
          </label>
          <p className="hint">{t("displays.nameHint")}</p>
          <div className="display-row-actions">
            <button
              className="small"
              disabled={disabled || Array.from(draft).length > 40}
              type="submit"
            >
              {t("displays.saveName")}
            </button>
            <button
              className="small"
              disabled={timer.busy}
              type="button"
              onClick={finishEditing}
            >
              {t("displays.cancelName")}
            </button>
          </div>
        </form>
      ) : null}
    </li>
  );
}
