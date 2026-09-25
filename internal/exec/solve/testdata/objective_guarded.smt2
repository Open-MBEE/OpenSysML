; OpenSysML SMT-LIB2 translation of analysis GuardedRatio
; the runtime evaluator remains normative; solving is an optional extension
; objectives are optimized lexicographically, in the order the analysis declares them:
; each is optimized within what the ones before it already settled
(set-option :opt.priority lex)
(set-logic AUFNIRA)
; test::GuardedRatio::parts, declared at objectives.sysml:141:3
(declare-const |test::GuardedRatio::parts| Int)
; test::GuardedRatio::total, declared at objectives.sysml:140:3
(declare-const |test::GuardedRatio::total| Int)
; well-definedness: total / parts >= 2 — analysis GuardedRatio, at objectives.sysml:143:4
(assert (distinct |test::GuardedRatio::parts| 0))
; required condition: total / parts >= 2 — analysis GuardedRatio, at objectives.sysml:143:4
(assert (>= (/ (to_real |test::GuardedRatio::total|) (to_real |test::GuardedRatio::parts|)) 2.0))
; required condition: parts >= 1 — analysis GuardedRatio, at objectives.sysml:146:4
(assert (>= |test::GuardedRatio::parts| 1))
; required condition: parts <= 4 — analysis GuardedRatio, at objectives.sysml:149:4
(assert (<= |test::GuardedRatio::parts| 4))
; required condition: total <= 40 — analysis GuardedRatio, at objectives.sysml:152:4
(assert (<= |test::GuardedRatio::total| 40))
; maximize: mostParts = parts, at objectives.sysml:156:23
(maximize |test::GuardedRatio::parts|)
(check-sat)
