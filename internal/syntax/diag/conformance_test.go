package diag

import "testing"

// The default mode is the zero value, so a caller that names no mode gets the
// behavior it had before the option existed.
func TestDefaultIsTheZeroValue(t *testing.T) {
	var mode ConformanceMode
	if mode != ConformanceDefault || mode.IsStrict() {
		t.Fatalf("zero value = %v, want the default mode", mode)
	}
}

func TestModeStrings(t *testing.T) {
	for mode, want := range map[ConformanceMode]string{ConformanceDefault: "default", ConformanceStrict: "strict", ConformanceMode(7): "ConformanceMode(7)"} {
		if got := mode.String(); got != want {
			t.Errorf("ConformanceMode(%d).String() = %q, want %q", int(mode), got, want)
		}
	}
}

func TestModeOf(t *testing.T) {
	if ConformanceModeOf(true) != ConformanceStrict || ConformanceModeOf(false) != ConformanceDefault {
		t.Fatal("ConformanceModeOf must map true to strict and false to default")
	}
}

func TestParseMode(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    ConformanceMode
		wantErr bool
	}{
		{in: "", want: ConformanceDefault},
		{in: "default", want: ConformanceDefault},
		{in: "strict", want: ConformanceStrict},
		{in: "Strict", wantErr: true},
		{in: "lenient", wantErr: true},
	} {
		got, err := ParseConformanceMode(tc.in)
		if (err != nil) != tc.wantErr {
			t.Errorf("ParseConformanceMode(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
			continue
		}
		if err == nil && got != tc.want {
			t.Errorf("ParseConformanceMode(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
