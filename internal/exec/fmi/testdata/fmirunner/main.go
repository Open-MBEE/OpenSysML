// Command fmirunner stands in for OPENSYSML_FMI_RUNNER in the engine and
// session tests; it records requests and speaks the fmi/1 or fmi/2 exchange.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const protocol = 2

// scratchDir is this runner's file directory, isolated when FMIRUNNER_SCRATCH
// names one.
func scratchDir() string {
	if dir := strings.TrimSpace(os.Getenv("FMIRUNNER_SCRATCH")); dir != "" {
		_ = os.MkdirAll(dir, 0o700)
		return dir
	}
	return os.TempDir()
}

func scratch(name string) string {
	return filepath.Join(scratchDir(), "fmirunner-"+name)
}

// bump increments one of the runner's counting files and returns the new value.
func bump(name string) int {
	path := scratch(name)
	data, _ := os.ReadFile(path)
	n, _ := strconv.Atoi(strings.TrimSpace(string(data)))
	n++
	_ = os.WriteFile(path, []byte(strconv.Itoa(n)), 0o600)
	return n
}

type experiment struct {
	StartTime float64  `json:"startTime"`
	StopTime  float64  `json:"stopTime"`
	StepSize  *float64 `json:"stepSize,omitempty"`
	Tolerance *float64 `json:"tolerance,omitempty"`
}

type command struct {
	Protocol   int            `json:"protocol"`
	Action     string         `json:"action"`
	FMU        string         `json:"fmu,omitempty"`
	Interface  string         `json:"interface,omitempty"`
	Experiment *experiment    `json:"experiment,omitempty"`
	Start      map[string]any `json:"start,omitempty"`
	Time       *float64       `json:"time,omitempty"`
	Outputs    []string       `json:"outputs,omitempty"`
}

type answer struct {
	Protocol int            `json:"protocol"`
	Action   string         `json:"action"`
	Time     *float64       `json:"time,omitempty"`
	Outputs  map[string]any `json:"outputs,omitempty"`
	Error    *string        `json:"error,omitempty"`
}

// number reads one JSON number out of a start value, whatever Go type it decoded to.
func number(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	}
	return 0, false
}

// odeValue answers one requested output of the decay at time t. The fixture
// names its state y; anything else is a zero-valued model variable.
func odeValue(t, y0, k float64, name string) any {
	if name == "y" {
		return y0 * math.Exp(-k*t)
	}
	return 0.0
}

// startValues reads the two parameters the decay fixture binds once at init.
func startValues(start map[string]any) (float64, float64) {
	y0, k := 1.0, 1.0
	if v, ok := number(start["y0"]); ok {
		y0 = v
	}
	if v, ok := number(start["k"]); ok {
		k = v
	}
	return y0, k
}

// printAnswer writes one JSON reply line to standard output.
func printAnswer(a answer) {
	data, err := json.Marshal(a)
	if err != nil {
		fmt.Fprintln(os.Stderr, "encode answer:", err)
		os.Exit(2)
	}
	fmt.Println(string(data))
}

// fault answers one command with the runner's own error member.
func fault(action, message string) {
	printAnswer(answer{Protocol: protocol, Action: action, Error: &message})
}

