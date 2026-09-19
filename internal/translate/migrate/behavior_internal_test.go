package migrate

import "testing"

func TestParseDuration(t *testing.T) {
	for text, want := range map[string]string{
		"1s":                              "1.0",
		"0.5 s":                           "0.5",
		"80ms":                            "0.08",
		"2 min":                           "120.0",
		"1.5h":                            "5400.0",
		"7":                               "7.0",
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
		got, ok := parseDuration(text)
		if ok != (want != "") || got != want {
			t.Errorf("parseDuration(%q) = %q, %v; want %q", text, got, ok, want)
		}
	}
}
