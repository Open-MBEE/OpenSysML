; OpenSysML SMT-LIB2 translation of constraint CratesPerPallet
; the runtime evaluator remains normative; solving is an optional extension
(set-logic AUFLIRA)
; test::CratesPerPallet::crates, declared at logic_selection.sysml:9:3
(declare-const |test::CratesPerPallet::crates| Int)
; required condition: crates / 12 >= 3 — constraint CratesPerPallet, at logic_selection.sysml:11:4
(assert (>= (/ (to_real |test::CratesPerPallet::crates|) 12.0) 3.0))
(check-sat)
