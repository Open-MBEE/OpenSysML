; OpenSysML SMT-LIB2 translation of analysis MassBudget
; the runtime evaluator remains normative; solving is an optional extension
; objectives are optimized lexicographically, in the order the analysis declares them:
; each is optimized within what the ones before it already settled
(set-option :opt.priority lex)
(set-logic QF_LRA)
; test::MassBudget::mass in base units of M, declared at objectives.sysml:13:3
(declare-const |test::MassBudget::mass| Real)
; required condition: mass >= 10 [kg] — analysis MassBudget, at objectives.sysml:15:4
(assert (>= |test::MassBudget::mass| 10000.0))
; required condition: mass <= 90 [kg] — analysis MassBudget, at objectives.sysml:18:4
(assert (<= |test::MassBudget::mass| 90000.0))
; minimize: lightest = mass in gram, at objectives.sysml:22:23
(minimize |test::MassBudget::mass|)
(check-sat)
