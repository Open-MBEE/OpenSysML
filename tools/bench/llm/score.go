package llm

import (
	"math"
	"sort"
)

// Stat is a mean with its sample standard deviation over n samples.
type Stat struct {
	Mean  float64 `json:"mean"`
	Stdev float64 `json:"stdev"`
	N     int     `json:"n"`
}

// NewStat computes mean and sample standard deviation.
func NewStat(xs []float64) Stat {
	n := len(xs)
	if n == 0 {
		return Stat{}
	}
	sum := 0.0
	for _, x := range xs {
		sum += x
	}
	mean := sum / float64(n)
	if n < 2 {
		return Stat{Mean: mean, N: n}
	}
	ss := 0.0
	for _, x := range xs {
		ss += (x - mean) * (x - mean)
	}
	return Stat{Mean: mean, Stdev: math.Sqrt(ss / float64(n-1)), N: n}
}

// Aggregate is one arm's metrics: every rate is computed per sample over the
// tasks, then summarised over the samples.
type Aggregate struct {
	Cells          int  `json:"cells"`
	Errors         int  `json:"errors"`
	CompileRate0   Stat `json:"compileRate0"`
	FinalCompile   Stat `json:"finalCompileRate"`
	RepairRounds   Stat `json:"repairRounds"`
	Fidelity       Stat `json:"fidelity"`
	Hallucination  Stat `json:"hallucinationRate"`
	PilotAgreement Stat `json:"pilotAgreement"`
	PilotJudged    int  `json:"pilotJudged"`
	ToolCalls      Stat `json:"toolCallsPerCell"`
	// ByFamily and ByDifficulty hold final compile rate and fidelity per stratum.
	ByFamily     map[string]Stratum `json:"byFamily"`
	ByDifficulty map[string]Stratum `json:"byDifficulty"`
}

// Stratum is the per-stratum view of the headline metrics.
type Stratum struct {
	Cells        int     `json:"cells"`
	FinalCompile float64 `json:"finalCompileRate"`
	Fidelity     float64 `json:"fidelity"`
}

// AggregateCells summarises one arm's cells. Cells that ended in an Error are
// counted in Cells and Errors but measured nowhere else.
func AggregateCells(cells []CellResult) Aggregate {
	agg := Aggregate{ByFamily: map[string]Stratum{}, ByDifficulty: map[string]Stratum{}}
	bySample := map[int][]CellResult{}
	var measured []CellResult
	for _, c := range cells {
		agg.Cells++
		if c.Error != "" {
			agg.Errors++
			continue
		}
		measured = append(measured, c)
		bySample[c.Sample] = append(bySample[c.Sample], c)
	}
	samples := make([]int, 0, len(bySample))
	for s := range bySample {
		samples = append(samples, s)
	}
	sort.Ints(samples)
	var cr0, fcr, rounds, fid, hall, pilot, tools []float64
	for _, s := range samples {
		cs := bySample[s]
		cr0 = append(cr0, rate(cs, func(c CellResult) float64 { return b2f(c.CompiledRound0) }))
		fcr = append(fcr, rate(cs, func(c CellResult) float64 { return b2f(c.CompiledFinal) }))
		rounds = append(rounds, rate(cs, func(c CellResult) float64 { return float64(c.RepairRounds) }))
		fid = append(fid, rate(cs, func(c CellResult) float64 { return c.Fidelity }))
		hall = append(hall, rate(cs, func(c CellResult) float64 { return b2f(len(c.Hallucinated) > 0) }))
		tools = append(tools, rate(cs, func(c CellResult) float64 { return float64(c.ToolCalls) }))
		var judged []CellResult
		for _, c := range cs {
			if c.PilotAccepted != nil {
				judged = append(judged, c)
			}
		}
		agg.PilotJudged += len(judged)
		if len(judged) > 0 {
			pilot = append(pilot, rate(judged, func(c CellResult) float64 { return b2f(*c.PilotAccepted) }))
		}
	}
	agg.CompileRate0, agg.FinalCompile, agg.RepairRounds = NewStat(cr0), NewStat(fcr), NewStat(rounds)
	agg.Fidelity, agg.Hallucination, agg.ToolCalls = NewStat(fid), NewStat(hall), NewStat(tools)
	agg.PilotAgreement = NewStat(pilot)
	agg.ByFamily = strata(measured, func(c CellResult) string { return c.Family })
	agg.ByDifficulty = strata(measured, func(c CellResult) string { return c.Difficulty })
	return agg
}

func strata(cells []CellResult, key func(CellResult) string) map[string]Stratum {
	groups := map[string][]CellResult{}
	for _, c := range cells {
		groups[key(c)] = append(groups[key(c)], c)
	}
	out := map[string]Stratum{}
	for k, cs := range groups {
		out[k] = Stratum{
			Cells:        len(cs),
			FinalCompile: rate(cs, func(c CellResult) float64 { return b2f(c.CompiledFinal) }),
			Fidelity:     rate(cs, func(c CellResult) float64 { return c.Fidelity }),
		}
	}
	return out
}

func rate(cs []CellResult, f func(CellResult) float64) float64 {
	if len(cs) == 0 {
		return 0
	}
	sum := 0.0
	for _, c := range cs {
		sum += f(c)
	}
	return sum / float64(len(cs))
}

