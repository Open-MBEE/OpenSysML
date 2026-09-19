; OpenSysML SMT-LIB2 translation of constraint fits
; the runtime evaluator remains normative; solving is an optional extension
(set-logic QF_LIA)
; test::Panel::height, declared at panel_pins.sysml:17:3
(declare-const |test::Panel::height| Int)
; test::Panel::width, declared at panel_pins.sysml:16:3
(declare-const |test::Panel::width| Int)
; fixed value: test::Panel::width == 4, declared by the model — feature test::Panel::width, declared by width, at panel_pins.sysml:16:3
(assert (= |test::Panel::width| 4))
; required condition: width + height <= 10 — constraint fits, at panel_pins.sysml:24:4
(assert (<= (+ |test::Panel::width| |test::Panel::height|) 10))
(check-sat)
