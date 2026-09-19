; OpenSysML SMT-LIB2 translation of requirement TouchdownRequirement
; the runtime evaluator remains normative; solving is an optional extension
(set-logic QF_LRA)
; test::TouchdownRequirement::actualVerticalSpeed in base units of L·T^-1, declared at touchdown.sysml:9:3
(declare-const |test::TouchdownRequirement::actualVerticalSpeed| Real)
; test::TouchdownRequirement::maxVerticalSpeed in base units of L·T^-1, declared at touchdown.sysml:10:3
(declare-const |test::TouchdownRequirement::maxVerticalSpeed| Real)
; assumed condition: maxVerticalSpeed <= 5.4 [km/h] — requirement TouchdownRequirement, at touchdown.sysml:12:4
(assert (<= |test::TouchdownRequirement::maxVerticalSpeed| 1.5))
; required condition: actualVerticalSpeed <= maxVerticalSpeed — requirement TouchdownRequirement, at touchdown.sysml:15:4
(assert (<= |test::TouchdownRequirement::actualVerticalSpeed| |test::TouchdownRequirement::maxVerticalSpeed|))
(check-sat)
