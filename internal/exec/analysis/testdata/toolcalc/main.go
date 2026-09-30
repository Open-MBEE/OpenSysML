// Command toolcalc stands in for a tool-computed calc's external tool in the tests of
// the tool protocol: it reads the one JSON request on standard input and answers
// Tmax = mass*power in the unit TOOL_CALC_UNIT names (K by default), plus warn =
// mass*power > 100. TOOL_CALC_MODE picks a failure instead: error, exit or hang.
// It is built by the tests that run it and lives under testdata, out of every build.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// ModeEnv selects how the stand-in answers; the empty mode answers the request.
const ModeEnv = "TOOL_CALC_MODE"

// UnitEnv names the unit the Tmax answer is measured in.
const UnitEnv = "TOOL_CALC_UNIT"

// value is one protocol value: a JSON value and, for a quantity, its unit.
type value struct {
	Value json.RawMessage `json:"value"`
	Unit  string          `json:"unit,omitempty"`
}

// request is the protocol's request object.
type request struct {
	ToolName string           `json:"toolName"`
	URI      string           `json:"uri"`
	Inputs   map[string]value `json:"inputs"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "toolcalc:", err)
		os.Exit(3)
	}
}

func run() error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	var req request
	if err := json.Unmarshal(data, &req); err != nil {
		return fmt.Errorf("request is not the protocol's object: %w", err)
	}
	switch mode := os.Getenv(ModeEnv); mode {
	case "", "answer":
		return reply(answer(req))
	case "error":
		return json.NewEncoder(os.Stdout).Encode(map[string]string{"error": "thermal model did not converge"})
	case "exit":
		fmt.Fprintln(os.Stderr, "license server unreachable")
		os.Exit(2)
	case "hang":
		time.Sleep(time.Minute)
		return reply(answer(req))
	default:
		return fmt.Errorf("%s=%q is not a mode", ModeEnv, mode)
	}
	return nil
}

// number reads one input's magnitude as the protocol carries it.
func number(in value) (float64, error) {
	var n float64
	if err := json.Unmarshal(in.Value, &n); err != nil {
		return 0, fmt.Errorf("input %s is not a number: %w", in.Value, err)
	}
	return n, nil
}

// answer is the stand-in's computation: the product of the mass and power sent.
func answer(req request) map[string]value {
	mass, err := number(req.Inputs["mass"])
	if err != nil {
		mass = 0
	}
	power, err := number(req.Inputs["power"])
	if err != nil {
		power = 0
	}
	product := mass * power
	unit := os.Getenv(UnitEnv)
	if unit == "" {
		unit = "K"
	}
	return map[string]value{
		"Tmax": {Value: raw(product), Unit: unit},
		"warn": {Value: raw(product > 100)},
	}
}

func raw(x any) json.RawMessage {
	data, err := json.Marshal(x)
	if err != nil {
		return json.RawMessage("null")
	}
	return data
}

// reply writes the protocol's reply object.
func reply(outputs map[string]value) error {
	return json.NewEncoder(os.Stdout).Encode(struct {
		Outputs map[string]value `json:"outputs"`
	}{Outputs: outputs})
}
