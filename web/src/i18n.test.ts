import assert from "node:assert/strict";
import { test } from "node:test";
import { operationError } from "./i18n.ts";

test("operation errors always have a Japanese fallback", () => {
  assert.equal(
    operationError(),
    "操作に失敗しました。自動再送は行いません。現在の状態を確認してください。",
  );
  assert.equal(
    operationError("設定が競合しました"),
    "設定が競合しました。自動再送は行いません。現在の状態を確認してください。",
  );
});
