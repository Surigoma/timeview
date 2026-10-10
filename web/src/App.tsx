import { useEffect, useState } from "react";
import { Stage } from "./components/Stage";
import { t } from "./i18n";
import { useRemoteControls } from "./input/useRemoteControls";
import { ControlScreen } from "./screens/control/ControlScreen";
import { LogsScreen } from "./screens/LogsScreen";
import { SettingsScreen } from "./screens/settings/SettingsScreen";
import { TouchScreen } from "./screens/TouchScreen";
import { useDisplayReport } from "./timer/useDisplayReport";
import { useTimer } from "./timer/useTimer";

function currentPage() {
  return location.pathname === "/settings" || location.pathname === "/logs"
    ? location.pathname
    : "/";
}

export default function App() {
  const display = location.pathname === "/display";
  const touch = location.pathname === "/touch";
  const timer = useTimer(display ? "display" : touch ? "touch" : "control");
  useDisplayReport(timer, display);
  const [tab, setTab] = useState(currentPage);
  const remote = useRemoteControls(timer, display);
  const { setKeypad, setRecord, setMidiRecord } = remote;
  useEffect(() => {
    const restore = () => setTab(currentPage());
    addEventListener("popstate", restore);
    return () => removeEventListener("popstate", restore);
  }, []);
  useEffect(() => {
    setKeypad(false);
    setRecord(null);
    setMidiRecord(null);
  }, [tab, setKeypad, setRecord, setMidiRecord]);
  const navigate = (path: string) => {
    if (location.pathname !== path) history.pushState(null, "", path);
    setTab(path);
  };

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
        {(
          [
            ["/", "app.control"],
            ["/settings", "app.settings"],
            ["/logs", "app.logs"],
          ] as const
        ).map(([path, label]) => (
          <a
            key={path}
            href={path}
            className={tab === path ? "selected" : ""}
            aria-current={tab === path ? "page" : undefined}
            onClick={(event) => {
              if (
                event.button !== 0 ||
                event.metaKey ||
                event.ctrlKey ||
                event.shiftKey ||
                event.altKey
              )
                return;
              event.preventDefault();
              navigate(path);
            }}
          >
            {t(label)}
          </a>
        ))}
        <span>LOCAL / JSON CONFIG</span>
      </nav>
      <main>
        <div className="page-heading">
          <div>
            <p className="eyebrow">
              {tab === "/" ? "CONTROL ROOM" : "PREFERENCES"}
            </p>
            <h1>
              {tab === "/"
                ? t("app.controlHeading")
                : tab === "/settings"
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
        {timer.state && tab === "/" && (
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
        {timer.state && (
          <div hidden={tab !== "/settings"}>
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
          </div>
        )}
        {tab === "/logs" && <LogsScreen timer={timer} />}
      </main>
      <footer>
        TIMEVIEW <span>{t("app.footer")}</span>
      </footer>
    </div>
  );
}
