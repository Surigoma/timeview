import type { TimerState } from "./timer.ts";

export type DisplayReport = {
  id: string;
  lastSeenMs: number;
  version: number;
  online: boolean;
};
export type DisplaySnapshot = { instanceId: string; displays: DisplayReport[] };
export type AuditEntry = {
  time: string;
  type: "process" | "operation";
  action: string;
  result: "success" | "failure";
  client?: string;
  statusCode?: number;
  errorCode?: string;
};
export type LogSnapshot = {
  instanceId: string;
  entries: AuditEntry[] | null;
  error?: string;
};
export type StreamView = "control" | "touch" | "display";
export function eventURL(view: StreamView): string {
  return (
    "/api/v1/timer/events" +
    (view === "control"
      ? "?topics=displays,logs"
      : view === "touch"
        ? "?topics=displays"
        : "")
  );
}
export function belongsToInstance(
  snapshot: { instanceId: string },
  state: TimerState | null,
): boolean {
  return state !== null && snapshot.instanceId === state.instanceId;
}
