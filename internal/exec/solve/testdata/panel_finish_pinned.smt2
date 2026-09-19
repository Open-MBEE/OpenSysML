; OpenSysML SMT-LIB2 translation of constraint polishedIsWide
; the runtime evaluator remains normative; solving is an optional extension
; no SMT-LIB logic covers algebraic datatypes (declare-datatypes), so the logic set below is ALL, which the SMT-LIB logic list does not define
(set-logic ALL)
; |test::Finish| of test::Finish
(declare-datatypes ((|test::Finish| 0)) (((|test::Finish::polished|) (|test::Finish::brushed|))))
; test::Panel::finish, declared at panel_pins.sysml:18:3
(declare-const |test::Panel::finish| |test::Finish|)
; test::Panel::width, declared at panel_pins.sysml:16:3
(declare-const |test::Panel::width| Int)
; fixed value: test::Panel::finish == Finish::polished, declared by the model — feature test::Panel::finish, declared by finish, at panel_pins.sysml:18:3
(assert (= |test::Panel::finish| |test::Finish::polished|))
; fixed value: test::Panel::width == 4, declared by the model — feature test::Panel::width, declared by width, at panel_pins.sysml:16:3
(assert (= |test::Panel::width| 4))
; required condition: finish == Finish::polished implies width >= 6 — constraint polishedIsWide, at panel_pins.sysml:28:4
(assert (=> (= |test::Panel::finish| |test::Finish::polished|) (>= |test::Panel::width| 6)))
(check-sat)
