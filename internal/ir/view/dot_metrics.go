package view

import "math"

// Graphviz sets a label's text with Pango at 96 dots an inch, each glyph's
// advance hinted to a whole pixel and a line standing the font's ascent and
// descent each rounded up to one. Which font is the installation's to say: the
// Helvetica the skins ask for is DejaVu Sans where no Helvetica is installed,
// the widest of its usual substitutes, so text is measured by DejaVu's metrics
// and a stated box holds the lines Graphviz finds room for, at worst a little
// spare.
const (
	dotPixelsPerPoint = 96.0 / 72
	dotAscent         = 0.928
	dotDescent        = 0.236
)

// dotTextWidth is the width in points a line of text is set at, at a font size,
// plain or bold: one point over the floor of the exact width, which is the span
// older Graphviz releases report and current releases' exact span never exceeds.
func dotTextWidth(text string, size float64, bold bool) float64 {
	px := size * dotPixelsPerPoint
	var width float64
	for _, r := range text {
		width += math.Round(dotAdvance(r, bold) * px)
	}
	return math.Floor(width/dotPixelsPerPoint) + 1
}

// dotLineHeight is the height in points a line of a font size is set at.
func dotLineHeight(size float64) float64 {
	px := size * dotPixelsPerPoint
	return (math.Ceil(dotAscent*px) + math.Ceil(dotDescent*px)) / dotPixelsPerPoint
}

// dotAdvance is a glyph's advance in ems, plain or bold, a glyph the tables lack
// taking the average.
func dotAdvance(r rune, bold bool) float64 {
	ascii, others, average := dotPlainASCII, dotPlainOthers, dotGlyphEm
	if bold {
		ascii, others, average = dotBoldASCII, dotBoldOthers, dotBoldGlyphEm
	}
	if 0x20 <= r && r <= 0x7e {
		return ascii[r-0x20]
	}
	if w, ok := others[r]; ok {
		return w
	}
	return average
}

// DejaVu Sans's advances, in thousandths of an em, over printable ASCII (from
// the space) and the punctuation a label carries beyond it.
var dotPlainASCII = emsOf([]int{
	318, 401, 460, 838, 636, 950, 780, 275, 390, 390, 500, 838, 318, 361, 318, 337, // space to /
	636, 636, 636, 636, 636, 636, 636, 636, 636, 636, 337, 337, 838, 838, 838, 531, // 0 to ?
	1000, 684, 686, 698, 770, 632, 575, 775, 752, 295, 295, 656, 557, 863, 748, 787, // @ to O
	603, 787, 695, 635, 611, 732, 684, 989, 685, 611, 685, 390, 337, 390, 838, 500, // P to _
	500, 613, 635, 550, 635, 615, 352, 635, 634, 278, 278, 579, 278, 974, 634, 612, // ` to o
	635, 635, 411, 521, 392, 634, 592, 818, 592, 592, 525, 636, 337, 636, 838, // p to ~
})

var dotBoldASCII = emsOf([]int{
	348, 456, 521, 838, 696, 1002, 872, 306, 457, 457, 523, 838, 380, 415, 380, 365, // space to /
	696, 696, 696, 696, 696, 696, 696, 696, 696, 696, 400, 400, 838, 838, 838, 580, // 0 to ?
	1000, 774, 762, 734, 830, 683, 683, 821, 837, 372, 372, 775, 637, 995, 837, 850, // @ to O
	733, 850, 770, 720, 682, 812, 774, 1103, 771, 724, 725, 457, 365, 457, 838, 500, // P to _
	500, 675, 716, 593, 716, 678, 435, 716, 712, 343, 343, 665, 343, 1042, 712, 687, // ` to o
	716, 716, 493, 595, 478, 712, 652, 924, 645, 652, 582, 712, 365, 712, 838, // p to ~
})

var dotPlainOthers = map[rune]float64{
	'\u00a0': 0.318, '«': 0.612, '»': 0.612, '·': 0.318, '×': 0.838, '÷': 0.838,
	'–': 0.500, '—': 1.000, '‘': 0.318, '’': 0.318, '“': 0.518, '”': 0.518, '•': 0.590, '…': 1.000,
	'→': 0.838, '←': 0.838, '↔': 0.838, '≤': 0.838, '≥': 0.838, '≠': 0.838,
}

var dotBoldOthers = map[rune]float64{
	'\u00a0': 0.348, '«': 0.646, '»': 0.646, '·': 0.380, '×': 0.838, '÷': 0.838,
	'–': 0.500, '—': 1.000, '‘': 0.380, '’': 0.380, '“': 0.657, '”': 0.657, '•': 0.639, '…': 1.000,
	'→': 0.838, '←': 0.838, '↔': 0.838, '≤': 0.838, '≥': 0.838, '≠': 0.838,
}

// emsOf converts advances in thousandths of an em to ems.
func emsOf(thousandths []int) []float64 {
	ems := make([]float64, len(thousandths))
	for i, w := range thousandths {
		ems[i] = float64(w) / 1000
	}
	return ems
}
