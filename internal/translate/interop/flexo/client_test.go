package flexo

import (
	"errors"
	"testing"
)

func TestCheckTransportKeepsTheTokenOffPlaintextNetworks(t *testing.T) {
	t.Setenv(EnvPlainHTTP, "")
	for _, tc := range []struct {
		name          string
		sysml, layer1 string
		plaintext     string
	}{
		{"compose defaults", DefaultSysMLV2URL, DefaultLayer1URL, ""},
		{"loopback ip", "http://127.0.0.1:8083", "http://[::1]:8080", ""},
		{"localhost subdomain", "http://flexo.localhost:8083", DefaultLayer1URL, ""},
		{"https anywhere", "https://flexo.example.org", "https://layer1.example.org", ""},
		{"plain http host", "http://flexo.example.org:8083", DefaultLayer1URL, "http://flexo.example.org:8083"},
		{"plain http layer1", DefaultSysMLV2URL, "http://10.0.0.5:8080", "http://10.0.0.5:8080"},
	} {
		err := Config{SysMLV2URL: tc.sysml, Layer1URL: tc.layer1}.CheckTransport()
		var plain *PlaintextError
		switch {
		case tc.plaintext == "" && err != nil:
			t.Errorf("%s: refused: %v", tc.name, err)
		case tc.plaintext != "" && (!errors.As(err, &plain) || plain.URL != tc.plaintext):
			t.Errorf("%s: want PlaintextError for %s, got %v", tc.name, tc.plaintext, err)
		}
	}

	t.Setenv(EnvPlainHTTP, "1")
	if err := (Config{SysMLV2URL: "http://flexo.example.org:8083", Layer1URL: DefaultLayer1URL}).CheckTransport(); err != nil {
		t.Errorf("the opt-in did not allow plain http: %v", err)
	}
}
