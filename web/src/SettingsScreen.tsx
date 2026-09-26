import { useState } from "react";
import type { FormEvent } from "react";
import { defaultBindings, keyLabel, storeBindings } from "./timer";
import type { Binding, TimerState } from "./timer";
import type { TimerConnection } from "./useTimer";

export function SettingsScreen({
  timer,
  presets,
  bindings,
  record,
  setRecord,
  setBindings,
}: {
  timer: TimerConnection;
  presets: string[];
  bindings: Binding[];
  record: string | null;
  setRecord: (action: string | null) => void;
  setBindings: (bindings: Binding[]) => void;
}) {
  const [toast, setToast] = useState("");
  const state = timer.state!;
  const copy = async (text: string) => {
    // LAN HTTP may not provide navigator.clipboard; the manual field remains usable.
    try {
      await navigator.clipboard.writeText(text);
      setToast("コピーしました");
    } catch {
      setToast("下のURLを選択してコピーしてください");
    }
    setTimeout(() => setToast(""), 3000);
  };
  return (
    <div className="settings-grid">
      <SettingsPanel timer={timer} />
      <PresetEditor timer={timer} presets={presets} />
      <section className="panel">
        <div className="panel-title">
          <h2>キー割り当て</h2>
          <button
            className="small"
            onClick={() => {
              const defaults = defaultBindings.map((binding) => ({
                ...binding,
              }));
              void timer
                .send("/bindings", { bindings: storeBindings(defaults) }, "PUT")
                .then((ok) => {
                  if (ok) setBindings(defaults);
                });
              setRecord(null);
            }}
          >
            既定に戻す
          </button>
        </div>
        <p className="hint">
          変更を押してキーを入力。Escでキャンセル。変更はJSON設定へ保存します。
        </p>
        <div className="bindings">
          {bindings.map((binding) => (
            <div key={binding.action}>
              <span>{binding.label}</span>
              <button
                className="small"
                onClick={() => setRecord(binding.action)}
              >
                {record === binding.action ? "キーを入力…" : keyLabel(binding)}
              </button>
            </div>
          ))}
        </div>
      </section>
      <section className="panel">
        <div className="panel-title">
          <h2>外部連携</h2>
          <span className="badge">
            {state.browserOnly ? "参照のみ" : "HTTP"}
          </span>
        </div>
        <p className="hint">
          {state.browserOnly
            ? "ブラウザ操作限定中。外部連携では状態取得とSSEだけを利用できます。"
            : "認証不要。同じ分離ネットワークから操作できます。"}
        </p>
        <label>
          演台URL
          <input
            readOnly
            value={`${location.origin}/display`}
            onFocus={(event) => event.target.select()}
          />
        </label>
        <button
          className="small"
          onClick={() => void copy(`${location.origin}/display`)}
        >
          URLをコピー
        </button>
        <label>
          タイマーAPI
          <input
            readOnly
            value={`${location.origin}/api/v1/timer`}
            onFocus={(event) => event.target.select()}
          />
        </label>
        <p aria-live="polite">{toast}</p>
        <pre>
          {state.browserOnly
            ? "GET /api/v1/timer\nGET /api/v1/timer/events\n\n変更API: 403 BROWSER_ONLY"
            : 'PUT /api/v1/timer/blackout\nContent-Type: application/json\n\n{"enabled": true}'}
        </pre>
        <p className="hint">
          設定・定型文・キー割り当てはJSONへ保存します。再起動時も暗転はONです。
        </p>
      </section>
    </div>
  );
}

