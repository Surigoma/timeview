import { useEffect, useRef } from "react";
import type { CSSProperties } from "react";
import {
  elapsedPercent,
  foregroundColor,
  formatTime,
  phase,
  warningGauge,
} from "./timer";
import type { TimerState } from "./timer";

export function Stage({
  state,
  remaining,
  connected,
  preview = false,
  onOverflow,
}: {
  state: TimerState;
  remaining: number;
  connected: boolean;
  preview?: boolean;
  onOverflow?: (value: boolean) => void;
}) {
  const container = useRef<HTMLDivElement>(null);
  const message = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const el = message.current,
      box = container.current;
    if (!el || !box) {
      onOverflow?.(false);
      return;
    }
    const fit = () => {
      const minimum = preview ? 10 : 24;
      let size = Math.min(120, Math.max(minimum, box.clientWidth / 18));
      el.style.fontSize = `${size}px`;
      while (
        size > minimum &&
        (el.scrollHeight > el.clientHeight + 1 ||
          el.scrollWidth > el.clientWidth + 1)
      ) {
        size -= 1;
        el.style.fontSize = `${size}px`;
      }
      onOverflow?.(
        el.scrollHeight > el.clientHeight + 1 ||
          el.scrollWidth > el.clientWidth + 1,
      );
    };
    const observer = new ResizeObserver(fit);
    observer.observe(box);
    fit();
    return () => observer.disconnect();
  }, [
    state.message.text,
    state.message.visible,
    state.displayMode,
    state.blackout,
    preview,
    onOverflow,
  ]);
  const currentPhase = phase(state, remaining);
  const elapsed = elapsedPercent(state.durationSeconds, remaining);
  const gauge = warningGauge(
    state.durationSeconds,
    state.warning1Seconds,
    state.warning2Seconds,
    remaining,
  );
  return (
    <div
      ref={container}
      className={`stage ${preview ? "preview" : ""} ${state.blackout ? "blackout" : ""} ${state.displayMode === "message" ? "message-only" : ""}`}
      style={
        {
          "--stage-background": currentPhase.color,
          "--stage-foreground": foregroundColor(currentPhase.color),
          "--gauge-normal": state.colors.normal,
          "--gauge-warning1": state.colors.warning1,
          "--gauge-warning2": state.colors.warning2,
          "--gauge-warning1-start": `${gauge.warning1Start}%`,
          "--gauge-warning2-start": `${gauge.warning2Start}%`,
        } as CSSProperties
      }
    >
      {!state.blackout && (
        <>
          {state.displayMode !== "message" && (
            <div className="time-block">
              <div
                className={`stage-time ${state.flash && remaining <= 0 ? "pulse" : ""}`}
                style={{
                  fontSize: `${Math.min(26, 145 / formatTime(remaining).length)}cqw`,
                }}
              >
                {formatTime(remaining)}
              </div>
            </div>
          )}
          {state.displayMode !== "timer" && state.message.visible && (
            <div ref={message} className="stage-message">
              {state.message.text}
            </div>
          )}
          {!connected && (
            <span className="stage-offline">接続待ち · 推定表示</span>
          )}
          <div className="gauge-block">
            {gauge.next && (
              <div className="gauge-next">
                <span>{gauge.next.label}</span>
                <strong>{formatTime(gauge.next.remainingMs)}</strong>
              </div>
            )}
            <div
              className="elapsed-gauge"
              role="progressbar"
              aria-label="警告領域付き経過時間"
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={Math.round(elapsed)}
              aria-valuetext={
                gauge.next
                  ? `${gauge.next.label} ${formatTime(gauge.next.remainingMs)}`
                  : "終了時刻を超過"
              }
            >
              <div
                className="gauge-elapsed"
                style={{ width: `${elapsed}%` }}
                aria-hidden="true"
              />
              <div
                className="gauge-position"
                style={{ left: `${elapsed}%` }}
                aria-hidden="true"
              />
            </div>
          </div>
          {!preview && (
            <button
              className="fullscreen"
              onClick={() => {
                void document.documentElement
                  .requestFullscreen()
                  .catch(() => {});
              }}
            >
              全画面表示
            </button>
          )}
        </>
      )}
    </div>
  );
}
