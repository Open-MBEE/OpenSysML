package view

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Palette names the set of fill colours the DOT form colours nodes with, one
// per keyword family (part, item, port, …, see paletteFamilies). The empty
// Palette is the black-and-white default; every named one is colourblind-safe.
type Palette string

const (
	// PaletteOkabeIto is the eight-colour palette of Okabe & Ito, "Color
	// Universal Design" (2002, jfly.uni-koeln.de/color).
	PaletteOkabeIto Palette = "okabe-ito"
	// PaletteTolBright, PaletteTolMuted and PaletteTolLight are Paul Tol's
	// qualitative schemes (personal.sron.nl/~pault, technical note 3.2, 2021).
	PaletteTolBright Palette = "tol-bright"
	PaletteTolMuted  Palette = "tol-muted"
	PaletteTolLight  Palette = "tol-light"
	// PaletteBrewerSet2 and PaletteBrewerDark2 are ColorBrewer's Set2 and
	// Dark2 (Cynthia Brewer, colorbrewer2.org; Apache-2.0, notice below).
	PaletteBrewerSet2  Palette = "brewer-set2"
	PaletteBrewerDark2 Palette = "brewer-dark2"
	// PaletteViridis and PaletteCividis are sequential ramps, sampled at 16
	// stops of matplotlib's colour maps (van der Walt & Smith; Nuñez, Anderton
	// & Renslow 2018), darkest first.
	PaletteViridis Palette = "viridis"
	PaletteCividis Palette = "cividis"
)

// Palettes are the palettes a rendering can be asked for, in the order they
// are offered.
func Palettes() []Palette {
	return []Palette{PaletteOkabeIto, PaletteTolBright, PaletteTolMuted, PaletteTolLight,
		PaletteBrewerSet2, PaletteBrewerDark2, PaletteViridis, PaletteCividis}
}

// ParsePalette reports the palette name names, and whether it names one. The
// empty name is no palette: the black-and-white default is asked for by
// stating none.
func ParsePalette(name string) (Palette, bool) {
	for _, palette := range Palettes() {
		if string(palette) == name {
			return palette, true
		}
	}
	return "", false
}

// PaletteNames spells the palettes as a list, for help and error text.
func PaletteNames() string {
	names := make([]string, 0, len(Palettes()))
	for _, palette := range Palettes() {
		names = append(names, string(palette))
	}
	return strings.Join(names, ", ")
}

// ErrUnknownPalette is a palette name that names none. UnknownPaletteError
// wraps it.
var ErrUnknownPalette = errors.New("unknown palette")

// UnknownPaletteError is a palette asked for by a name no palette has; it names
// the palettes there are, so no surface refuses one without saying which.
type UnknownPaletteError struct {
	Name string
}

func (e *UnknownPaletteError) Error() string {
	return fmt.Sprintf("unknown palette %q; the palettes are %s", e.Name, PaletteNames())
}

func (e *UnknownPaletteError) Unwrap() error { return ErrUnknownPalette }

// check is the UnknownPaletteError of a Palette value no registry entry has;
// the empty palette and every registered one pass.
func (p Palette) check() error {
	if p == "" {
		return nil
	}
	if _, ok := ParsePalette(string(p)); !ok {
		return &UnknownPaletteError{Name: string(p)}
	}
	return nil
}

// paletteNotice is the notice a form that draws no palette writes for one asked
// for, so the request is not dropped silently.
func paletteNotice(palette Palette) string {
	return fmt.Sprintf("palette %s; only the DOT form fills nodes by keyword family", palette)
}

// SupportsPalette reports whether a rendering of the kind is drawn as a graph
// of nodes a palette can fill: the kinds the DOT form is written for.
func (k Kind) SupportsPalette() bool { return k.SupportsForm(FormDot) }

