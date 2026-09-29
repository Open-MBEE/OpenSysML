; OpenSysML SMT-LIB2 translation of constraint SafeWindow
; the runtime evaluator remains normative; solving is an optional extension
(set-logic QF_LIA)
; test::SafeWindow::level, declared at safe_window.sysml:8:3
(declare-const |test::SafeWindow::level| Int)
; violated conditions: not (level >= 0 and not { level > 10; level < 20 }) — constraint SafeWindow, at safe_window.sysml:7:2
(assert (not (and (>= |test::SafeWindow::level| 0) (not (and (> |test::SafeWindow::level| 10) (< |test::SafeWindow::level| 20))))))
(check-sat)
