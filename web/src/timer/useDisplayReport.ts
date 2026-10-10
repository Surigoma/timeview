import { useEffect, useRef } from "react";
import type { TimerConnection } from "./useTimer";
import { requestID } from "./useTimer";

// Each mounted /display page is counted separately. Preview and control never report.
export function useDisplayReport(timer: TimerConnection, enabled: boolean) {
  const current = useRef(timer);
  useEffect(() => {
    current.current = timer;
  }, [timer]);
  useEffect(() => {
    if (!enabled) return;
    const id = requestID();
    let pending = false;
    let stopped = false;
    const controller = new AbortController();
    const report = async () => {
      const { state, connected } = current.current;
      if (stopped || pending || !state || !connected) return;
      pending = true;
      try {
        await fetch(`/api/v1/timer/displays/${id}`, {
          method: "PUT",
          headers: {
            "Content-Type": "application/json",
            "X-Timeview-Client": "browser",
          },
          body: JSON.stringify({
            instanceId: state.instanceId,
            version: state.version,
          }),
          signal: AbortSignal.any([
            controller.signal,
            AbortSignal.timeout(4000),
          ]),
        });
      } catch {
        /* The operator sees a timeout if reports stop arriving. */
      } finally {
        pending = false;
      }
    };
    void report();
    const interval = setInterval(() => void report(), 5000);
    return () => {
      stopped = true;
      controller.abort();
      clearInterval(interval);
    };
  }, [enabled]);
}
