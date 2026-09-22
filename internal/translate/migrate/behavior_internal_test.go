package migrate

import "testing"

func TestParseDuration(t *testing.T) {
	for text, want := range map[string]string{
		"1s":                              "1.0",
		"0.5 s":                           "0.5",
		"80ms":                            "0.08",
		"2 min":                           "120.0",
		"1.5h":                            "5400.0",
		"7":                               "0.007",
		"200":                             "0.2",
		"100m":                            "6000.0",
		"2 wk":                            "1209600.0",
		"5 millisec":                      "0.005",
		"t = 41 seconds":                  "41.0",
		"t = 1 minute 31 seconds":         "91.0",
		"t = 52 seconds 790 milliseconds": "52.79",
		"simtime = 2 min 3 s":             "123.0",
		"t = 0":                           "0.0",
		"":                                "",
		"t =":                             "",
		"ditSetup s":                      "",
		"1 2":                             "",
		"1 min 30":                        "",
		"3 fortnights":                    "",
		"1s and 2s":                       "",
		"1e308 d":                         "",
		"1e400":                           "",
		"1.5e308 s 1.5e308 s":             "",
	} {
		got, _, ok := parseDuration(text)
		if ok != (want != "") || got != want {
			t.Errorf("parseDuration(%q) = %q, %v; want %q", text, got, ok, want)
		}
	}
}

// The unit binds to the primary before it, so a primary keeps its form and
// anything else is parenthesized to be one quantity in seconds.
func TestInSeconds(t *testing.T) {
	for expr, want := range map[string]string{
		"0.2":                                   "0.2 [SI::s]",
		"this.settle":                           "this.settle [SI::s]",
		"RandomFunctions::uniform(0.1, 0.25)":   "RandomFunctions::uniform(0.1, 0.25) [SI::s]",
		"this.settle * 0.001":                   "(this.settle * 0.001) [SI::s]",
		"(this.coarse + this.fine) * 0.001":     "((this.coarse + this.fine) * 0.001) [SI::s]",
		"this.settle - 1.0":                     "(this.settle - 1.0) [SI::s]",
		"if this.fast ? 0.1 else 0.2":           "(if this.fast ? 0.1 else 0.2) [SI::s]",
		"this.settle > 1.0 ? this.settle : 0.1": "(this.settle > 1.0 ? this.settle : 0.1) [SI::s]",
	} {
		if got := inSeconds(expr); got != want {
			t.Errorf("inSeconds(%q) = %q; want %q", expr, got, want)
		}
	}
}
