import { useState } from "react";
import { Stage } from "./components/Stage";
import { t } from "./i18n";
import { useRemoteControls } from "./input/useRemoteControls";
import { ControlScreen } from "./screens/control/ControlScreen";
import { LogsScreen } from "./screens/LogsScreen";
import { SettingsScreen } from "./screens/settings/SettingsScreen";
import { TouchScreen } from "./screens/TouchScreen";
import { useTimer } from "./timer/useTimer";

export default function App() {
  const timer = useTimer();
  const display = location.pathname === "/display";
  const touch = location.pathname === "/touch";
  const [tab, setTab] = useState(
    location.pathname === "/settings"
      ? "settings"
      : location.pathname === "/logs"
        ? "logs"
        : "control",
  );
  const remote = useRemoteControls(timer, display);
  const language = timer.state?.language ?? "ja";

  if (display) {
    return timer.state ? (
      <Stage
        state={timer.state}
        remaining={timer.remaining}
        connected={timer.connected}
      />
    ) : (
      <div className="stage blackout" />
    );
  }

  if (touch) return <TouchScreen timer={timer} presets={remote.presets} />;

  return (
    <div className="app-shell">
      <header>
        <a className="brand" href="/">
          <span className="brand-icon">◷</span> TimeView{" "}
          <span className="brand-tag">SCREENING TIMER</span>
        </a>
        <div className="header-right">
          <span className={`connection ${timer.connected ? "online" : ""}`}>
            <i />
            {timer.connected ? t("common.connected") : t("common.connecting")}
          </span>
          <a className="button small" href="/touch">
            {t("app.touch")}
          </a>
          <a
            className="button small"
            href="/display"
            target="_blank"
            rel="noreferrer"
          >
            {t("app.openDisplay")}
          </a>
        </div>
      </header>
      <nav aria-label={t("app.mainNav")}>
        <button
          className={tab === "control" ? "selected" : ""}
          onClick={() => {
            setTab("control");
            remote.setKeypad(false);
            remote.setRecord(null);
            remote.setMidiRecord(null);
          }}
        >
          {t("app.control")}
        </button>
        <button
          className={tab === "settings" ? "selected" : ""}
          onClick={() => {
            setTab("settings");
            remote.setKeypad(false);
            remote.setMidiRecord(null);
          }}
        >
          {t("app.settings")}
        </button>
        <button
          className={tab === "logs" ? "selected" : ""}
          onClick={() => {
            setTab("logs");
            remote.setKeypad(false);
            remote.setRecord(null);
            remote.setMidiRecord(null);
          }}
        >
          {t("app.logs")}
        </button>
        <span>LOCAL / JSON CONFIG</span>
      </nav>
      <main>
        <div className="page-heading">
          <div>
            <p className="eyebrow">
              {tab === "control" ? "CONTROL ROOM" : "PREFERENCES"}
            </p>
            <h1>
              {tab === "control"
                ? t("app.controlHeading")
                : tab === "settings"
                  ? t("app.settingsHeading")
                  : t("app.logsHeading")}
            </h1>
          </div>
          <p>{t("app.environment")}</p>
        </div>
        {timer.notice && (
          <div role="status" className="notice">
            {timer.notice}
          </div>
        )}
        {timer.error && (
          <div role="alert" className="error">
            {timer.error}
            <button onClick={() => timer.setError("")}>
              {t("common.close")}
            </button>
          </div>
        )}
        {!timer.connected && (
          <div className="notice">{t("app.reconnecting")}</div>
        )}
        {timer.state && tab === "control" && (
          <ControlScreen
            timer={timer}
            presets={remote.presets}
            bindings={remote.bindings}
            keypad={remote.keypad}
            setKeypad={remote.setKeypad}
            lastKey={remote.lastKey}
            midiEnabled={remote.midiEnabled}
            midiSupported={"requestMIDIAccess" in navigator}
            toggleMidi={() => {
              if (remote.midiEnabled) remote.setMidiEnabled(false);
              else void remote.connectMidi();
            }}
            lastMidi={remote.lastMidi}
          />
        )}
        {timer.state && tab === "settings" && (
          <SettingsScreen
            timer={timer}
            presets={remote.presets}
            bindings={remote.bindings}
            record={remote.record}
            setRecord={(action) => {
              if (action) remote.setKeypad(false);
              remote.setRecord(action);
            }}
            setBindings={remote.setBindings}
            midiRecord={remote.midiRecord}
            setMidiRecord={(action) => {
              remote.setRecord(null);
              remote.setMidiRecord(action);
            }}
            connectMidi={remote.connectMidi}
          />
        )}
        {tab === "logs" && <LogsScreen language={language} />}
      </main>
      <footer>
        TIMEVIEW <span>{t("app.footer")}</span>
      </footer>
    </div>
  );
}
