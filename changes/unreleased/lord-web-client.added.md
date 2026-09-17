- **The Legend of the Red Dragon example, played in the browser.** `go run ./cmd/lord-web`
  serves `examples/lord-demo/lord.sysml` as the door game looked: a terminal-styled page with
  the warrior's stats, the menu of the state the day machine is in, and the keys the game
  took. Each browser gets a runtime of its own with `LordPlay::hero` instantiated; every key
  is one of the `accept` triggers out of the current state, checked and guarded by the machine,
  a deed that takes an argument (a blessing, a wager, a weapon, a favour) asks for it and binds
  the model's own value, and every figure on the page is read from the instance. No rule of the
  game is in the page or the server; the README's *In the browser* section walks through it.
