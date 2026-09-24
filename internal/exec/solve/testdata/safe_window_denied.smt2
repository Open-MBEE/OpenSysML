; OpenSysML SMT-LIB2 translation of constraint safeWindow
; the runtime evaluator remains normative; solving is an optional extension
; the element asserts that its required conditions do not all hold
(set-logic QF_LIA)
; test::SafeWindow::level, declared at safe_window.sysml:8:3
(declare-const |test::SafeWindow::level| Int)
; denied conditions: not (level >= 0 and not { level > 10; level < 20 }) — constraint safeWindow, at safe_window.sysml:19:3
(assert (not (and (>= |test::SafeWindow::level| 0) (not (and (> |test::SafeWindow::level| 10) (< |test::SafeWindow::level| 20))))))
(check-sat)
