; OpenSysML SMT-LIB2 translation of constraint speedIsBounded
; the runtime evaluator remains normative; solving is an optional extension
(set-logic QF_LRA)
; test::Panel::maxSpeed in base units of L·T^-1, declared at panel_pins.sysml:19:3
(declare-const |test::Panel::maxSpeed| Real)
; fixed value: test::Panel::maxSpeed == 5.4 [km/h], declared by the model — feature test::Panel::maxSpeed, declared by maxSpeed, at panel_pins.sysml:19:3
(assert (= |test::Panel::maxSpeed| 1.5))
; required condition: maxSpeed <= 2.0 [m/s] — constraint speedIsBounded, at panel_pins.sysml:32:4
(assert (<= |test::Panel::maxSpeed| 2.0))
(check-sat)
