; OpenSysML SMT-LIB2 translation of analysis WheelChoice
; the runtime evaluator remains normative; solving is an optional extension
; objectives are optimized lexicographically, in the order the analysis declares them:
; each is optimized within what the ones before it already settled
(set-option :opt.priority lex)
; no SMT-LIB logic covers algebraic datatypes (declare-datatypes), so the logic set below is ALL, which the SMT-LIB logic list does not define
(set-logic ALL)
; |test::WheelChoice::wheel::rim| of test::WheelChoice::wheel::rim
(declare-datatypes ((|test::WheelChoice::wheel::rim| 0)) (((|test::WheelChoice::wheel::rim::steel|) (|test::WheelChoice::wheel::rim::carbon|))))
; test::WheelChoice::wheel.rim, declared at objectives.sysml:123:4
(declare-const |test::WheelChoice::wheel.rim| |test::WheelChoice::wheel::rim|)
; required condition: wheel::rim == wheel::rim::steel or wheel::rim == wheel::rim::carbon — analysis WheelChoice, at objectives.sysml:129:4
(assert (or (= |test::WheelChoice::wheel.rim| |test::WheelChoice::wheel::rim::steel|) (= |test::WheelChoice::wheel.rim| |test::WheelChoice::wheel::rim::carbon|)))
; minimize: lightestRim = operator if, at objectives.sysml:133:23
(minimize (ite (= |test::WheelChoice::wheel.rim| |test::WheelChoice::wheel::rim::carbon|) 4 9))
(check-sat)
