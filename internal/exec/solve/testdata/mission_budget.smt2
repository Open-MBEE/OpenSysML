; OpenSysML SMT-LIB2 translation of constraint BudgetConstraint
; the runtime evaluator remains normative; solving is an optional extension
; no SMT-LIB logic covers the strings theory, so the logic set below is ALL, which the SMT-LIB logic list does not define
(set-logic ALL)
; test::BudgetConstraint::crewCount, declared at mission_budget.sysml:7:3
(declare-const |test::BudgetConstraint::crewCount| Int)
; test::BudgetConstraint::phase, declared at mission_budget.sysml:9:3
(declare-const |test::BudgetConstraint::phase| String)
; test::BudgetConstraint::redundant, declared at mission_budget.sysml:8:3
(declare-const |test::BudgetConstraint::redundant| Bool)
; declared domain: a Natural is not negative — declaration test::BudgetConstraint::crewCount, declared by crewCount, at mission_budget.sysml:7:3
(assert (>= |test::BudgetConstraint::crewCount| 0))
; required condition: operator if <= 4 — constraint BudgetConstraint, at mission_budget.sysml:11:4
(assert (<= (ite |test::BudgetConstraint::redundant| (+ |test::BudgetConstraint::crewCount| 1) |test::BudgetConstraint::crewCount|) 4))
; required condition: phase == "descent" implies redundant — constraint BudgetConstraint, at mission_budget.sysml:14:4
(assert (=> (= |test::BudgetConstraint::phase| "descent") |test::BudgetConstraint::redundant|))
(check-sat)
