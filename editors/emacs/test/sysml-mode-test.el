;;; sysml-mode-test.el --- Tests for sysml-mode  -*- lexical-binding: t; -*-

;; Copyright The OpenSysML Authors
;; SPDX-License-Identifier: Apache-2.0

;;; Commentary:

;; Run with `make emacs-test' from the repository root.

;;; Code:

(require 'ert)
(require 'imenu)
(require 'sysml-mode)

(defconst sysml-test--dir
  (file-name-directory (or load-file-name buffer-file-name))
  "Directory holding this file.")

(defconst sysml-test--repo
  (expand-file-name "../../../" sysml-test--dir)
  "Root of the OpenSysML checkout.")

(defmacro sysml-test--with-buffer (mode text &rest body)
  "Run BODY in a fontified MODE buffer holding TEXT."
  (declare (indent 2))
  `(with-temp-buffer
     (insert ,text)
     (funcall ,mode)
     (font-lock-ensure)
     (goto-char (point-min))
     ,@body))

(defun sysml-test--face-of (needle &optional nth)
  "Return the face at the start of the NTH (default 1st) occurrence of NEEDLE."
  (goto-char (point-min))
  (dotimes (_ (or nth 1))
    (let ((case-fold-search nil))
    (unless (search-forward needle nil t)
      (error "%S not found" needle))))
  (let ((face (get-text-property (match-beginning 0) 'face)))
    (if (listp face) (car face) face)))

(defun sysml-test--faces (mode text expected)
  "Assert that in MODE, TEXT gives each (NEEDLE FACE [NTH]) of EXPECTED."
  (sysml-test--with-buffer mode text
    (dolist (case expected)
      (should (equal (list (car case) (sysml-test--face-of (car case) (nth 2 case)))
                     (list (car case) (nth 1 case)))))))

;;;; Keywords

(ert-deftest sysml-mode-every-category-has-a-face ()
  (dolist (group sysml-mode-keyword-groups)
    (should (alist-get (car group) sysml--category-faces))))

(ert-deftest sysml-mode-every-keyword-is-highlighted ()
  (dolist (group sysml-mode-keyword-groups)
    (let ((face (alist-get (car group) sysml--category-faces)))
      (dolist (word (cdr group))
        (sysml-test--with-buffer #'sysml-mode (concat "x " word " y;")
          (should (equal (list word (sysml-test--face-of (concat " " word " ")))
                         (list word nil)))
          (goto-char 3)
          (should (equal (list word (get-text-property (point) 'face))
                         (list word face))))))))

(ert-deftest sysml-mode-contextual-words-are-per-language ()
  (sysml-test--faces #'sysml-mode "chain c; var v;"
                     '(("chain" sysml-contextual-keyword-face)
                       ("var" nil)))
  (sysml-test--faces #'kerml-mode "var feature v;"
                     '(("var" sysml-contextual-keyword-face))))

(ert-deftest sysml-mode-keywords-match-whole-words ()
  (sysml-test--faces #'sysml-mode "attribute partial; attribute subsetsX;"
                     '(("partial" nil) ("subsetsX" nil))))

;;;; Names and types

(ert-deftest sysml-mode-declaration-names ()
  (sysml-test--faces #'sysml-mode
                     "standard library package Lib { part def 'Quoted Def'; alias A for B; }"
                     '(("Lib" sysml-declaration-name-face)
                       ("'Quoted Def'" sysml-declaration-name-face)
                       ("A " sysml-declaration-name-face)
                       ("library" sysml-modifier-keyword-face))))

(ert-deftest sysml-mode-type-references ()
  (sysml-test--faces #'sysml-mode
                     "part e : Engine; part p ::> Base; part q :>> A::B::c; attribute x :> 'Q R';"
                     '(("Engine" sysml-type-reference-face)
                       (":" sysml-operator-face)
                       ("Base" sysml-type-reference-face)
                       ("::>" sysml-operator-face)
                       ("A::" sysml-namespace-face)
                       ("B::" sysml-namespace-face)
                       ("c;" sysml-type-reference-face)
                       ("'Q R'" sysml-type-reference-face))))

(ert-deftest sysml-mode-names-in-comments-are-text ()
  (sysml-test--faces #'sysml-mode "// part def Foo : Bar\npart p;"
                     '(("Foo" font-lock-comment-face)
                       ("Bar" font-lock-comment-face))))

(ert-deftest sysml-mode-assignment-is-not-a-typing ()
  (sysml-test--faces #'sysml-mode "attribute x := y;"
                     '((":=" sysml-operator-face) ("y" nil))))

(ert-deftest sysml-mode-qualified-names ()
  (sysml-test--faces #'sysml-mode "private import ScalarValues::*;"
                     '(("ScalarValues" sysml-namespace-face)
                       ("*" sysml-operator-face))))

;;;; Literals

(ert-deftest sysml-mode-numbers ()
  (sysml-test--faces #'sysml-mode "attribute a = 32.0 + .5e+3 + 1e9; attribute r = 1..2;"
                     '(("32.0" sysml-number-face)
                       (".5e+3" sysml-number-face)
                       ("1e9" sysml-number-face)
                       ("1." sysml-number-face)
                       (".." sysml-operator-face)
                       ("2;" sysml-number-face))))

(ert-deftest sysml-mode-strings-and-escapes ()
  (sysml-test--faces #'sysml-mode "attribute s = \"a \\\" // b\"; part 'x\\'y';"
                     '(("\"a" font-lock-string-face)
                       ("\\\"" sysml-escape-face)
                       ("// b" font-lock-string-face)
                       ("'x" sysml-quoted-name-face))))

(ert-deftest sysml-mode-keywords-inside-quoted-names-are-not-highlighted ()
  (sysml-test--faces #'sysml-mode "part 'part def';"
                     '(("part def" sysml-quoted-name-face))))

;;;; Comments

(ert-deftest sysml-mode-comment-forms ()
  (sysml-test--with-buffer #'sysml-mode
      "// line part\n/* block part */\n//* note\n   part */\npart p;\n"
    (dolist (needle '("line part" "block part" "note" "part */"))
      (goto-char (point-min))
      (search-forward needle)
      (should (nth 4 (syntax-ppss (match-beginning 0)))))
    (should (eq (sysml-test--face-of "//*") 'font-lock-comment-delimiter-face))
    (goto-char (point-min))
    (search-forward "part p")
    (should-not (nth 8 (syntax-ppss (match-beginning 0))))
    (should (eq (sysml-test--face-of "part p") 'sysml-declaration-keyword-face))))

(ert-deftest sysml-mode-note-opener-inside-a-string-is-text ()
  (sysml-test--with-buffer #'sysml-mode "attribute s = \"//* x\"; part p;"
    (goto-char (point-min))
    (search-forward "part p")
    (should-not (nth 8 (syntax-ppss (match-beginning 0))))))

(ert-deftest sysml-mode-comment-dwim-uses-line-notes ()
  (with-temp-buffer
    (sysml-mode)
    (insert "part p;")
    (comment-region (point-min) (point-max))
    (should (equal (buffer-string) "// part p;"))))

;;;; Indentation

(ert-deftest sysml-mode-indents-like-the-formatter ()
  "The formatter's continuation fixture must indent to its golden."
  (let ((in (expand-file-name "internal/syntax/format/testdata/continuation.sysml" sysml-test--repo))
        (golden (expand-file-name "internal/syntax/format/testdata/continuation.golden.sysml"
                                  sysml-test--repo)))
    (with-temp-buffer
      (insert-file-contents in)
      (sysml-mode)
      (indent-region (point-min) (point-max))
      (should (equal (buffer-string)
                     (with-temp-buffer (insert-file-contents golden) (buffer-string)))))))

(ert-deftest sysml-mode-indents-nesting-and-closing-braces ()
  (with-temp-buffer
    (insert "package P {\npart def A {\nattribute x;\n}\n}\n")
    (sysml-mode)
    (indent-region (point-min) (point-max))
    (should (equal (buffer-string)
                   "package P {\n    part def A {\n        attribute x;\n    }\n}\n"))))

(ert-deftest sysml-mode-leaves-comment-text-alone ()
  (with-temp-buffer
    (insert "package P {\n/* a\n      keep\n */\n}\n")
    (sysml-mode)
    (indent-region (point-min) (point-max))
    (should (equal (buffer-string) "package P {\n    /* a\n      keep\n */\n}\n"))))

(ert-deftest sysml-mode-respects-indent-offset ()
  (with-temp-buffer
    (insert "package P {\npart p;\n}\n")
    (sysml-mode)
    (setq-local sysml-indent-offset 2)
    (indent-region (point-min) (point-max))
    (should (equal (buffer-string) "package P {\n  part p;\n}\n"))))

;;;; Files and imenu

(ert-deftest sysml-mode-file-associations ()
  (should (eq (assoc-default "m.sysml" auto-mode-alist #'string-match) 'sysml-mode))
  (should (eq (assoc-default "m.kerml" auto-mode-alist #'string-match) 'kerml-mode)))

(ert-deftest sysml-mode-imenu ()
  (with-temp-buffer
    (insert "package P {\n    abstract part def Vehicle;\n    action def 'Drive It';\n}\n")
    (sysml-mode)
    (let ((index (funcall imenu-create-index-function)))
      (should (assoc "P" (cdr (assoc "Packages" index))))
      (should (assoc "Vehicle" (cdr (assoc "Definitions" index))))
      (should (assoc "'Drive It'" (cdr (assoc "Definitions" index)))))))

(ert-deftest sysml-mode-fontifies-every-repository-model ()
  "Every example model fontifies without error."
  (dolist (file (directory-files-recursively
                 (expand-file-name "examples" sysml-test--repo) "\\.\\(sysml\\|kerml\\)\\'"))
    (with-temp-buffer
      (insert-file-contents file)
      (if (string-suffix-p ".kerml" file) (kerml-mode) (sysml-mode))
      (font-lock-ensure))))

(provide 'sysml-mode-test)

;;; sysml-mode-test.el ends here
