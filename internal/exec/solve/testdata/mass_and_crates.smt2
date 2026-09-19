; OpenSysML SMT-LIB2 translation of constraint MassAndCrates
; the runtime evaluator remains normative; solving is an optional extension
(set-logic AUFLIRA)
; test::MassAndCrates::crates, declared at logic_selection.sysml:25:3
(declare-const |test::MassAndCrates::crates| Int)
; test::MassAndCrates::mass, declared at logic_selection.sysml:26:3
(declare-const |test::MassAndCrates::mass| Real)
; required condition: mass <= 4.5 — constraint MassAndCrates, at logic_selection.sysml:28:4
(assert (<= |test::MassAndCrates::mass| 4.5))
; required condition: crates >= 2 — constraint MassAndCrates, at logic_selection.sysml:29:4
(assert (>= |test::MassAndCrates::crates| 2))
(check-sat)