// The palettes' colours as their authors publish them, `#RRGGBB`. The
// qualitative sets are in their published order; the sequential ramps are
// matplotlib's viridis and cividis sampled at i/15 for i in 0..15.
//
// ColorBrewer notice: Set2 and Dark2 are colour specifications and designs
// developed by Cynthia Brewer (http://colorbrewer.org/), licensed under the
// Apache License, Version 2.0.
var paletteColors = map[Palette][]string{
	PaletteOkabeIto:    {"#E69F00", "#56B4E9", "#009E73", "#F0E442", "#0072B2", "#D55E00", "#CC79A7", "#999999"},
	PaletteTolBright:   {"#4477AA", "#EE6677", "#228833", "#CCBB44", "#66CCEE", "#AA3377", "#BBBBBB"},
	PaletteTolMuted:    {"#332288", "#88CCEE", "#44AA99", "#117733", "#999933", "#DDCC77", "#CC6677", "#882255", "#AA4499", "#DDDDDD"},
	PaletteTolLight:    {"#77AADD", "#99DDFF", "#44BB99", "#BBCC33", "#AAAA00", "#EEDD88", "#EE8866", "#FFAABB", "#DDDDDD"},
	PaletteBrewerSet2:  {"#66C2A5", "#FC8D62", "#8DA0CB", "#E78AC3", "#A6D854", "#FFD92F", "#E5C494", "#B3B3B3"},
	PaletteBrewerDark2: {"#1B9E77", "#D95F02", "#7570B3", "#E7298A", "#66A61E", "#E6AB02", "#A6761D", "#666666"},
	PaletteViridis: {"#440154", "#481A6C", "#472F7D", "#414487", "#39568C", "#31688E", "#2A788E", "#23888E",
		"#1F988B", "#22A884", "#35B779", "#54C568", "#7AD151", "#A5DB36", "#D2E21B", "#FDE725"},
	PaletteCividis: {"#00224E", "#002E6C", "#1E3A6F", "#35456C", "#47516C", "#575D6D", "#666970", "#757575",
		"#848279", "#948E77", "#A59C74", "#B7A96E", "#C8B866", "#DBC75A", "#EED649", "#FEE838"},
}

// Sequential reports whether the palette is a ramp sampled to fit the
// categories drawn, rather than a fixed set of distinct colours.
func (p Palette) Sequential() bool {
	return p == PaletteViridis || p == PaletteCividis
}

// Color is the palette's colour for category i of n, `#RRGGBB`: a qualitative
// palette's i-th colour, wrapping round past its last, or a sequential ramp
// sampled evenly over its stops with category 0 the darkest.
func (p Palette) Color(i, n int) string {
	colors := paletteColors[p]
	if len(colors) == 0 {
		return ""
	}
	if !p.Sequential() {
		return colors[i%len(colors)]
	}
	if n < 2 || i <= 0 {
		return colors[0]
	}
	if i >= n-1 {
		return colors[len(colors)-1]
	}
	return colors[int(math.Round(float64(i)*float64(len(colors)-1)/float64(n-1)))]
}

// paletteFamilies is the order keyword families take a palette's colours in;
// a kind fitting none comes last.
var paletteFamilies = []string{"part", "item", "port", "attribute", "action", "state", "requirement", "constraint",
	"connection", "interface", "use case", "case", "allocation", "analysis", "verification", "enum", "occurrence", "flow"}

// familyOther is the family of every kind no keyword family fits.
const familyOther = "other"

// familyWords maps a keyword of a node's kind to its family; a kind is placed
// by the first of its words that has one, so `perform action` is an action and
// `analysis case` an analysis.
var familyWords = map[string]string{
	"part": "part", "item": "item", "port": "port", "attribute": "attribute", "action": "action", "state": "state",
	"requirement": "requirement", "constraint": "constraint", "connection": "connection", "connect": "connection",
	"interface": "interface", "case": "case", "allocation": "allocation", "allocate": "allocation",
	"analysis": "analysis", "verification": "verification", "enum": "enum", "occurrence": "occurrence",
	"flow": "flow", "message": "flow", "perform": "action",
}

