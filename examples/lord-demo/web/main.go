//go:build js && wasm

// Command web runs the Legend of the Red Dragon model in the browser. Compiled
// to WebAssembly, it hands the page a `lord` object whose calls take and return
// JSON: the page fetches lord.sysml, loads it, and plays one warrior in the
// runtime that lives in the tab. No server is involved; no rule lives here.
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"syscall/js"

	"github.com/Open-MBEE/OpenSysML/examples/lord-demo/web/lord"
)

// The one warrior the tab plays, and the model it is played against.
var (
	source []byte
	game   *lord.Game
)

var errNoModel = errors.New("no model is loaded")

// playRequest is a key from the menu with the inputs its choice asked for.
type playRequest struct {
	Key    string            `json:"key"`
	Inputs map[string]string `json:"inputs"`
}

func main() {
	api := js.Global().Get("Object").New()
	api.Set("load", call(load))
	api.Set("view", call(view))
	api.Set("newGame", call(newGame))
	api.Set("play", call(play))
	api.Set("retire", call(retire))
	js.Global().Set("lord", api)
	select {}
}

// call wraps a handler as a JavaScript function taking one JSON string and
// returning one: the handler's result, or {"error": ...} when it fails.
func call(handler func(arg string) (any, error)) js.Func {
	return js.FuncOf(func(_ js.Value, args []js.Value) any {
		arg := ""
		if len(args) > 0 && args[0].Type() == js.TypeString {
			arg = args[0].String()
		}
		result, err := handler(arg)
		if err != nil {
			result = map[string]string{"error": err.Error()}
		}
		out, err := json.Marshal(result)
		if err != nil {
			return `{"error":"the result could not be encoded"}`
		}
		return string(out)
	})
}

// load takes the model's text, proving it plays before any warrior is made.
func load(arg string) (any, error) {
	var text string
	if err := json.Unmarshal([]byte(arg), &text); err != nil {
		return nil, fmt.Errorf("%w: %v", lord.ErrBadArgument, err)
	}
	if _, err := lord.NewGame([]byte(text), 0, lord.Character{}); err != nil {
		return nil, fmt.Errorf("lord.sysml: %w", err)
	}
	source = []byte(text)
	game = nil
	return view("")
}

func view(string) (any, error) {
	if source == nil {
		return nil, errNoModel
	}
	if game == nil {
		return lord.CharacterView("No warrior of yours walks the realm. Create one."), nil
	}
	return game.View()
}

func newGame(arg string) (any, error) {
	if source == nil {
		return nil, errNoModel
	}
	var character lord.Character
	if err := json.Unmarshal([]byte(arg), &character); err != nil {
		return nil, fmt.Errorf("%w: %v", lord.ErrBadArgument, err)
	}
	g, err := lord.NewGame(source, lord.RandomSeed(), character)
	if err != nil {
		return nil, err
	}
	game = g
	return game.Welcome()
}

func play(arg string) (any, error) {
	if game == nil {
		return nil, fmt.Errorf("%w: no warrior walks the realm", lord.ErrNoSuchCommand)
	}
	var req playRequest
	if err := json.Unmarshal([]byte(arg), &req); err != nil {
		return nil, fmt.Errorf("%w: %v", lord.ErrBadArgument, err)
	}
	outcome, err := game.Play(req.Key, req.Inputs)
	if err != nil {
		return nil, err
	}
	return game.Played(outcome)
}

func retire(string) (any, error) {
	game = nil
	return view("")
}
