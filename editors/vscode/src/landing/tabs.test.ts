import assert from "node:assert/strict";
import { test } from "node:test";

import { JSDOM } from "jsdom";

import { wireTabs } from "./tabs";

function tablist(): { list: HTMLElement; tabs: HTMLElement[]; panel: HTMLElement; window: Window & typeof globalThis } {
  const dom = new JSDOM(`<!DOCTYPE html><body>
    <div role="tablist" data-osml-tabs>
      <button type="button" role="tab" id="tab-stack" data-osml-scene="stack" aria-controls="panel">OpenSysML stack</button>
      <button type="button" role="tab" id="tab-mass" data-osml-scene="mass" aria-controls="panel">Saturn V mass roll-up</button>
    </div>
    <div id="panel" role="tabpanel"></div>
  </body>`);
  const { document } = dom.window;
  const list = document.querySelector<HTMLElement>("[data-osml-tabs]")!;
  return {
    list,
    tabs: [...list.querySelectorAll<HTMLElement>('[role="tab"]')],
    panel: document.getElementById("panel")!,
    window: dom.window as unknown as Window & typeof globalThis,
  };
}

test("wireTabs marks the selected tab and labels its panel", () => {
  const { list, tabs, panel } = tablist();
  const set = wireTabs(list, "mass", () => undefined);
  assert.deepEqual(set.ids(), ["stack", "mass"]);
  assert.deepEqual(tabs.map((tab) => tab.getAttribute("aria-selected")), ["false", "true"]);
  assert.deepEqual(tabs.map((tab) => tab.tabIndex), [-1, 0]);
  assert.equal(panel.getAttribute("aria-labelledby"), "tab-mass");
  set.select("stack");
  assert.deepEqual(tabs.map((tab) => tab.getAttribute("aria-selected")), ["true", "false"]);
  assert.equal(panel.getAttribute("aria-labelledby"), "tab-stack");
});

test("a click selects the tab and reports it once", () => {
  const { list, tabs } = tablist();
  const selected: string[] = [];
  wireTabs(list, "stack", (id) => selected.push(id));
  tabs[1].click();
  assert.deepEqual(selected, ["mass"]);
  assert.equal(tabs[1].getAttribute("aria-selected"), "true");
  assert.equal(tabs[0].tabIndex, -1);
});

test("arrow keys move the selection and the focus, wrapping at the ends", () => {
  const { list, tabs, window } = tablist();
  const selected: string[] = [];
  wireTabs(list, "stack", (id) => selected.push(id));
  const press = (tab: HTMLElement, key: string): boolean =>
    tab.dispatchEvent(new window.KeyboardEvent("keydown", { key, bubbles: true, cancelable: true }));
  assert.equal(press(tabs[0], "ArrowRight"), false, "the key is consumed");
  assert.equal(tabs[1].getAttribute("aria-selected"), "true");
  assert.equal(window.document.activeElement, tabs[1]);
  press(tabs[1], "ArrowRight");
  assert.equal(tabs[0].getAttribute("aria-selected"), "true", "Right wraps to the first");
  press(tabs[0], "ArrowLeft");
  assert.equal(tabs[1].getAttribute("aria-selected"), "true", "Left wraps to the last");
  press(tabs[1], "Home");
  assert.equal(tabs[0].getAttribute("aria-selected"), "true");
  press(tabs[0], "End");
  assert.equal(tabs[1].getAttribute("aria-selected"), "true");
  assert.equal(press(tabs[1], "ArrowDown"), true, "another key is left to the page");
  assert.deepEqual(selected, ["mass", "stack", "mass", "stack", "mass"]);
});

test("an aborted signal detaches the tabs", () => {
  const { list, tabs, window } = tablist();
  // The listeners are the page's, so the signal is the page's too.
  const ac = new window.AbortController();
  const selected: string[] = [];
  wireTabs(list, "stack", (id) => selected.push(id), ac.signal);
  ac.abort();
  tabs[1].click();
  assert.deepEqual(selected, []);
});
