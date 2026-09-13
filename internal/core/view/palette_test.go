package view

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
)

// Every palette is offered by name, parsed back to itself, and spelled in the
// list a help text or an error carries; the empty name and a stranger are none.
func TestPaletteRegistry(t *testing.T) {
	want := []Palette{PaletteOkabeIto, PaletteTolBright, PaletteTolMuted, PaletteTolLight,
		PaletteBrewerSet2, PaletteBrewerDark2, PaletteViridis, PaletteCividis}
	if got := Palettes(); !slices.Equal(got, want) {
		t.Errorf("Palettes() = %v, want %v", got, want)
	}
	for _, palette := range Palettes() {
		got, ok := ParsePalette(string(palette))
		if !ok || got != palette {
			t.Errorf("ParsePalette(%q) = %q, %v", palette, got, ok)
		}
		if len(paletteColors[palette]) == 0 {
			t.Errorf("%s has no colours", palette)
		}
		if !strings.Contains(PaletteNames(), string(palette)) {
			t.Errorf("PaletteNames() %q omits %s", PaletteNames(), palette)
		}
	}
	for _, name := range []string{"", "OKABE-ITO", "okabe_ito", "rainbow"} {
		if got, ok := ParsePalette(name); ok || got != "" {
			t.Errorf("ParsePalette(%q) = %q, %v; want none", name, got, ok)
		}
	}
	if got := PaletteNames(); got != "okabe-ito, tol-bright, tol-muted, tol-light, brewer-set2, brewer-dark2, viridis, cividis" {
		t.Errorf("PaletteNames() = %q", got)
	}
}

// An unknown palette is a typed error that wraps ErrUnknownPalette and names
// the palette asked for and every palette there is.
func TestUnknownPaletteError(t *testing.T) {
	var err error = &UnknownPaletteError{Name: "rainbow"}
	if !errors.Is(err, ErrUnknownPalette) {
		t.Errorf("error does not wrap ErrUnknownPalette: %v", err)
	}
	var unknown *UnknownPaletteError
	if !errors.As(err, &unknown) || unknown.Name != "rainbow" {
		t.Errorf("errors.As = %v, %+v", errors.As(err, &unknown), unknown)
	}
	want := `unknown palette "rainbow"; the palettes are okabe-ito, tol-bright, tol-muted, tol-light, brewer-set2, brewer-dark2, viridis, cividis`
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err, want)
	}
}

// Every published colour is `#RRGGBB`, and the palettes hold the colours their
// authors publish, in their order.
func TestPaletteColorsAsPublished(t *testing.T) {
	for palette, colors := range paletteColors {
		for _, color := range colors {
			if len(color) != 7 || color[0] != '#' || strings.ToUpper(color) != color {
				t.Errorf("%s colour %q is not #RRGGBB in upper case", palette, color)
			}
			if parseHex(color).hex() != color {
				t.Errorf("%s colour %q does not round-trip through parseHex", palette, color)
			}
		}
	}
	if got := paletteColors[PaletteOkabeIto]; strings.Join(got, " ") != "#E69F00 #56B4E9 #009E73 #F0E442 #0072B2 #D55E00 #CC79A7 #999999" {
		t.Errorf("okabe-ito = %v", got)
	}
	for _, palette := range []Palette{PaletteViridis, PaletteCividis} {
		colors := paletteColors[palette]
		if len(colors) != 16 {
			t.Errorf("%s has %d stops, want 16", palette, len(colors))
		}
		for i := 1; i < len(colors); i++ {
			if relativeLuminance(parseHex(colors[i])) <= relativeLuminance(parseHex(colors[i-1])) {
				t.Errorf("%s stop %d (%s) is no lighter than stop %d (%s)", palette, i, colors[i], i-1, colors[i-1])
			}
		}
	}
	for _, palette := range Palettes() {
		if palette.Sequential() != (palette == PaletteViridis || palette == PaletteCividis) {
			t.Errorf("%s.Sequential() = %v", palette, palette.Sequential())
		}
	}
}

