import assert from "node:assert/strict";
import { test } from "node:test";
import { setLanguage } from "../i18n.ts";
import {
  bindingLabel,
  defaultBindings,
  matches,
  midiInput,
  midiLabel,
  midiMatches,
  restoreBindings,
  storeBindings,
} from "./bindings.ts";

test("MIDI note and control inputs normalize press and release", () => {
  assert.deepEqual(midiInput([0x92, 36, 127]), {
    binding: { status: 0x92, data1: 36 },
    active: true,
  });
  assert.deepEqual(midiInput([0x82, 36, 0]), {
    binding: { status: 0x92, data1: 36 },
    active: false,
  });
  assert.deepEqual(midiInput([0xb0, 64, 0]), {
    binding: { status: 0xb0, data1: 64 },
    active: false,
  });
  assert.equal(midiInput([0xe0, 0, 0]), null);
  const binding = { ...defaultBindings[0], midi: { status: 0x92, data1: 36 } };
  assert.equal(midiMatches(binding, { status: 0x92, data1: 36 }), true);
  assert.equal(midiLabel(binding.midi), "Note 36 · Ch 3");
});

test("blackout shortcut never also adjusts time; repeats ignored", () => {
  const event = {
    code: "NumpadSubtract",
    ctrlKey: true,
    shiftKey: false,
    altKey: false,
    metaKey: false,
    repeat: false,
    isComposing: false,
  };
  assert.deepEqual(
    defaultBindings
      .filter((binding) => matches(binding, event))
      .map((binding) => binding.action),
    ["blackout"],
  );
  assert.equal(
    defaultBindings.some((binding) =>
      matches(binding, { ...event, repeat: true }),
    ),
    false,
  );
  assert.equal(
    defaultBindings.some((binding) =>
      matches(binding, { ...event, isComposing: true }),
    ),
    false,
  );
});

test("key bindings round-trip without persisting presentation labels", () => {
  const changed = defaultBindings.map((binding) => ({ ...binding }));
  changed[0].code = "KeyS";
  const stored = storeBindings(changed);
  assert.equal("label" in stored[0], false);
  assert.equal(restoreBindings(stored)[0].code, "KeyS");
  assert.equal(restoreBindings(stored)[0].label, "開始 / 再開");
});

test("English localizes binding labels", () => {
  setLanguage("en");
  assert.equal(bindingLabel("start", "en"), "Start / Resume");
  setLanguage("ja");
});