// session speaks fmi/2: one init, many steps and reads, on one process.
func session() {
	state := 0.0
	y0, k := 1.0, 1.0
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		file, err := os.OpenFile(scratch("commands.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
		if err != nil {
			fmt.Fprintln(os.Stderr, "record command:", err)
			os.Exit(2)
		}
		_, _ = file.Write(append(append([]byte(nil), line...), '\n'))
		_ = file.Close()

		var cmd command
		if err := json.Unmarshal(line, &cmd); err != nil || cmd.Protocol != protocol {
			fault(cmd.Action, fmt.Sprintf("protocol %d is not a session command", cmd.Protocol))
			continue
		}
		// A test can make the next command fail without restarting the process.
		control, _ := os.ReadFile(scratch("session-fault.txt"))
		switch text := strings.TrimSpace(string(control)); {
		case text == "garbage":
			fmt.Println("not json")
			continue
		case strings.HasPrefix(text, "error:"):
			fault(cmd.Action, strings.TrimPrefix(text, "error:"))
			continue
		case strings.HasPrefix(text, "exit:"):
			fmt.Fprintln(os.Stderr, strings.TrimPrefix(text, "exit:"))
			os.Exit(3)
		}
		switch cmd.Action {
		case "init":
			bump("init-count.txt")
			if cmd.Experiment == nil {
				fault(cmd.Action, "init names no experiment")
				continue
			}
			state = cmd.Experiment.StartTime
			y0, k = startValues(cmd.Start)
			t := state
			printAnswer(answer{Protocol: protocol, Action: cmd.Action, Time: &t})
		case "step":
			if cmd.Time == nil {
				fault(cmd.Action, "step names no time")
				continue
			}
			if *cmd.Time < state-1e-9 {
				fault(cmd.Action, fmt.Sprintf("cannot step from %v to %v", state, *cmd.Time))
				continue
			}
			state = *cmd.Time
			t := state
			printAnswer(answer{Protocol: protocol, Action: cmd.Action, Time: &t})
		case "read":
			values := make(map[string]any, len(cmd.Outputs))
			for _, name := range cmd.Outputs {
				values[name] = odeValue(state, y0, k, name)
			}
			t := state
			printAnswer(answer{Protocol: protocol, Action: cmd.Action, Time: &t, Outputs: values})
		default:
			fault(cmd.Action, fmt.Sprintf("unknown session action %q", cmd.Action))
		}
	}
}

// sessionHang reads one command and never answers it.
func sessionHang() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()
	time.Sleep(time.Hour)
}

// ode answers one whole fmi/1 run at the request's stop time.
func ode(request []byte) string {
	var cmd command
	if err := json.Unmarshal(request, &cmd); err != nil {
		return fmt.Sprintf(`{"protocol":1,"error":%q}`, "the request is not a JSON object")
	}
	stopTime := 0.0
	y0, k := 1.0, 1.0
	if cmd.Experiment != nil {
		stopTime = cmd.Experiment.StopTime
	}
	y0, k = startValues(cmd.Start)
	values := make(map[string]any, len(cmd.Outputs))
	for _, name := range cmd.Outputs {
		values[name] = odeValue(stopTime, y0, k, name)
	}
	data, err := json.Marshal(answer{Protocol: 1, Time: &stopTime, Outputs: values})
	if err != nil {
		return fmt.Sprintf(`{"protocol":1,"error":%q}`, err.Error())
	}
	return string(data)
}

func main() {
	bump("start-count.txt")
	mode, _ := os.ReadFile(scratch("mode.txt"))
	text := strings.TrimSpace(string(mode))
	if text == "session" {
		session()
		return
	}
	if text == "session-hang" {
		sessionHang()
		return
	}
	request, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read stdin: %v\n", err)
		os.Exit(2)
	}
	if err := os.WriteFile(scratch("request.json"), request, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "record request: %v\n", err)
		os.Exit(2)
	}
	switch {
	case text == "" || text == "answer":
		reply, err := os.ReadFile(scratch("reply.json"))
		if err != nil {
			fmt.Fprintf(os.Stderr, "read reply: %v\n", err)
			os.Exit(2)
		}
		fmt.Print(string(reply))
	case text == "ode":
		fmt.Println(ode(request))
	case strings.HasPrefix(text, "error:"):
		fmt.Printf(`{"protocol": 1, "error": %q}`+"\n", strings.TrimPrefix(text, "error:"))
	case strings.HasPrefix(text, "exit:"):
		fmt.Fprintln(os.Stderr, strings.TrimPrefix(text, "exit:"))
		os.Exit(3)
	case text == "hang":
		time.Sleep(time.Hour)
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %q\n", text)
		os.Exit(2)
	}
}
