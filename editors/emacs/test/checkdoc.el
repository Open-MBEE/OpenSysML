;;; checkdoc.el --- Fail on checkdoc findings  -*- lexical-binding: t; -*-

;; Copyright The OpenSysML Authors
;; SPDX-License-Identifier: Apache-2.0

;;; Commentary:

;; Run by `make emacs-compile': checks the documentation conventions of
;; each file named after it on the command line, and exits non-zero when
;; any is broken.

;;; Code:

(require 'checkdoc)

(let ((failed nil))
  (dolist (file command-line-args-left)
    (with-current-buffer (find-file-noselect (expand-file-name file))
      (with-current-buffer (get-buffer-create checkdoc-diagnostic-buffer)
        (let ((inhibit-read-only t)) (erase-buffer)))
      (checkdoc-current-buffer t)
      ;; The report heads each file with a `*** FILE: ...' line and separates
      ;; files with a page break; only findings, listed under the heading,
      ;; fail the check.
      (let ((report (with-current-buffer checkdoc-diagnostic-buffer
                      (let ((inhibit-read-only t))
                        (goto-char (point-min))
                        (flush-lines "^\\*\\*\\* ")
                        (string-trim (buffer-string) "[ \t\n\f]+" "[ \t\n\f]+")))))
        (unless (string-empty-p report)
          (setq failed t)
          (message "%s" report)))))
  (setq command-line-args-left nil)
  (kill-emacs (if failed 1 0)))

;;; checkdoc.el ends here