func b2f(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// Paired compares two arms over the (task, sample) observations both have.
type Paired struct {
	Metric string `json:"metric"`
	Pairs  int    `json:"pairs"`
	// MeanDiff is mean(b - a).
	MeanDiff float64 `json:"meanDiff"`
	// T and PT are the paired t statistic and its two-sided p-value.
	T  float64 `json:"t"`
	PT float64 `json:"pT"`
	// Wins, Losses and PSign are the sign test: cells where b beat a, where a
	// beat b, and the exact two-sided binomial p-value over the non-ties.
	Wins   int     `json:"wins"`
	Losses int     `json:"losses"`
	PSign  float64 `json:"pSign"`
}

// PairedMetrics are the per-cell metrics the arms are compared on.
// LowerIsBetter names the paired metrics where a smaller value is the better
// outcome, so a negative difference counts as a win for the second arm.
var LowerIsBetter = map[string]bool{"repairRounds": true}

var PairedMetrics = map[string]func(CellResult) float64{
	"compileRate0":     func(c CellResult) float64 { return b2f(c.CompiledRound0) },
	"finalCompileRate": func(c CellResult) float64 { return b2f(c.CompiledFinal) },
	"repairRounds":     func(c CellResult) float64 { return float64(c.RepairRounds) },
	"fidelity":         func(c CellResult) float64 { return c.Fidelity },
}

// ComparePaired pairs cells of arm a with cells of arm b by (task, sample).
func ComparePaired(a, b []CellResult) []Paired {
	index := map[string]CellResult{}
	for _, c := range a {
		index[c.Task+"/"+itoa(c.Sample)] = c
	}
	names := make([]string, 0, len(PairedMetrics))
	for n := range PairedMetrics {
		names = append(names, n)
	}
	sort.Strings(names)
	var out []Paired
	for _, name := range names {
		f := PairedMetrics[name]
		var diffs []float64
		p := Paired{Metric: name}
		for _, cb := range b {
			ca, ok := index[cb.Task+"/"+itoa(cb.Sample)]
			if !ok || ca.Error != "" || cb.Error != "" {
				continue
			}
			d := f(cb) - f(ca)
			diffs = append(diffs, d)
			better := d
			if LowerIsBetter[name] {
				better = -d
			}
			switch {
			case better > 0:
				p.Wins++
			case better < 0:
				p.Losses++
			}
		}
		p.Pairs = len(diffs)
		if p.Pairs > 0 {
			st := NewStat(diffs)
			p.MeanDiff = st.Mean
			if p.Pairs > 1 && st.Stdev > 0 {
				p.T = st.Mean / (st.Stdev / math.Sqrt(float64(p.Pairs)))
				p.PT = studentTwoSided(p.T, float64(p.Pairs-1))
			} else {
				p.PT = 1
			}
			p.PSign = signTest(p.Wins, p.Losses)
		}
		out = append(out, p)
	}
	return out
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for ; i > 0; i /= 10 {
		b = append([]byte{byte('0' + i%10)}, b...)
	}
	return string(b)
}

// signTest is the exact two-sided binomial test over the non-tied pairs.
func signTest(wins, losses int) float64 {
	n := wins + losses
	if n == 0 {
		return 1
	}
	k := wins
	if losses < wins {
		k = losses
	}
	p := 0.0
	for i := 0; i <= k; i++ {
		p += math.Exp(lchoose(n, i) - float64(n)*math.Ln2)
	}
	return math.Min(1, 2*p)
}

func lchoose(n, k int) float64 {
	a, _ := math.Lgamma(float64(n + 1))
	b, _ := math.Lgamma(float64(k + 1))
	c, _ := math.Lgamma(float64(n - k + 1))
	return a - b - c
}

// studentTwoSided is P(|T| >= |t|) for Student's t with nu degrees of freedom.
func studentTwoSided(t, nu float64) float64 {
	x := nu / (nu + t*t)
	return regIncBeta(nu/2, 0.5, x)
}

// regIncBeta is the regularized incomplete beta function I_x(a, b) by Lentz's
// continued fraction.
func regIncBeta(a, b, x float64) float64 {
	if x <= 0 {
		return 0
	}
	if x >= 1 {
		return 1
	}
	if x > (a+1)/(a+b+2) {
		return 1 - regIncBeta(b, a, 1-x)
	}
	la, _ := math.Lgamma(a + b)
	lb, _ := math.Lgamma(a)
	lc, _ := math.Lgamma(b)
	front := math.Exp(la-lb-lc+a*math.Log(x)+b*math.Log(1-x)) / a
	const tiny = 1e-30
	f, c, d := 1.0, 1.0, 0.0
	for m := 0; m <= 400; m++ {
		var num float64
		switch {
		case m == 0:
			num = 1
		case m%2 == 0:
			k := float64(m / 2)
			num = k * (b - k) * x / ((a + 2*k - 1) * (a + 2*k))
		default:
			k := float64((m - 1) / 2)
			num = -(a + k) * (a + b + k) * x / ((a + 2*k) * (a + 2*k + 1))
		}
		d = 1 + num*d
		if math.Abs(d) < tiny {
			d = tiny
		}
		d = 1 / d
		c = 1 + num/c
		if math.Abs(c) < tiny {
			c = tiny
		}
		cd := c * d
		f *= cd
		if math.Abs(1-cd) < 1e-12 {
			break
		}
	}
	return front * (f - 1)
}
