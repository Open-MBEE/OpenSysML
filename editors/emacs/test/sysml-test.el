;;; sysml-test.el --- Helpers the OpenSysML Emacs suites share  -*- lexical-binding: t; -*-

;; Copyright The OpenSysML Authors
;; SPDX-License-Identifier: Apache-2.0

;;; Commentary:

;; The live tests need `bin/sysml-lsp' (run `make build'); they skip
;; without it, and fail instead when OPENSYSML_REQUIRE_EMACS_LSP is set, as
;; CI sets it.  `sysml-test--with-session' starts the server under the
;; client deps.el installed, through that client's own test file.

;;; Code:

(require 'ert)
(require 'ert-x)

(defvar sysml-test-client)

(defconst sysml-test--repo
  (expand-file-name "../../../" (file-name-directory (or load-file-name buffer-file-name)))
  "Root of the OpenSysML checkout.")

(defconst sysml-test--server
  (expand-file-name "bin/sysml-lsp" sysml-test--repo)
  "The server `make build' writes.")

(defun sysml-test--require-server ()
  "Skip the test unless the built server exists, or fail when it is required."
  (unless (file-executable-p sysml-test--server)
    (if (getenv "OPENSYSML_REQUIRE_EMACS_LSP")
        (ert-fail (format "%s is missing; run `make build'" sysml-test--server))
      (ert-skip (format "%s is missing; run `make build'" sysml-test--server)))))

(defun sysml-test--wait (pred &optional seconds)
  "Process input until PRED holds, failing after SECONDS (default 20).
Idle timers run too, as they do while a user waits: Eglot sends a
buffer's changes from one."
  (let ((deadline (+ (float-time) (or seconds 20))))
    (while (not (funcall pred))
      (when (> (float-time) deadline)
        (ert-fail "timed out waiting for the language server"))
      (accept-process-output nil 0.1)
      (ert-run-idle-timers))))

(defun sysml-test--fixture (text)
  "Write TEXT to a .sysml file in a fresh directory and return its name.
Each test gets its own directory, so its own workspace."
  (let ((file (expand-file-name "model.sysml" (make-temp-file "sysml-lsp-test" t))))
    (write-region text nil file)
    file))

(defmacro sysml-test--with-session (file &rest body)
  "Visit FILE under a live `sysml-lsp' session and run BODY in its buffer.
The session is the client's the suites run under, `sysml-test-client'."
  (declare (indent 1))
  (pcase sysml-test-client
    ('eglot `(sysml-eglot-test--with-session ,file ,@body))
    ('lsp-mode `(sysml-lsp-test--with-session ,file ,@body))))

(provide 'sysml-test)

;;; sysml-test.el ends here
