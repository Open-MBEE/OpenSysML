;;; sysml-eglot-test.el --- Tests for sysml-eglot  -*- lexical-binding: t; -*-

;; Copyright The OpenSysML Authors
;; SPDX-License-Identifier: Apache-2.0

;;; Commentary:

;; Needs Eglot 1.20 or later on `load-path'.  The live tests drive
;; `bin/sysml-lsp'; see sysml-test.el.

;;; Code:

(require 'ert)
(require 'sysml-eglot)
(require 'sysml-test)

;;;; Registration

(ert-deftest sysml-eglot-registers-the-server ()
  (should (eq (alist-get '(sysml-mode kerml-mode) eglot-server-programs nil nil #'equal)
              #'sysml-eglot--server-command)))

;;;; Server resolution

(defmacro sysml-eglot-test--with-project-build (root exe &rest body)
  "Run BODY with ROOT bound to a fresh project and EXE to its `bin/sysml-lsp'."
  (declare (indent 2))
  `(let* ((,root (file-name-as-directory (make-temp-file "proj" t)))
          (,exe (expand-file-name "bin/sysml-lsp" ,root)))
     (unwind-protect
         (progn
           (make-directory (file-name-directory ,exe))
           (write-region "" nil ,exe)
           (set-file-modes ,exe #o755)
           ,@body)
       (delete-directory ,root t))))

(ert-deftest sysml-eglot-prefers-the-configured-path ()
  (sysml-eglot-test--with-project-build root _exe
    (let* ((configured (make-temp-file "sysml-lsp"))
           (sysml-lsp-server-path configured)
           (sysml-lsp-server-args nil))
      (unwind-protect
          (progn
            (set-file-modes configured #o755)
            (should (equal (sysml-eglot--server-command nil (cons 'transient root))
                           (list configured))))
        (delete-file configured)))))

(ert-deftest sysml-eglot-refuses-a-configured-path-that-is-not-executable ()
  (let ((sysml-lsp-server-path "/nonexistent/sysml-lsp"))
    (should-error (sysml-eglot--server-command) :type 'user-error)))

(ert-deftest sysml-eglot-finds-the-project-build ()
  (sysml-eglot-test--with-project-build root exe
    (let ((sysml-lsp-server-path nil)
          (sysml-lsp-server-args nil)
          (exec-path nil))
      (should (equal (sysml-eglot--server-command nil (cons 'transient root))
                     (list exe))))))

(ert-deftest sysml-eglot-finds-the-build-in-the-buffer-project ()
  (sysml-eglot-test--with-project-build root exe
    (let ((sysml-lsp-server-path nil)
          (sysml-lsp-server-args nil)
          (exec-path nil)
          (default-directory root)
          (project-find-functions (list (lambda (dir) (cons 'transient dir)))))
      (should (equal (sysml-eglot--server-command) (list exe))))))

(ert-deftest sysml-eglot-falls-back-to-exec-path ()
  (sysml-eglot-test--with-project-build root exe
    (let* ((empty (file-name-as-directory (make-temp-file "empty" t)))
           (sysml-lsp-server-path nil)
           (sysml-lsp-server-args nil)
           (exec-path (list (file-name-directory exe))))
      (unwind-protect
          (should (equal (sysml-eglot--server-command nil (cons 'transient empty))
                         (list exe)))
        (delete-directory empty t)))))

(ert-deftest sysml-eglot-passes-server-args ()
  (sysml-eglot-test--with-project-build root exe
    (let ((sysml-lsp-server-path nil)
          (sysml-lsp-server-args '("-strict")))
      (should (equal (sysml-eglot--server-command nil (cons 'transient root))
                     (list exe "-strict"))))))

(ert-deftest sysml-eglot-starts-the-server-it-finds ()
  (sysml-test--require-server)
  (let* ((file (sysml-test--fixture "package P;\n"))
         (buffer (find-file-noselect file))
         (sysml-lsp-server-path sysml-test--server)
         (sysml-lsp-server-args '("-strict"))
         (exec-path nil))
    (unwind-protect
        (with-current-buffer buffer
          (apply #'eglot (eglot--guess-contact))
          (sysml-test--wait #'eglot-current-server)
          (should (equal (process-command (jsonrpc--process (eglot-current-server)))
                         (list sysml-test--server "-strict"))))
      (let ((server (with-current-buffer buffer
                      (set-buffer-modified-p nil)
                      (eglot-current-server))))
        (kill-buffer buffer)
        (when server
          (eglot-shutdown server)))
      (delete-directory (file-name-directory file) t))))

(ert-deftest sysml-eglot-passes-render-changed-on ()
  (let* (seen
         (sysml-eglot-render-changed-functions
          (list (lambda (file version) (setq seen (list file version))))))
    (eglot-handle-notification nil 'opensysml/renderChanged
                               :textDocument '(:uri "file:///tmp/m.sysml") :version 8)
    (should (equal seen '("/tmp/m.sysml" 8)))))

(ert-deftest sysml-eglot-leaves-lexical-tokens-to-the-mode ()
  (with-temp-buffer
    (sysml-mode)
    (sysml-eglot--keep-mode-faces)
    (should (local-variable-p 'eglot-semantic-token-types))
    (dolist (type '("keyword" "comment" "string" "number"))
      (should-not (member type eglot-semantic-token-types)))
    (should (member "variable" eglot-semantic-token-types))
    (dolist (modifier '("declaration" "definition" "readonly" "abstract"))
      (should-not (member modifier eglot-semantic-token-modifiers))))
  (with-temp-buffer
    (sysml-eglot--keep-mode-faces)
    (should-not (local-variable-p 'eglot-semantic-token-types))))

;;;; Live server

(defmacro sysml-eglot-test--with-session (file &rest body)
  "Visit FILE under a live `sysml-lsp' Eglot session and run BODY in its buffer."
  (declare (indent 1))
  `(progn
     (sysml-test--require-server)
     (let* ((file ,file)
            (buffer (find-file-noselect file)))
       (unwind-protect
           (with-current-buffer buffer
             (eglot '(sysml-mode kerml-mode) `(transient . ,default-directory) 'eglot-lsp-server
                    (list sysml-test--server) '("sysml" "kerml"))
             (sysml-test--wait #'eglot-current-server)
             ,@body)
         ;; Close the document while the server still runs, then stop it, so
         ;; the next test does not join a server that is shutting down.
         (let ((server (with-current-buffer buffer
                         (set-buffer-modified-p nil)
                         (eglot-current-server))))
           (kill-buffer buffer)
           (when server
             (eglot-shutdown server)))
         (delete-directory (file-name-directory file) t)))))

(defun sysml-eglot-test--definition ()
  "Return the locations the server gives for the definition of the name at point."
  (jsonrpc-request (eglot-current-server) :textDocument/definition
                   (list :textDocument (list :uri (eglot-path-to-uri buffer-file-name))
                         :position (list :line (1- (line-number-at-pos))
                                         :character (current-column)))))

(ert-deftest sysml-eglot-live-advertises-rendering ()
  (sysml-eglot-test--with-session (sysml-test--fixture "part def A;\n")
    (should (eglot-server-capable :experimental :openSysmlRender))))

(ert-deftest sysml-eglot-live-publishes-diagnostics ()
  (sysml-eglot-test--with-session (sysml-test--fixture "part def A {\n")
    ;; Flymake starts after a command, and a batch Emacs runs none.
    (flymake-start)
    (sysml-test--wait #'flymake-diagnostics)))

(ert-deftest sysml-eglot-live-goes-to-definition ()
  (sysml-eglot-test--with-session
      (sysml-test--fixture "package P {\n    part def Engine;\n    part e : Engine;\n}\n")
    (goto-char (point-min))
    (search-forward ": Engine")
    (backward-char 2)
    (let ((location (xref-item-location
                     (car (xref-backend-definitions 'eglot (xref-backend-identifier-at-point 'eglot))))))
      (should (= (xref-location-line location) 2)))))

(ert-deftest sysml-eglot-live-shows-a-library-declaration ()
  (sysml-eglot-test--with-session
      (sysml-test--fixture "package P {\n    private import ScalarValues::*;\n    attribute x : Integer;\n}\n")
    (goto-char (point-min))
    (search-forward ": Integer")
    (backward-char 2)
    (let* ((location (aref (sysml-eglot-test--definition) 0))
           (uri (plist-get location :uri))
           (buffer (progn
                     (should (sysml-library-uri-p uri))
                     (sysml-eglot-stdlib-buffer uri))))
      (unwind-protect
          (with-current-buffer buffer
            (should buffer-read-only)
            (should (derived-mode-p 'kerml-mode))
            (goto-char (car (eglot-range-region (plist-get location :range))))
            (should (string-match-p "datatype Integer"
                                    (buffer-substring (line-beginning-position) (line-end-position)))))
        (kill-buffer buffer)))))

(ert-deftest sysml-eglot-live-formats-with-the-mode-offset ()
  (sysml-eglot-test--with-session (sysml-test--fixture "package P {\npart def A;\n}\n")
    (setq-local sysml-indent-offset 3)
    (eglot-format-buffer)
    (should (equal (buffer-string) "package P {\n   part def A;\n}\n"))))

(ert-deftest sysml-eglot-live-reports-each-change ()
  (let* (versions
         (sysml-eglot-render-changed-functions
          (list (lambda (_file version) (push version versions)))))
    (sysml-eglot-test--with-session (sysml-test--fixture "package P {\n    part def A;\n}\n")
      (sysml-test--wait (lambda () versions))
      (let ((opened (car versions)))
        (goto-char (point-max))
        (insert "part def B;\n")
        (sysml-test--wait (lambda () (> (car versions) opened)))))))

(ert-deftest sysml-eglot-live-semantic-tokens-keep-keyword-faces ()
  (sysml-eglot-test--with-session
      (sysml-test--fixture "package P {\n    part def Engine;\n    part e : Engine;\n}\n")
    (should eglot-semantic-tokens-mode)
    (goto-char (point-min))
    (search-forward "part e")
    (let ((keyword (match-beginning 0))
          (name (1- (point))))
      ;; The server's token for `e' shows its answer has been drawn.
      (sysml-test--wait (lambda ()
                          (font-lock-ensure)
                          (get-text-property name 'face)))
      (should (eq (get-text-property keyword 'face) 'sysml-declaration-keyword-face)))))

(provide 'sysml-eglot-test)

;;; sysml-eglot-test.el ends here
