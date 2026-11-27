;;; sysml-mode-test.el --- Tests for sysml-mode  -*- lexical-binding: t; -*-

;; Copyright The OpenSysML Authors
;; SPDX-License-Identifier: Apache-2.0

;;; Commentary:

;; Run with `make emacs-test' from the repository root.

;;; Code:

(require 'cl-lib)
(require 'ert)
(require 'ert-x)
(require 'imenu)
(require 'sysml-mode)

;; No test reads or writes the answers of whoever runs the suite.
(setq sysml-project-builds-file
      (expand-file-name "builds.eld" (make-temp-file "sysml-test" t)))

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

;;;; Running elements

(ert-deftest sysml-mode-run-names-the-check ()
  (dolist (case '(("action" . "-action") ("state" . "-state") ("calc" . "-calc")
                  ("constraint" . "-constraint") ("requirement" . "-requirement")))
    (let (started)
      (cl-letf (((symbol-function 'save-some-buffers) #'ignore)
                ((symbol-function 'compilation-start)
                 (lambda (command &rest _) (setq started (list command default-directory)))))
        (sysml-run-element (car case) "Demo::X" (list temporary-file-directory) "/bin/sysml"))
      (should (equal (car started)
                     (mapconcat #'shell-quote-argument
                                (list "/bin/sysml" (cdr case) "Demo::X" temporary-file-directory) " ")))
      (should (equal (cadr started) (file-name-as-directory temporary-file-directory))))))

(ert-deftest sysml-mode-run-refuses-what-sysml-cannot-run ()
  (let (started)
    (cl-letf (((symbol-function 'save-some-buffers) #'ignore)
              ((symbol-function 'compilation-start) (lambda (&rest _) (setq started t))))
      (dolist (case '(("part" "Demo::X" ("/ws") "/bin/sysml")
                      ("action" "" ("/ws") "/bin/sysml")
                      ("action" nil ("/ws") "/bin/sysml")
                      ("action" "Demo::X" nil "/bin/sysml")
                      ;; The tool was not found.
                      ("action" "Demo::X" ("/ws") nil)))
        (should-error (apply #'sysml-run-element case) :type 'user-error)))
    (should-not started)))

(ert-deftest sysml-mode-run-paths-are-every-folder-when-one-holds-the-file ()
  (let ((folders '("/ws/" "/other")))
    ;; The server indexes every root together, so a run loads them all.
    (should (equal (sysml-run-paths "/ws/sub/m.sysml" folders) folders))
    (should (equal (sysml-run-paths "/other/m.sysml" folders) folders))
    (should (equal (sysml-run-paths "/elsewhere/m.sysml" folders) '("/elsewhere/m.sysml")))
    ;; A folder holds what lies under it, not a sibling sharing its prefix.
    (should (equal (sysml-run-paths "/otherwise/m.sysml" folders) '("/otherwise/m.sysml")))
    (should (equal (sysml-run-paths "/ws/m.sysml" nil) '("/ws/m.sysml")))))

;;;; Project builds

(defmacro sysml-test--with-project-build (root &rest body)
  "Run BODY with ROOT bound to a new project holding `bin/sysml-lsp' and `bin/sysml'.
No answer is on record for it, and nothing else names a server or tool."
  (declare (indent 1))
  `(let* ((,root (file-name-as-directory (make-temp-file "proj" t)))
          (sysml-project-builds nil)
          (sysml-project-builds-file (expand-file-name "answers/builds.eld" ,root))
          (sysml--project-build-answers nil)
          (sysml-lsp-server-path nil)
          (exec-path nil))
     (unwind-protect
         (progn
           (make-directory (expand-file-name "bin" ,root))
           (dolist (f '("bin/sysml-lsp" "bin/sysml"))
             (write-region "" nil (expand-file-name f ,root))
             (set-file-modes (expand-file-name f ,root) #o755))
           ,@body)
       (delete-directory ,root t))))

(defmacro sysml-test--answering (answer asked saved &rest body)
  "Run BODY as an interactive session whose user answers ANSWER to a question.
ASKED is bound to the questions put, newest first.  SAVED is a function
returning the answers `sysml-project-builds-file' holds for later
sessions.  Nothing may be saved through Custom, whose file holds the
user's other settings."
  (declare (indent 3))
  `(let ((noninteractive nil) ,asked)
     (cl-letf (((symbol-function 'y-or-n-p)
                (lambda (prompt) (push prompt ,asked) ,answer))
               ((symbol-function 'customize-save-variable)
                (lambda (&rest _) (ert-fail "saved through Custom")))
               ((symbol-function 'custom-save-all)
                (lambda (&rest _) (ert-fail "saved through Custom"))))
       (cl-flet ((,saved () (sysml-test--saved-builds)))
         ,@body))))

(defun sysml-test--saved-builds ()
  "Return the answers `sysml-project-builds-file' holds, or nil when it is absent."
  (when (file-exists-p sysml-project-builds-file)
    (with-temp-buffer
      (insert-file-contents sysml-project-builds-file)
      (read (current-buffer)))))

