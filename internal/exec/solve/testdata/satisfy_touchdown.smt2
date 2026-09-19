; OpenSysML SMT-LIB2 translation of satisfaction satisfy touchdown by lander
; the runtime evaluator remains normative; solving is an optional extension
(set-logic QF_LRA)
; test::TouchdownRequirement::craft.verticalSpeed, declared at satisfy_touchdown.sysml:7:3
(declare-const |test::TouchdownRequirement::craft.verticalSpeed| Real)
; test::TouchdownRequirement::maxVerticalSpeed, declared at satisfy_touchdown.sysml:12:3
(declare-const |test::TouchdownRequirement::maxVerticalSpeed| Real)
; required condition: craft.verticalSpeed <= maxVerticalSpeed — satisfaction satisfy touchdown by lander, declared by TouchdownRequirement, at satisfy_touchdown.sysml:14:4
(assert (<= |test::TouchdownRequirement::craft.verticalSpeed| |test::TouchdownRequirement::maxVerticalSpeed|))
(check-sat)
