// The diagram panel's controls and regions, which its script finds by id: served by the
// extension, loaded by the script's tests. Free of the vscode module for the webview.

/** PANEL_MARKUP is the body of the diagram panel's document, less its state and script. */
export const PANEL_MARKUP = `<div id="bar">
  <label for="view">View</label>
  <select id="view"></select>
  <label for="style">Style</label>
  <select id="style"></select>
  <span id="kind"></span>
  <details id="relations" hidden>
    <summary title="Show or hide relationships by kind">Relations</summary>
    <div id="relation-list" role="group" aria-label="Relationship kinds"></div>
  </details>
  <span id="nav" role="group" aria-label="Zoom">
    <button id="zoom-out" type="button" title="Zoom out (-)" aria-label="Zoom out">−</button>
    <span id="zoom" aria-live="polite">100%</span>
    <button id="zoom-in" type="button" title="Zoom in (+)" aria-label="Zoom in">+</button>
    <button id="zoom-fit" type="button" title="Fit the whole diagram in view">Fit</button>
    <button id="zoom-reset" type="button" title="Actual size (0)">1:1</button>
    <button id="auto-layout" type="button" title="Lay the diagram out afresh, ignoring the model's Layout and Route annotations; nothing is written">Auto layout</button>
    <button id="reset-layout" type="button" hidden title="Back to the layout the model states">Reset to model</button>
  </span>
</div>
<div id="status"></div>
<div id="diagram" tabindex="-1"></div>
<ul id="menu" hidden role="menu"></ul>
<details id="notices" hidden>
  <summary></summary>
  <ul id="notice-list"></ul>
</details>
<details id="undrawable" hidden>
  <summary></summary>
  <ul id="undrawable-list"></ul>
</details>`;