// A qualitative palette's colour for category i is its i-th colour, wrapping
// past its last; a sequential palette's is sampled evenly over its stops,
// category 0 the darkest and the last the lightest.
func TestPaletteColorForCategory(t *testing.T) {
	okabe := paletteColors[PaletteOkabeIto]
	for i := 0; i < 20; i++ {
		if got := PaletteOkabeIto.Color(i, 20); got != okabe[i%len(okabe)] {
			t.Errorf("okabe-ito Color(%d, 20) = %s, want %s", i, got, okabe[i%len(okabe)])
		}
	}
	viridis := paletteColors[PaletteViridis]
	cases := []struct {
		i, n int
		want string
	}{
		{0, 1, viridis[0]}, {0, 2, viridis[0]}, {1, 2, viridis[15]},
		{0, 3, viridis[0]}, {1, 3, viridis[8]}, {2, 3, viridis[15]},
		{1, 4, viridis[5]}, {2, 4, viridis[10]},
		{7, 16, viridis[7]}, {5, 6, viridis[15]},
	}
	for _, tc := range cases {
		if got := PaletteViridis.Color(tc.i, tc.n); got != tc.want {
			t.Errorf("viridis Color(%d, %d) = %s, want %s", tc.i, tc.n, got, tc.want)
		}
	}
	// A sequential palette never runs backwards: a later category is never darker.
	for n := 1; n <= 20; n++ {
		for i := 1; i < n; i++ {
			before, after := parseHex(PaletteCividis.Color(i-1, n)), parseHex(PaletteCividis.Color(i, n))
			if relativeLuminance(after) < relativeLuminance(before) {
				t.Errorf("cividis Color(%d, %d) is darker than Color(%d, %d)", i, n, i-1, n)
			}
		}
	}
	if got := Palette("none").Color(0, 1); got != "" {
		t.Errorf("no palette's Color = %q, want empty", got)
	}
}

// The keyword families are indexed in a fixed order, so a diagram of parts and
// ports takes the first two colours whatever else it lacks; a definition and its
// usages share a family, and a kind no family fits comes last.
func TestPaletteFamilyIndex(t *testing.T) {
	order := []string{"part", "item", "port", "attribute", "action", "state", "requirement", "constraint",
		"connection", "interface", "use case", "case", "allocation", "analysis", "verification", "enum", "occurrence", "flow"}
	if !slices.Equal(paletteFamilies, order) {
		t.Errorf("paletteFamilies = %v, want %v", paletteFamilies, order)
	}
	for i, family := range order {
		if got := familyRank(family); got != i {
			t.Errorf("familyRank(%s) = %d, want %d", family, got, i)
		}
	}
	if got := familyRank(familyOther); got != len(order) {
		t.Errorf("familyRank(other) = %d, want %d", got, len(order))
	}
	kinds := map[string]string{
		"part": "part", "part def": "part", "item": "item", "item def": "item", "port": "port", "port def": "port",
		"attribute": "attribute", "attribute def": "attribute", "action": "action", "action def": "action",
		"perform": "action", "perform action": "action", "state": "state", "state def": "state",
		"requirement": "requirement", "requirement def": "requirement", "constraint": "constraint",
		"connection": "connection", "connection def": "connection", "connect": "connection",
		"interface": "interface", "interface def": "interface", "use case": "use case", "use case def": "use case",
		"case": "case", "case def": "case", "allocation": "allocation", "allocate": "allocation",
		"analysis": "analysis", "analysis def": "analysis", "verification": "verification", "verification def": "verification",
		"enum": "enum", "enum def": "enum", "occurrence": "occurrence", "occurrence def": "occurrence",
		"flow": "flow", "message": "flow", "view": familyOther, "package": familyOther, "region": familyOther,
		"class": familyOther, "statements": familyOther, "": familyOther,
	}
	for kind, want := range kinds {
		if got := paletteFamily(kind); got != want {
			t.Errorf("paletteFamily(%q) = %q, want %q", kind, got, want)
		}
	}
	// The colour of a family is the palette's colour at the family's index.
	w := &dotWriter{palette: PaletteOkabeIto}
	for i, family := range order[:8] {
		if got := w.dotFamilyColor(&Node{Kind: family}); got != paletteColors[PaletteOkabeIto][i] {
			t.Errorf("okabe-ito colour of %s = %s, want %s", family, got, paletteColors[PaletteOkabeIto][i])
		}
	}
	if def, usage := w.dotFamilyColor(&Node{Kind: "part def"}), w.dotFamilyColor(&Node{Kind: "part"}); def != usage {
		t.Errorf("part def is %s and part %s; want one family colour", def, usage)
	}
}