// paletteFamily is the keyword family of a node kind: `part def` and `part`
// share one, so a definition and its usages share a hue.
func paletteFamily(kind string) string {
	words := strings.Fields(definitionKeyword(kind))
	for i, word := range words {
		if word == "use" && i+1 < len(words) && words[i+1] == "case" {
			return "use case"
		}
		if family, ok := familyWords[word]; ok {
			return family
		}
	}
	return familyOther
}

// familyRank is where a family stands in paletteFamilies, the other family last.
func familyRank(family string) int {
	for i, known := range paletteFamilies {
		if known == family {
			return i
		}
	}
	return len(paletteFamilies)
}

// definitionKeyword is a node kind without the `def` a SysML definition ends
// in, `part` for `part def`; a usage's kind is returned as it is.
func definitionKeyword(kind string) string {
	return strings.TrimSuffix(kind, " def")
}

// kermlClassifierKinds are the KerML classifier keywords, definitions the
// notation writes without `def`.
var kermlClassifierKinds = map[string]bool{
	"class": true, "classifier": true, "subclassifier": true, "datatype": true, "struct": true, "assoc": true,
	"assoc struct": true, "behavior": true, "function": true, "predicate": true, "metaclass": true, "type": true,
}

// isDefinitionKind reports whether a node kind is a definition's notation, a
// `… def` keyword or a KerML classifier, as against a usage's.
func isDefinitionKind(kind string) bool {
	return strings.HasSuffix(kind, " def") || kermlClassifierKinds[kind]
}

// The tint a usage's fill is lightened by, and the contrast black text on a
// fill must reach: WCAG 2 level AA for normal text.
const (
	usageTint   = 0.6
	minContrast = 4.5
)

// paletteFill is the fill a node of a family colour takes: the colour itself for
// a definition, a tint of it for a usage, each lightened toward white until
// black text on it reads at minContrast.
func paletteFill(color string, usage bool) string {
	c := parseHex(color)
	if usage {
		c = c.tint(usageTint)
	}
	return legibleFill(c).hex()
}

// legibleFill lightens a colour toward white, a hundredth at a time, until
// black text on it reaches minContrast; a colour already legible is unchanged.
func legibleFill(c rgb) rgb {
	for step := 0; step <= 100; step++ {
		lightened := c.tint(float64(step) / 100)
		if contrastWithBlack(lightened) >= minContrast {
			return lightened
		}
	}
	return rgb{255, 255, 255}
}

// rgb is a colour as its sRGB channels, each 0 to 255.
type rgb struct{ r, g, b float64 }

// parseHex reads a `#RRGGBB` colour; every palette colour is one.
func parseHex(color string) rgb {
	channel := func(from int) float64 {
		v, _ := strconv.ParseUint(color[from:from+2], 16, 8)
		return float64(v)
	}
	return rgb{channel(1), channel(3), channel(5)}
}

func (c rgb) hex() string {
	return fmt.Sprintf("#%02X%02X%02X", int(math.Round(c.r)), int(math.Round(c.g)), int(math.Round(c.b)))
}

// tint blends the colour toward white by amount, 0 leaving it and 1 white.
func (c rgb) tint(amount float64) rgb {
	blend := func(channel float64) float64 { return math.Round(channel + (255-channel)*amount) }
	return rgb{blend(c.r), blend(c.g), blend(c.b)}
}

// contrastWithBlack is the WCAG 2 contrast ratio of black text on the colour,
// (L + 0.05) / 0.05 for the colour's relative luminance L.
func contrastWithBlack(c rgb) float64 {
	return (relativeLuminance(c) + 0.05) / 0.05
}

// relativeLuminance is the WCAG 2 relative luminance of an sRGB colour.
func relativeLuminance(c rgb) float64 {
	linear := func(channel float64) float64 {
		v := channel / 255
		if v <= 0.03928 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(c.r) + 0.7152*linear(c.g) + 0.0722*linear(c.b)
}
