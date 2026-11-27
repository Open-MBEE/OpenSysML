;;; deps.el --- Test dependencies for the OpenSysML Emacs package  -*- lexical-binding: t; -*-

;; Copyright The OpenSysML Authors
;; SPDX-License-Identifier: Apache-2.0

;;; Commentary:

;; Loaded before the test suites by `make emacs-test': installs the client
;; OPENSYSML_EMACS_CLIENT names, lsp-mode from MELPA or Eglot from GNU
;; ELPA, into a directory of its own under OPENSYSML_EMACS_DEPS, once, and
;; activates it, so the suites run under `emacs -Q' without a personal
;; configuration, and without the other client.

;;; Code:

(require 'package)

(defconst sysml-test-client
  (intern (or (getenv "OPENSYSML_EMACS_CLIENT") ""))
  "The client the suites run under: `lsp-mode' or `eglot'.")

(let ((dir (getenv "OPENSYSML_EMACS_DEPS")))
  (unless dir
    (error "OPENSYSML_EMACS_DEPS must name the directory to install test dependencies in"))
  (unless (memq sysml-test-client '(lsp-mode eglot))
    (error "OPENSYSML_EMACS_CLIENT must be lsp-mode or eglot"))
  (setq package-user-dir (expand-file-name (symbol-name sysml-test-client) dir)
        package-archives '(("gnu" . "https://elpa.gnu.org/packages/")
                           ("melpa" . "https://melpa.org/packages/"))))

(package-initialize)
;; Emacs 29 and 30 ship an Eglot older than the 1.20 `sysml-eglot' needs.
(let ((package-install-upgrade-built-in t))
  (unless (package-installed-p sysml-test-client (and (eq sysml-test-client 'eglot) '(1 20)))
    (package-refresh-contents)
    (package-install sysml-test-client)))

;;; deps.el ends here
