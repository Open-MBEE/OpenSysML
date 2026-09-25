; OpenSysML SMT-LIB2 translation of constraint finishMatchesNesting
; the runtime evaluator remains normative; solving is an optional extension
; no SMT-LIB logic covers algebraic datatypes (declare-datatypes), so the logic set below is ALL, which the SMT-LIB logic list does not define
(set-logic ALL)
; |test::Finish| of test::Finish
(declare-datatypes ((|test::Finish| 0)) (((|test::Finish::polished|) (|test::Finish::brushed|))))
; |test::ringFamily::nesting| of test::ringFamily::nesting
(declare-datatypes ((|test::ringFamily::nesting| 0)) (((|test::ringFamily::nesting::nestingTrue|) (|test::ringFamily::nesting::nestingFalse|))))
; test::Ring::finish, declared at ring_variants.sysml:14:3
(declare-const |test::Ring::finish| |test::Finish|)
; test::ringFamily::nesting, declared at ring_variants.sysml:18:3
(declare-const |test::ringFamily::nesting| |test::ringFamily::nesting|)
; required condition: nesting == nesting::nestingTrue implies finish == Finish::polished — constraint finishMatchesNesting, at ring_variants.sysml:24:4
(assert (=> (= |test::ringFamily::nesting| |test::ringFamily::nesting::nestingTrue|) (= |test::Ring::finish| |test::Finish::polished|)))
(check-sat)
