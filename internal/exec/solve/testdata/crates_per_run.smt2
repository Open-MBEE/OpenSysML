; OpenSysML SMT-LIB2 translation of constraint CratesPerRun
; the runtime evaluator remains normative; solving is an optional extension
(set-logic AUFNIRA)
; test::CratesPerRun::crates, declared at logic_selection.sysml:16:3
(declare-const |test::CratesPerRun::crates| Int)
; test::CratesPerRun::runs, declared at logic_selection.sysml:17:3
(declare-const |test::CratesPerRun::runs| Int)
; well-definedness: crates / runs >= 3 — constraint CratesPerRun, at logic_selection.sysml:20:4
(assert (distinct |test::CratesPerRun::runs| 0))
; required condition: runs > 0 — constraint CratesPerRun, at logic_selection.sysml:19:4
(assert (> |test::CratesPerRun::runs| 0))
; required condition: crates / runs >= 3 — constraint CratesPerRun, at logic_selection.sysml:20:4
(assert (>= (/ (to_real |test::CratesPerRun::crates|) (to_real |test::CratesPerRun::runs|)) 3.0))
(check-sat)
