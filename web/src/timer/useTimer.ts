import { useCallback, useEffect, useRef, useState } from "react";
import { operationError, setLanguage, t } from "../i18n.ts";
import type { TimerState } from "./timer.ts";

import { belongsToInstance, eventURL } from "./events.ts";
import type { DisplaySnapshot, LogSnapshot, StreamView } from "./events.ts";

export function useTimer(view: StreamView = "control") {
  const [displays, setDisplays] = useState<DisplaySnapshot | null>(null);
  const [logs, setLogs] = useState<LogSnapshot | null>(null);
  const [state, setState] = useState<TimerState | null>(null);
  const current = useRef<TimerState | null>(null);
  const [remaining, setRemaining] = useState(0);
  const [connected, setConnected] = useState(false);
  const [notice, setNotice] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const pending = useRef(false);
  const received = useRef(0);
  const baseline = useRef({ remaining: 0, at: 0 });
  const latency = useRef(0);
  const generation = useRef(0);

  const accept = useCallback((next: TimerState) => {
    const prev = current.current;
    if (
      prev &&
      prev.instanceId === next.instanceId &&
      (prev.version > next.version ||
        (prev.version === next.version &&
          prev.serverTimeMs > next.serverTimeMs))
    )
      return;
    setLanguage(next.language);
    if (prev && prev.instanceId !== next.instanceId) {
      generation.current++;
      setDisplays(null);
      setLogs(null);
      setError("");
      setNotice(t("notices.restarted"));
    }
    current.current = next;
    baseline.current = {
      remaining:
        next.remainingMs - (next.state === "running" ? latency.current : 0),
      at: performance.now(),
    };
    setRemaining(baseline.current.remaining);
    setState(next);
  }, []);

  const refresh = useCallback(async () => {
    const gen = generation.current;
    const t = performance.now();
    const r = await fetch("/api/v1/timer", {
      signal: AbortSignal.timeout(4000),
    });
    if (!r.ok) throw new Error("状態を取得できませんでした");
    const next: TimerState = await r.json();
    if (gen !== generation.current) return;
    latency.current = (performance.now() - t) / 2;
    accept(next);
  }, [accept]);

  useEffect(() => {
    let closed = false,
      source: EventSource | undefined,
      retry = 1000;
    let reconnect: ReturnType<typeof setTimeout>;
    const disconnect = () => {
      source?.close();
      source = undefined;
      setConnected(false);
      setDisplays(null);
      setLogs(null);
      if (!closed) {
        clearTimeout(reconnect);
        reconnect = setTimeout(open, retry);
        retry = Math.min(10000, retry * 2);
      }
    };
    const open = () => {
      if (closed) return;
      received.current = performance.now();
      source = new EventSource(eventURL(view));
      source.addEventListener("displays", (e) => {
        const snapshot = JSON.parse(
          (e as MessageEvent).data,
        ) as DisplaySnapshot;
        if (belongsToInstance(snapshot, current.current)) setDisplays(snapshot);
      });
      source.addEventListener("logs", (e) => {
        const snapshot = JSON.parse((e as MessageEvent).data) as LogSnapshot;
        if (belongsToInstance(snapshot, current.current)) setLogs(snapshot);
      });
      source.addEventListener("state", (e) => {
        const next = JSON.parse((e as MessageEvent).data) as TimerState;
        received.current = performance.now();
        setConnected(true);
        retry = 1000;
        accept(next);
      });
      source.onerror = disconnect;
    };
    open();
    void refresh().catch(() => {});
    const clock = setInterval(() => {
      const s = current.current;
      if (s)
        setRemaining(
          baseline.current.remaining -
            (s.state === "running"
              ? performance.now() - baseline.current.at
              : 0),
        );
      if (source && performance.now() - received.current > 5000) disconnect();
    }, 100);
    return () => {
      closed = true;
      source?.close();
      clearTimeout(reconnect);
      clearInterval(clock);
    };
  }, [accept, refresh, view]);

  const send = useCallback(
    async (path: string, body?: unknown, method = "POST", etag?: string) => {
      if (pending.current || !current.current || !connected) return false;
      pending.current = true;
      setBusy(true);
      setError("");
      const boot = current.current.instanceId,
        gen = generation.current;
      let failure: string | undefined;
      try {
        const headers: Record<string, string> = {
          "Content-Type": "application/json",
          "Idempotency-Key": requestID(),
          "X-Timeview-Instance": boot,
          "X-Timeview-Client": "browser",
        };
        if (etag) headers["If-Match"] = etag;
        const response = await fetch(`/api/v1/timer${path}`, {
          method,
          headers,
          body: body === undefined ? undefined : JSON.stringify(body),
          signal: AbortSignal.timeout(5000),
        });
        if (!response.ok) {
          const data = (await response.json().catch(() => null)) as {
            error?: { message?: unknown };
          } | null;
          if (typeof data?.error?.message === "string") {
            failure = data.error.message;
          }
          throw new Error();
        }
        const next: TimerState = await response.json();
        if (generation.current !== gen || next.instanceId !== boot)
          return false;
        accept(next);
        return true;
      } catch {
        if (generation.current === gen) {
          setError(operationError(failure));
          void refresh().catch(() => {});
        }
        return false;
      } finally {
        pending.current = false;
        setBusy(false);
      }
    },
    [accept, connected, refresh],
  );
  return {
    state,
    remaining,
    connected,
    notice,
    error,
    setError,
    busy,
    send,
    displays,
    logs,
  };
}

export type TimerConnection = ReturnType<typeof useTimer>;

// crypto.randomUUID is unavailable on plain HTTP LAN addresses in some browsers.
export function requestID() {
  const bytes = new Uint8Array(16);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (x) => x.toString(16).padStart(2, "0")).join("");
}