(ert-deftest sysml-mode-asks-once-before-running-a-project-build ()
  (sysml-test--with-project-build root
    (sysml-test--answering t asked saved
      (should (equal (sysml--find-server root) (expand-file-name "bin/sysml-lsp" root)))
      (should (equal (sysml-find-tool nil root) (expand-file-name "bin/sysml" root)))
      (should (equal (sysml--find-server root) (expand-file-name "bin/sysml-lsp" root)))
      (should (= (length asked) 1))
      (should (string-search root (car asked)))
      (should (equal (saved) (list (cons root t))))
      (should-not sysml-project-builds))))

(ert-deftest sysml-mode-remembers-a-refused-project-build ()
  (sysml-test--with-project-build root
    (sysml-test--answering nil asked saved
      (should-not (sysml--find-server root))
      (should-not (sysml-find-tool nil root))
      (should-not (sysml--find-server root))
      (should (= (length asked) 1))
      (should (equal (saved) (list (cons root nil)))))))

(ert-deftest sysml-mode-keeps-an-answer-it-cannot-save-for-the-session ()
  (sysml-test--with-project-build root
    ;; A file stands where the directory of the answers would be made.
    (write-region "" nil (expand-file-name "answers" root))
    (sysml-test--answering t asked saved
      (should (sysml--find-server root))
      (should (sysml-find-tool nil root))
      (should (= (length asked) 1))
      (should-not (saved)))))

