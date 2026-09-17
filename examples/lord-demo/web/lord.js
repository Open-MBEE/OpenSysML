// The page is a dumb terminal: it shows the view the game reports and turns keys
// into plays. The game is lord.wasm, the model runtime compiled for the browser;
// which keys exist, what they do and what they cost is the model's business,
// reported back in every view.
(() => {
  "use strict";

  const $ = (id) => document.getElementById(id);
  const character = $("character");
  const notice = $("notice");
  const game = $("game");
  const stats = $("stats");
  const location = $("location");
  const log = $("log");
  const menu = $("menu");
  const prompt = $("prompt");

  let screen = null;   // the choices of the current view
  let pending = null;  // {choice, params, index, inputs} while a choice is asking for inputs
  let typed = "";      // digits typed so far towards a numbered option
  let history = [];    // the last few results, oldest first

  // Every call into the game takes and returns JSON; a failure is {error}.
  function api(name, body) {
    const data = JSON.parse(window.lord[name](JSON.stringify(body ?? null)));
    if (data.error) throw new Error(data.error);
    return data;
  }

  function show(view, error) {
    if (view && view.character) {
      character.hidden = false;
      game.hidden = true;
      screen = null;
      tell(view.lines, error);
      $("name").focus();
      return;
    }
    if (!view && !character.hidden) {
      tell([], error);
      return;
    }
    if (view) {
      character.hidden = true;
      game.hidden = false;
      screen = view.screen;
      renderStats(view.warrior);
      location.textContent = view.screen.title;
      renderMenu(view.screen.choices);
      if (view.lines && view.lines.length) history.push({lines: view.lines, error: view.refused});
    }
    if (error) history.push({lines: [error], error: true});
    history = history.slice(-4);
    renderLog();
  }

  // tell puts a word on the character screen, which has no log of its own.
  function tell(lines, error) {
    notice.textContent = error || (lines || []).join("\n");
    notice.classList.toggle("error", Boolean(error));
    notice.hidden = !notice.textContent;
  }

  const pad = (s, n) => String(s).padEnd(n);

  function renderStats(w) {
    const hp = `${w.hitPoints}/${w.maxHitPoints}`;
    const hpClass = w.hitPoints * 2 < w.maxHitPoints ? "hp low" : "hp";
    const spouse = w.spouse === "nobody" ? "single" : `wed to ${w.spouse === "violet" ? "Violet" : "Seth Able"}`;
    const kids = w.children ? `, ${w.children} ${w.children === 1 ? "child" : "children"}` : "";
    const extras = [w.horse && "horse", w.fairy && "fairy", w.innRoom && "a room at the inn", w.bribed && "bartender bribed"].filter(Boolean);
    stats.replaceChildren(
      span("name", pad(w.name, 26)), `Level ${pad(w.level, 3)}`, `Exp ${pad(w.experience, 8)}`, span(hpClass, `HP ${hp}`), "\n",
      `${pad(w.sex === "female" ? "Lady" : "Sir", 5)}${pad(w.class, 21)}`, `Str ${pad(w.strength, 5)}`, `Def ${pad(w.defense, 9)}`, span("gold", `Gold ${w.gold}`), "\n",
      pad(`Charm ${w.charm}  Gems ${w.gems}`, 26), `Forest fights ${pad(w.forestFightsLeft, 3)}`, `Player fights ${pad(w.playerFightsLeft, 3)}`, span("gold", `Bank ${w.bankGold}`), "\n",
      `${spouse}${kids}${extras.length ? " · " + extras.join(", ") : ""}`, w.dragonKills ? `  · dragon kills ${w.dragonKills}` : "", w.playerKills ? `  · warriors slain ${w.playerKills}` : "",
    );
  }

  function span(cls, text) {
    const el = document.createElement("span");
    el.className = cls;
    el.textContent = text;
    return el;
  }

  function renderMenu(choices) {
    menu.replaceChildren(...choices.map((c) => {
      const b = document.createElement("button");
      b.type = "button";
      b.disabled = !c.enabled;
      b.append(kbd(c.key), " ", c.label);
      b.addEventListener("click", () => choose(c));
      return b;
    }));
  }

  function kbd(key) {
    const k = document.createElement("kbd");
    k.textContent = key;
    return k;
  }

  function renderLog() {
    log.replaceChildren(...history.flatMap((entry, i) => {
      const cls = entry.error ? "error" : i === history.length - 1 ? "fresh" : "old";
      return entry.lines.map((line) => span(cls, line + "\n"));
    }));
    const cursor = span("cursor", "");
    log.append(cursor);
  }

  // choose starts a choice: straight to the model if it needs nothing, else the prompts.
  function choose(choice) {
    if (!choice.enabled) return;
    if (!choice.params || choice.params.length === 0) {
      play(choice.key, {});
      return;
    }
    pending = {choice, index: 0, inputs: {}};
    renderPrompt();
  }

  function renderPrompt() {
    const param = pending.choice.params[pending.index];
    typed = "";
    prompt.hidden = false;
    prompt.replaceChildren();
    const question = document.createElement("p");
    question.textContent = param.prompt;
    prompt.append(question);
    if (param.options && param.options.length) {
      param.options.forEach((o, i) => {
        const label = document.createElement("label");
        label.className = "option";
        label.append(kbd(String(i + 1)), " ", o.label);
        if (o.detail) label.append(" ", span("detail", o.detail));
        label.addEventListener("click", () => answer(o.value));
        prompt.append(label);
      });
      const cancel = document.createElement("label");
      cancel.className = "option";
      cancel.append(kbd("Esc"), " Never mind");
      cancel.addEventListener("click", cancelPrompt);
      prompt.append(cancel);
    } else {
      const input = document.createElement("input");
      input.name = param.name;
      input.autocomplete = "off";
      input.inputMode = param.integer ? "numeric" : "text";
      prompt.append(input);
      const submit = document.createElement("button");
      submit.type = "submit";
      submit.append(kbd("Enter"), " Done");
      prompt.append(submit);
      input.focus();
    }
  }

  function answer(value) {
    const param = pending.choice.params[pending.index];
    pending.inputs[param.name] = value;
    pending.index++;
    if (pending.index < pending.choice.params.length) {
      renderPrompt();
      return;
    }
    const {choice, inputs} = pending;
    cancelPrompt();
    play(choice.key, inputs);
  }

  function cancelPrompt() {
    pending = null;
    typed = "";
    prompt.hidden = true;
    prompt.replaceChildren();
  }

  // pick takes a digit towards a numbered option, answering as soon as no
  // longer number could name one, or on Enter.
  function pick(options, key) {
    if (/^[0-9]$/.test(key)) typed += key;
    const n = Number(typed);
    const complete = key === "Enter" || n * 10 > options.length;
    if (!complete) {
      prompt.querySelector("p").textContent = `${pending.choice.params[pending.index].prompt} ${typed}`;
      return;
    }
    typed = "";
    const option = options[n - 1];
    if (option) answer(option.value);
    else renderPrompt();
  }

  function play(key, inputs) {
    try {
      show(api("play", {key, inputs}));
    } catch (err) {
      show(null, err.message);
    }
  }

  prompt.addEventListener("submit", (event) => {
    event.preventDefault();
    const input = prompt.querySelector("input");
    if (input) answer(input.value.trim());
  });

  $("character-form").addEventListener("submit", (event) => {
    event.preventDefault();
    const form = new FormData(event.target);
    const body = {
      Name: form.get("name").trim(),
      Female: form.get("sex") === "female",
      Class: form.get("class"),
    };
    history = [];
    try {
      show(api("newGame", body));
    } catch (err) {
      show(null, err.message);
    }
  });

  $("retire").addEventListener("click", (event) => {
    event.preventDefault();
    cancelPrompt();
    history = [];
    try {
      show(api("retire"));
    } catch (err) {
      show(null, err.message);
    }
  });

  // Keys: on a prompt with options, the digits pick one; otherwise a menu key
  // presses its choice. Typing in a text box is left alone.
  document.addEventListener("keydown", (event) => {
    if (event.ctrlKey || event.metaKey || event.altKey) return;
    if (event.key === "Escape" && pending) {
      cancelPrompt();
      return;
    }
    if (event.target instanceof HTMLInputElement || !screen) return;
    if (pending) {
      const param = pending.choice.params[pending.index];
      if (param.options && (/^[0-9]$/.test(event.key) || event.key === "Enter")) pick(param.options, event.key);
      return;
    }
    const key = event.key.toUpperCase();
    const choice = screen.choices.find((c) => c.key === key);
    if (choice) {
      event.preventDefault();
      choose(choice);
    }
  });

  // instantiate compiles lord.wasm: streamed where the server calls it
  // application/wasm, from its bytes where a plain file server does not.
  async function instantiate(go) {
    const response = await fetch("lord.wasm");
    if (!response.ok) throw new Error(`lord.wasm: ${response.status} ${response.statusText}`);
    const type = (response.headers.get("Content-Type") || "").split(";", 1)[0].trim();
    if (type === "application/wasm") return WebAssembly.instantiateStreaming(response, go.importObject);
    return WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
  }

  // Start the runtime, then hand it the model; the game exists only in this tab.
  async function boot() {
    show({character: true, lines: ["Waking the realm..."]});
    const go = new Go();
    const wasm = await instantiate(go);
    go.run(wasm.instance);
    const model = await fetch("lord.sysml");
    if (!model.ok) throw new Error(`lord.sysml: ${model.status} ${model.statusText}`);
    show(api("load", await model.text()));
    $("begin").disabled = false;
  }

  boot().catch((err) => show({character: true}, err.message));
})();
