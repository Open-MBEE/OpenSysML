// The extension manifest's diagram contributions: what keys open a diagram, from
// where, and that nothing they name is missing or fires outside a model file.
import assert from "node:assert/strict";
import { test } from "node:test";

import * as manifest from "../package.json";
import { AUTO_OPEN_SETTING } from "./autoopen";
import { MODEL_LANGUAGES, PANEL_TYPE } from "./target";

interface Keybinding {
  command: string;
  key: string;
  mac?: string;
  when?: string;
}

interface MenuItem {
  command: string;
  group?: string;
  when?: string;
}

const OPEN = "opensysml.openDiagram";
const EXPORT = "opensysml.exportDiagram";

const { commands, keybindings, menus } = manifest.contributes as {
  commands: { command: string; title: string; category?: string; icon?: string; enablement?: string }[];
  keybindings: Keybinding[];
  menus: Record<string, MenuItem[]>;
};
const declared = new Set(commands.map((entry) => entry.command));

// Every when clause names both model languages, or the diagram panel, so the
// contribution never fires in another language's editor.
function scopedToModels(when: string | undefined, contextKey: string): void {
  assert.ok(when, "a when clause is required");
  for (const language of MODEL_LANGUAGES) {
    assert.match(when, new RegExp(`${contextKey} == \\.?${language}\\b`), `${when} must name ${language}`);
  }
}

test("keybindings and menus name declared commands only", () => {
  for (const binding of keybindings) {
    assert.ok(declared.has(binding.command), `${binding.key} binds undeclared ${binding.command}`);
  }
  for (const [menu, items] of Object.entries(menus)) {
    for (const item of items) {
      assert.ok(declared.has(item.command), `${menu} lists undeclared ${item.command}`);
    }
  }
});

test("the diagram commands are palette entries under the SysML category, not gated by a context key", () => {
  for (const id of [OPEN, EXPORT]) {
    const command = commands.find((entry) => entry.command === id);
    assert.ok(command, `${id} is declared`);
    assert.equal(command.category, "SysML");
    assert.equal(command.enablement, undefined, `${id} answers for itself instead of greying out`);
  }
  assert.ok(commands.find((entry) => entry.command === OPEN)?.icon, "the title-bar button needs an icon");
});

test("Open Diagram has the diagram-tool and the preview shortcut, each scoped to model files or the panel", () => {
  const open = keybindings.filter((binding) => binding.command === OPEN);
  assert.deepEqual(
    open.map((binding) => [binding.key, binding.mac]),
    [
      ["alt+d", undefined],
      ["ctrl+shift+v", "cmd+shift+v"],
    ],
  );
  for (const binding of open) {
    scopedToModels(binding.when, "editorLangId");
    assert.ok(binding.when?.includes(`activeWebviewPanelId == ${PANEL_TYPE}`), `${binding.key} works from the panel`);
    assert.doesNotMatch(binding.when ?? "", /^\s*!/, `${binding.key} must not fire in any editor by default`);
  }
});

test("both shortcuts keep to the editor text, leaving the Find widget and the terminal their keys", () => {
  for (const key of ["alt+d", "ctrl+shift+v"]) {
    const binding = keybindings.find((entry) => entry.key === key);
    assert.ok(binding);
    assert.match(binding.when ?? "", /^editorTextFocus && \(/, `${key} needs the cursor in the text`);
  }
});

test("Open Diagram is in the editor title bar, the editor context menu and the Explorer menu", () => {
  const title = menus["editor/title"].find((item) => item.command === OPEN);
  assert.ok(title);
  assert.equal(title.group, "navigation", "an icon button, not an overflow entry");
  scopedToModels(title.when, "resourceLangId");

  const context = menus["editor/context"].find((item) => item.command === OPEN);
  assert.ok(context);
  scopedToModels(context.when, "editorLangId");

  const explorer = menus["explorer/context"].find((item) => item.command === OPEN);
  assert.ok(explorer);
  scopedToModels(explorer.when, "resourceExtname");
});

test("Export Diagram sits next to Open Diagram in the editor menus, and nowhere in the Explorer", () => {
  for (const menu of ["editor/title", "editor/context"]) {
    const item = menus[menu].find((entry) => entry.command === EXPORT);
    assert.ok(item, `${menu} offers export`);
    assert.ok(item.when, `${menu} export is scoped`);
  }
  assert.equal(menus["explorer/context"].some((item) => item.command === EXPORT), false);
});

test("no menu item or keybinding is left without a when clause", () => {
  for (const binding of keybindings) {
    assert.ok(binding.when, `${binding.key} is unscoped`);
  }
  for (const [menu, items] of Object.entries(menus)) {
    for (const item of items) {
      assert.ok(item.when, `${menu} ${item.command} is unscoped`);
    }
  }
});

test("the diagram opens on its own by default, and the setting that turns it off is described", () => {
  const { properties } = manifest.contributes.configuration as {
    properties: Record<string, { type: string; default: unknown; description?: string; markdownDescription?: string }>;
  };
  const setting = properties[AUTO_OPEN_SETTING];
  assert.ok(setting, `${AUTO_OPEN_SETTING} is declared`);
  assert.equal(setting.type, "boolean");
  assert.equal(setting.default, true);
  assert.ok(setting.description || setting.markdownDescription, "the setting is described");
});
