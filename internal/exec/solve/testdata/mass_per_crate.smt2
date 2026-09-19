; OpenSysML SMT-LIB2 translation of constraint MassPerCrate
; the runtime evaluator remains normative; solving is an optional extension
(set-logic AUFNIRA)
; test::MassPerCrate::crates, declared at logic_selection.sysml:34:3
(declare-const |test::MassPerCrate::crates| Int)
; test::MassPerCrate::mass, declared at logic_selection.sysml:35:3
(declare-const |test::MassPerCrate::mass| Real)
; well-definedness: mass / crates <= 4.5 — constraint MassPerCrate, at logic_selection.sysml:38:4
(assert (distinct (to_real |test::MassPerCrate::crates|) 0.0))
; required condition: crates > 0 — constraint MassPerCrate, at logic_selection.sysml:37:4
(assert (> |test::MassPerCrate::crates| 0))
; required condition: mass / crates <= 4.5 — constraint MassPerCrate, at logic_selection.sysml:38:4
(assert (<= (/ |test::MassPerCrate::mass| (to_real |test::MassPerCrate::crates|)) 4.5))
(check-sat)