(ert-deftest sysml-mode-keeps-the-answers-another-session-saved ()
  (sysml-test--with-project-build root
    (sysml-test--answering t asked saved
      (should (sysml--find-server root))
      ;; Another Emacs answers for its own checkouts after this one read the file.
      (make-directory (file-name-directory sysml-project-builds-file) t)
      (write-region (prin1-to-string (list '("/other/a/" . t) (cons root t) '("/other/b/" . nil)))
                    nil sysml-project-builds-file)
      (let ((second (file-name-as-directory (make-temp-file "proj" t))))
        (unwind-protect
            (progn
              (make-directory (expand-file-name "bin" second))
              (write-region "" nil (expand-file-name "bin/sysml-lsp" second))
              (set-file-modes (expand-file-name "bin/sysml-lsp" second) #o755)
              (should (sysml--find-server second))
              (should (= (length asked) 2))
              (should (equal (sort (saved) (lambda (a b) (string< (car a) (car b))))
                             (sort (list '("/other/a/" . t) '("/other/b/" . nil)
                                         (cons root t) (cons second t))
                                   (lambda (a b) (string< (car a) (car b)))))))
          (delete-directory second t))))))

(ert-deftest sysml-mode-sees-an-answer-another-session-changed ()
  (sysml-test--with-project-build root
    (sysml-test--answering t asked saved
      (should (sysml--find-server root))
      (write-region (prin1-to-string (list (cons root nil))) nil sysml-project-builds-file)
      (should-not (sysml--find-server root))
      (should (= (length asked) 1)))))

(ert-deftest sysml-mode-leaves-answers-it-cannot-read-alone ()
  (sysml-test--with-project-build root
    (make-directory (file-name-directory sysml-project-builds-file) t)
    (write-region "((\"/other/a/\" . t" nil sysml-project-builds-file)
    (sysml-test--answering t asked saved
      (should (sysml--find-server root))
      (should (sysml-find-tool nil root))
      (should (= (length asked) 1))
      (should (equal (with-temp-buffer
                       (insert-file-contents sysml-project-builds-file)
                       (buffer-string))
                     "((\"/other/a/\" . t")))))

(ert-deftest sysml-mode-puts-the-option-before-a-saved-answer ()
  (sysml-test--with-project-build root
    (dolist (answer '(t nil))
      (make-directory (file-name-directory sysml-project-builds-file) t)
      (write-region (prin1-to-string (list (cons root (not answer)))) nil
                    sysml-project-builds-file)
      (let ((sysml-project-builds (list (cons root answer))))
        (sysml-test--answering (not answer) asked saved
          (should (eq (and (sysml--find-server root) t) answer))
          (should-not asked)
          (should (equal (saved) (list (cons root (not answer))))))))))

(ert-deftest sysml-mode-refuses-a-project-build-it-cannot-ask-about ()
  (sysml-test--with-project-build root
    (let ((noninteractive t))
      (cl-letf (((symbol-function 'y-or-n-p)
                 (lambda (&rest _) (ert-fail "asked in a batch session"))))
        (should-not (sysml--find-server root))
        (should-not (sysml-find-tool nil root))
        (should-not sysml-project-builds)))))

(ert-deftest sysml-mode-runs-a-project-build-allowed-earlier ()
  (sysml-test--with-project-build root
    ;; The answer is on record under the root however the caller spells it.
    (let ((sysml-project-builds (list (cons root t))))
      (should (equal (sysml--find-server (directory-file-name root))
                     (expand-file-name "bin/sysml-lsp" root)))
      (should (equal (sysml-find-tool nil root) (expand-file-name "bin/sysml" root))))))

(ert-deftest sysml-mode-asks-nothing-without-a-project-build ()
  (let ((root (file-name-as-directory (make-temp-file "proj" t)))
        (sysml-project-builds nil)
        (sysml-lsp-server-path nil)
        (exec-path nil))
    (unwind-protect
        (sysml-test--answering t asked saved
          (should-not (sysml--find-server root))
          (should-not (sysml-find-tool nil root))
          (should-not asked)
          (should-not (saved)))
      (delete-directory root t))))

(ert-deftest sysml-mode-asks-nothing-when-a-server-is-configured ()
  (sysml-test--with-project-build root
    (let ((sysml-lsp-server-path (expand-file-name "bin/sysml-lsp" root)))
      (sysml-test--answering t asked saved
        (should (equal (sysml--find-server root) sysml-lsp-server-path))
        (should-not asked)
        (should-not (saved))))))

(ert-deftest sysml-mode-keeps-answers-apart-by-checkout ()
  ;; A checkout on another host is another checkout, under its full name.
  (let* ((root "/work/model/")
         (remote (concat "/ssh:host:" root))
         (sysml-project-builds (list (cons remote nil) (cons root t))))
    (should (sysml--project-build-allowed-p root))
    (should-not (sysml--project-build-allowed-p remote))))

(ert-deftest sysml-mode-finds-an-answer-recorded-without-the-trailing-slash ()
  (sysml-test--with-project-build root
    (dolist (answer '(t nil))
      (let ((sysml-project-builds (list (cons (directory-file-name root) answer))))
        (sysml-test--answering (not answer) asked saved
          (should (equal (sysml--find-server root)
                         (and answer (expand-file-name "bin/sysml-lsp" root))))
          (should (equal (sysml-find-tool nil root)
                         (and answer (expand-file-name "bin/sysml" root))))
          (should-not asked)
          (should-not (saved)))))))

(ert-deftest sysml-mode-finds-a-remote-answer-recorded-without-the-trailing-slash ()
  (dolist (answer '(t nil))
    (let ((sysml-project-builds (list (cons "/ssh:host:/work/model" answer)
                                      (cons "/work/model" (not answer)))))
      (sysml-test--answering (not answer) asked saved
        (should (eq (sysml--project-build-allowed-p "/ssh:host:/work/model/") answer))
        (should (eq (sysml--project-build-allowed-p "/work/model/") (not answer)))
        (should-not asked)
        (should-not (saved))))))

(ert-deftest sysml-mode-names-the-build-it-may-not-run-when-no-server-is-found ()
  (sysml-test--with-project-build root
    ;; Never asked about in a batch session, and refused earlier.
    (dolist (sysml-project-builds (list nil (list (cons root nil))))
      (let* ((noninteractive t)
             (message (cadr (should-error (sysml--server-command root) :type 'user-error))))
        (should (string-search (expand-file-name "bin/" root) message))
        (should (string-search "not allowed" message))
        (should (string-search "sysml-project-builds" message))))))

(ert-deftest sysml-mode-names-no-build-when-the-project-holds-none ()
  (let ((root (file-name-as-directory (make-temp-file "proj" t)))
        (sysml-project-builds nil)
        (sysml-lsp-server-path nil)
        (exec-path nil)
        (noninteractive t))
    (unwind-protect
        (let ((message (cadr (should-error (sysml--server-command root) :type 'user-error))))
          (should-not (string-search "sysml-project-builds" message))
          (should (string-search "make build" message)))
      (delete-directory root t))))

(ert-deftest sysml-mode-says-when-it-passes-over-a-build-for-exec-path ()
  (sysml-test--with-project-build root
    (let* ((elsewhere (file-name-as-directory (make-temp-file "path" t)))
           (exec-path (list elsewhere))
           (noninteractive t))
      (unwind-protect
          (progn
            (dolist (f '("sysml-lsp" "sysml"))
              (write-region "" nil (expand-file-name f elsewhere))
              (set-file-modes (expand-file-name f elsewhere) #o755))
            (dolist (case `((,(lambda () (sysml--find-server root)) "sysml-lsp")
                            (,(lambda () (sysml-find-tool nil root)) "sysml")))
              (ert-with-message-capture said
                (should (equal (funcall (car case)) (expand-file-name (cadr case) elsewhere)))
                (should (string-search (expand-file-name (concat "bin/" (cadr case)) root) said))
                (should (string-search (expand-file-name (cadr case) elsewhere) said))
                (should (string-search "sysml-project-builds" said))
                (should (= (length (split-string said "\n" t)) 1)))
              ;; Nothing was passed over when the build may be run.
              (let ((sysml-project-builds (list (cons root t))))
                (ert-with-message-capture said
                  (should (equal (funcall (car case))
                                 (expand-file-name (concat "bin/" (cadr case)) root)))
                  (should (string-empty-p said))))))
        (delete-directory elsewhere t)))))

(ert-deftest sysml-mode-says-nothing-of-builds-when-the-project-holds-none ()
  (let* ((root (file-name-as-directory (make-temp-file "proj" t)))
         (sysml-project-builds nil)
         (sysml-lsp-server-path nil)
         (exec-path (list root))
         (noninteractive t))
    (unwind-protect
        (progn
          (write-region "" nil (expand-file-name "sysml-lsp" root))
          (set-file-modes (expand-file-name "sysml-lsp" root) #o755)
          (ert-with-message-capture said
            (should (sysml--find-server root))
            (should (string-empty-p said))))
      (delete-directory root t))))

(ert-deftest sysml-mode-names-the-build-it-may-not-run-when-no-tool-is-found ()
  (sysml-test--with-project-build root
    (let* ((noninteractive t)
           (file (expand-file-name "m.sysml" root))
           (message (cadr (should-error
                           (sysml-run-element "action" "P::a" (list file)
                                              (sysml-find-tool nil root) root)
                           :type 'user-error))))
      (should (string-search (expand-file-name "bin/" root) message))
      (should (string-search "not allowed" message))
      (should (string-search "sysml-project-builds" message)))))

(ert-deftest sysml-mode-finds-the-tool-beside-the-server-then-in-the-project ()
  (let* ((root (file-name-as-directory (make-temp-file "root" t)))
         (beside (file-name-as-directory (make-temp-file "bin" t)))
         (sysml-project-builds (list (cons root t)))
         (exec-path nil))
    (unwind-protect
        (progn
          (make-directory (expand-file-name "bin" root))
          (should-not (sysml-find-tool (expand-file-name "sysml-lsp" beside) root))
          (dolist (f (list (expand-file-name "bin/sysml" root)))
            (write-region "" nil f)
            (set-file-modes f #o755))
          (should (equal (sysml-find-tool (expand-file-name "sysml-lsp" beside) root)
                         (expand-file-name "bin/sysml" root)))
          (should (equal (sysml-find-tool nil root) (expand-file-name "bin/sysml" root)))
          (write-region "" nil (expand-file-name "sysml" beside))
          (set-file-modes (expand-file-name "sysml" beside) #o755)
          (should (equal (sysml-find-tool (expand-file-name "sysml-lsp" beside) root)
                         (expand-file-name "sysml" beside)))
          (let ((exec-path (list beside)))
            (should (equal (sysml-find-tool nil nil) (expand-file-name "sysml" beside)))))
      (delete-directory root t)
      (delete-directory beside t))))

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