// Black text on every fill reads at WCAG AA: every palette colour as a
// definition's fill, and its usage tint, has a contrast ratio of at least 4.5.
func TestPaletteFillsAreLegible(t *testing.T) {
	if got := contrastWithBlack(rgb{255, 255, 255}); got < 20.99 || got > 21.01 {
		t.Errorf("contrast of black on white = %v, want 21", got)
	}
	if got := contrastWithBlack(rgb{0, 0, 0}); got != 1 {
		t.Errorf("contrast of black on black = %v, want 1", got)
	}
	for _, palette := range Palettes() {
		for i, color := range paletteColors[palette] {
			for _, usage := range []bool{false, true} {
				fill := paletteFill(color, usage)
				if ratio := contrastWithBlack(parseHex(fill)); ratio < minContrast {
					t.Errorf("%s[%d] %s as usage=%v fills %s, contrast %.2f < %v", palette, i, color, usage, fill, ratio, minContrast)
				}
			}
			// A usage is no darker than its definition.
			def, usage := parseHex(paletteFill(color, false)), parseHex(paletteFill(color, true))
			if relativeLuminance(usage) < relativeLuminance(def) {
				t.Errorf("%s[%d] %s: usage fill %s is darker than definition fill %s", palette, i, color, usage.hex(), def.hex())
			}
		}
	}
	// A colour already legible is kept as it is; a dark one is lightened.
	if got := paletteFill("#F0E442", false); got != "#F0E442" {
		t.Errorf("yellow definition fill = %s, want the colour unchanged", got)
	}
	if got := paletteFill("#440154", false); got == "#440154" || contrastWithBlack(parseHex(got)) < minContrast {
		t.Errorf("dark violet definition fill = %s, want a legible tint", got)
	}
	if got := paletteFill("#000000", true); contrastWithBlack(parseHex(got)) < minContrast {
		t.Errorf("black usage fill = %s, not legible", got)
	}
	if got := (rgb{0, 0, 0}).tint(1).hex(); got != "#FFFFFF" {
		t.Errorf("black tinted fully = %s, want white", got)
	}
	if got := (rgb{0, 100, 200}).tint(0.5).hex(); got != "#80B2E4" {
		t.Errorf("half tint = %s", got)
	}
}

// The notice a form that draws no palette writes names the palette and the form
// that does.
func TestPaletteNotice(t *testing.T) {
	want := fmt.Sprintf("palette %s; only the DOT form fills nodes by keyword family", PaletteTolMuted)
	if got := paletteNotice(PaletteTolMuted); got != want {
		t.Errorf("paletteNotice = %q, want %q", got, want)
	}
	for _, kind := range Kinds() {
		if kind.SupportsPalette() != kind.SupportsForm(FormDot) {
			t.Errorf("%s.SupportsPalette() = %v, but SupportsForm(dot) = %v", kind, kind.SupportsPalette(), kind.SupportsForm(FormDot))
		}
	}
}
