;;; deps.el --- Test dependencies for the OpenSysML Emacs package  -*- lexical-binding: t; -*-

;; Copyright The OpenSysML Authors
;; SPDX-License-Identifier: Apache-2.0

;;; Commentary:

;; Loaded before the test suites by `make emacs-test': installs lsp-mode
;; from MELPA into the directory OPENSYSML_EMACS_DEPS names, once, and
;; activates it, so the suites run under `emacs -Q' without a personal
;; configuration.

;;; Code:

(require 'package)

(let ((dir (getenv "OPENSYSML_EMACS_DEPS")))
  (unless dir
    (error "OPENSYSML_EMACS_DEPS must name the directory to install test dependencies in"))
  (setq package-user-dir (expand-file-name dir)
        package-archives '(("gnu" . "https://elpa.gnu.org/packages/")
                           ("melpa" . "https://melpa.org/packages/"))))

(package-initialize)
(unless (package-installed-p 'lsp-mode)
  (package-refresh-contents)
  (package-install 'lsp-mode))

;;; deps.el ends here
