; OpenSysML SMT-LIB2 translation of constraint SafeWindow
; the runtime evaluator remains normative; solving is an optional extension
(set-logic QF_LIA)
; test::SafeWindow::level, declared at safe_window.sysml:8:3
(declare-const |test::SafeWindow::level| Int)
; required condition: level >= 0 — constraint SafeWindow, at safe_window.sysml:10:4
(assert (>= |test::SafeWindow::level| 0))
; required condition: not { level > 10; level < 20 } — constraint SafeWindow, at safe_window.sysml:13:4
(assert (not (and (> |test::SafeWindow::level| 10) (< |test::SafeWindow::level| 20))))
(check-sat)
