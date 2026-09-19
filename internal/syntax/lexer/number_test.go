package lexer

import "testing"

func TestIsDecimalValue(t *testing.T) {
	for _, s := range []string{"0", "3", "007", "271828182845904523536"} {
		if !IsDecimalValue(s) {
			t.Errorf("IsDecimalValue(%q) = false", s)
		}
	}
	for _, s := range []string{"", "-3", "+3", "3.0", ".5", "1e3", "3 ", "3x", "false", "INF", "NaN", "."} {
		if IsDecimalValue(s) {
			t.Errorf("IsDecimalValue(%q) = true", s)
		}
	}
}

func TestIsRealValue(t *testing.T) {
	for _, s := range []string{"1.5", ".5", "0.0", "1e3", "1E20", "7.2973525693E-3", ".5e+2"} {
		if !IsRealValue(s) {
			t.Errorf("IsRealValue(%q) = false", s)
		}
	}
	for _, s := range []string{"", "3", "3.", "-1.5", "+.5", "1e", "1.5 ", "1.5.2", "INF", "-INF", "NaN", "."} {
		if IsRealValue(s) {
			t.Errorf("IsRealValue(%q) = true", s)
		}
	}
}