function SettingsPanel({ timer }: { timer: TimerConnection }) {
  const [editing, setEditing] = useState<TimerState | null>(null);
  const state = editing ?? timer.state!;
  const update = (patch: Partial<TimerState>) =>
    setEditing({ ...state, ...patch });
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const body = {
      ...(state.state === "idle"
        ? { durationSeconds: state.durationSeconds }
        : {}),
      warning1Seconds: state.warning1Seconds,
      warning2Seconds: state.warning2Seconds,
      displayMode: state.displayMode,
      flash: state.flash,
      browserOnly: state.browserOnly,
      colors: state.colors,
    };
    const ok = await timer.send(
      "",
      body,
      "PATCH",
      `"${state.instanceId}:${state.version}"`,
    );
    if (ok) setEditing(null);
  };
  return (
    <section className="panel">
      <div className="panel-title">
        <h2>タイマー・表示設定</h2>
        <span className="badge">{editing ? "編集中" : "同期済み"}</span>
      </div>
      <form onSubmit={(event) => void submit(event)}>
        <fieldset
          disabled={!timer.connected || timer.busy}
          className="form-fields"
        >
          <div className="duration-fields">
            <span>持ち時間（待機中のみ変更可能）</span>
            {(["時", "分", "秒"] as const).map((label, index) => {
              const parts = [
                Math.floor(state.durationSeconds / 3600),
                Math.floor(state.durationSeconds / 60) % 60,
                state.durationSeconds % 60,
              ];
              return (
                <label key={label}>
                  {label}
                  <input
                    type="number"
                    min={0}
                    max={index === 0 ? 24 : 59}
                    required
                    disabled={timer.state?.state !== "idle"}
                    value={parts[index]}
                    onChange={(event) => {
                      parts[index] = Number(event.target.value);
                      update({
                        durationSeconds:
                          parts[0] * 3600 + parts[1] * 60 + parts[2],
                      });
                    }}
                  />
                </label>
              );
            })}
          </div>
          <div className="two-columns">
            <label>
              第1警告（残り秒）
              <input
                type="number"
                min={0}
                max={state.durationSeconds}
                required
                value={state.warning1Seconds}
                onChange={(event) =>
                  update({ warning1Seconds: Number(event.target.value) })
                }
              />
            </label>
            <label>
              第2警告（残り秒）
              <input
                type="number"
                min={0}
                max={state.warning1Seconds}
                required
                value={state.warning2Seconds}
                onChange={(event) =>
                  update({ warning2Seconds: Number(event.target.value) })
                }
              />
            </label>
          </div>
          <label>
            表示モード
            <select
              value={state.displayMode}
              onChange={(event) =>
                update({
                  displayMode: event.target.value as TimerState["displayMode"],
                })
              }
            >
              <option value="timer_and_message">タイマー＋カンペ</option>
              <option value="timer">タイマーのみ</option>
              <option value="message">カンペのみ</option>
            </select>
          </label>
          <div className="colors">
            {(["normal", "warning1", "warning2", "overtime"] as const).map(
              (key, index) => (
                <label key={key}>
                  {["通常", "第1警告", "第2警告", "時間超過"][index]}
                  <input
                    type="color"
                    value={state.colors[key]}
                    onChange={(event) =>
                      update({
                        colors: { ...state.colors, [key]: event.target.value },
                      })
                    }
                  />
                </label>
              ),
            )}
          </div>
          <label className="checkbox">
            <input
              type="checkbox"
              checked={state.flash}
              onChange={(event) => update({ flash: event.target.checked })}
            />
            時間超過時にゆっくり点滅
          </label>
          <label className="checkbox">
            <input
              type="checkbox"
              checked={state.browserOnly}
              onChange={(event) =>
                update({ browserOnly: event.target.checked })
              }
            />
            ブラウザからの操作だけを許可
          </label>
          <p className="hint">
            有効時も外部APIからの状態取得は可能ですが、変更操作は拒否します。
          </p>
          <div>
            <button type="submit" className="primary">
              設定を反映
            </button>
            <button type="button" onClick={() => setEditing(null)}>
              最新の設定を読み直す
            </button>
          </div>
        </fieldset>
      </form>
      <p className="hint">
        設定が競合した場合は読み直してから再編集してください。
      </p>
    </section>
  );
}

function PresetEditor({
  timer,
  presets,
}: {
  timer: TimerConnection;
  presets: string[];
}) {
  const [draft, setDraft] = useState<string[] | null>(null);
  const values =
    draft ?? Array.from({ length: 9 }, (_, index) => presets[index] ?? "");
  return (
    <section className="panel">
      <div className="panel-title">
        <h2>定型文</h2>
        <span className="badge">最大9件</span>
      </div>
      <p className="hint">ボタン・テンキーから即時送信するメッセージです。</p>
      <div className="preset-editor">
        {values.map((text, index) => (
          <label key={index}>
            <kbd>{index + 1}</kbd>
            <input
              value={text}
              placeholder={`定型文 ${index + 1}`}
              onChange={(event) => {
                const next = [...values];
                next[index] = event.target.value;
                setDraft(next);
              }}
            />
          </label>
        ))}
      </div>
      <button
        className="primary"
        disabled={
          !timer.connected ||
          timer.busy ||
          values.some((value) => Array.from(value).length > 500)
        }
        onClick={() => {
          void timer.send("/presets", { presets: values }, "PUT").then((ok) => {
            if (ok) setDraft(null);
          });
        }}
      >
        定型文を反映
      </button>
      <button onClick={() => setDraft(null)}>編集を戻す</button>
    </section>
  );
}
