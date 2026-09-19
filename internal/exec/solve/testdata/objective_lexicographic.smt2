; OpenSysML SMT-LIB2 translation of analysis CostThenMargin
; the runtime evaluator remains normative; solving is an optional extension
; objectives are optimized lexicographically, in the order the analysis declares them:
; each is optimized within what the ones before it already settled
(set-option :opt.priority lex)
(set-logic QF_LIA)
; test::CostThenMargin::cost, declared at objectives.sysml:89:3
(declare-const |test::CostThenMargin::cost| Int)
; test::CostThenMargin::margin, declared at objectives.sysml:90:3
(declare-const |test::CostThenMargin::margin| Int)
; required condition: cost >= 3 — analysis CostThenMargin, at objectives.sysml:92:4
(assert (>= |test::CostThenMargin::cost| 3))
; required condition: cost <= 9 — analysis CostThenMargin, at objectives.sysml:95:4
(assert (<= |test::CostThenMargin::cost| 9))
; required condition: margin >= 0 — analysis CostThenMargin, at objectives.sysml:98:4
(assert (>= |test::CostThenMargin::margin| 0))
; required condition: margin <= cost * 2 — analysis CostThenMargin, at objectives.sysml:101:4
(assert (<= |test::CostThenMargin::margin| (* |test::CostThenMargin::cost| 2)))
; minimize: cheapest = cost, at objectives.sysml:105:23
(minimize |test::CostThenMargin::cost|)
; maximize: widestMargin = margin, at objectives.sysml:109:23
(maximize |test::CostThenMargin::margin|)
(check-sat)
