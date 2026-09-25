// Command toolreply stands in for an external tool answering in one of the manifest's
// reply formats, selected by its first argument: csv-stdout and diverge write a CSV reply
// on standard output, json-file writes a JSON document into the {outputDir} named as its
// second argument and chatter on standard output, lines, lines-regex and chatter write
// text lines, exit3 and exit1 exit with a status, and sleep never answers. It is built by
// the tests that run it and lives under testdata, out of every build.
package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// CounterEnv names the file the diverge mode counts its invocations in, so two calls
// with equal inputs answer differently.
const CounterEnv = "TOOLREPLY_COUNTER"

const header = "T_max,U,v_out,VU,done,code,note\n"

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: toolreply <mode>")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "csv-stdout":
		fmt.Print(header + "340.9,K,36,km/h,true,0,first\n341.2,K,36,km/h,FALSE,7,second\n")
	case "diverge":
		path := os.Getenv(CounterEnv)
		n := 0
		if data, err := os.ReadFile(filepath.Clean(path)); err == nil { // #nosec G304 -- the test names the file
			n, _ = strconv.Atoi(string(bytes.TrimSpace(data)))
		}
		if err := os.WriteFile(path, []byte(strconv.Itoa(n+1)), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, "counter:", err)
			os.Exit(2)
		}
		fmt.Printf("%s%.1f,K,36,km/h,true,7,v%d\n", header, 340.0+float64(n), n)
	case "json-file":
		dir := os.Args[2]
		doc := fmt.Sprintf(`{"results":[{"T_max":341.2,"v_out":36,"done":true,"code":7,"note":%q}],"units":{"T_max":"K","v_out":"km/h"},"error":""}`, dir)
		if err := os.WriteFile(filepath.Join(dir, "result.json"), []byte(doc), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, "json-file:", err)
			os.Exit(2)
		}
		fmt.Println("chatter: wrote result.json")
	case "lines":
		fmt.Print("starting solve\nT_max = 341.2\nv_out: 36\ndone = TRUE\ncode : 7\nnote = it ran\nall done\n")
	case "chatter":
		path := os.Getenv(CounterEnv)
		n := 0
		if data, err := os.ReadFile(filepath.Clean(path)); err == nil { // #nosec G304 -- the test names the file
			n, _ = strconv.Atoi(string(bytes.TrimSpace(data)))
		}
		if err := os.WriteFile(path, []byte(strconv.Itoa(n+1)), 0o600); err != nil {
			fmt.Fprintln(os.Stderr, "counter:", err)
			os.Exit(2)
		}
		fmt.Printf("started at 10:%02d\nT_max = 341.2\nv_out = 36\ndone = TRUE\ncode = 7\nnote = it ran\n", n)
	case "lines-regex":
		fmt.Println("chatter with no separator")
		fmt.Println("T=341.2K v=36km/h done=TRUE code=7 note=it ran")
	case "exit3":
		os.Exit(3)
	case "exit1":
		os.Exit(1)
	case "sleep":
		time.Sleep(time.Minute)
	default:
		fmt.Fprintln(os.Stderr, "unknown mode", os.Args[1])
		os.Exit(2)
	}
}
