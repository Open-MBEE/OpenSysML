- **The Legend of the Red Dragon example, played in the browser.** `make lord-web` builds
  `examples/lord-demo/web` — the model runtime compiled to WebAssembly — into a static page
  that plays `examples/lord-demo/lord.sysml` as the door game looked: a terminal-styled screen
  with the warrior's stats, the menu of the state the day machine is in, and the keys the game
  took. Each tab loads the model into a runtime of its own with `LordPlay::hero` instantiated,
  with no server behind it; every key is one of the `accept` triggers out of the current state,
  checked and guarded by the machine, a deed that takes an argument (a blessing, a wager, a
  weapon, a favour) asks for it and binds the model's own value, and every figure on the page is
  read from the instance. No rule of the game is in the page or the wrapper; the README's *In
  the browser* section walks through it.
