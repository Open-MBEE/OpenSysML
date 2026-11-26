;;; sysml-eglot.el --- Eglot support for the OpenSysML language server  -*- lexical-binding: t; -*-

;; Copyright The OpenSysML Authors
;; SPDX-License-Identifier: Apache-2.0

;; Author: The OpenSysML Authors
;; URL: https://github.com/Open-MBEE/OpenSysML
;; Version: 0.1.0
;; Package-Requires: ((emacs "29.1") (eglot "1.20"))
;; Keywords: languages

;;; Commentary:

;; Registers `sysml-lsp' with Eglot for `sysml-mode' and `kerml-mode'.
;; Eglot serves diagnostics, hover, go-to-definition, references, symbols,
;; completion, formatting, rename, quick fixes and semantic tokens itself;
;; this file adds what OpenSysML's own methods need:
;;
;; - `sysml-eglot-render-changed-functions', run on the server's
;;   `opensysml/renderChanged' notification;
;; - read-only buffers of the bundled library documents, whose text the
;;   server serves through `opensysml/stdlibContent'.
;;
;; The server is found as under lsp-mode: the `sysml-lsp-server-path'
;; setting, else `bin/sysml-lsp' in the project root, else `sysml-lsp' on
;; `exec-path', and it is passed `sysml-lsp-server-args'.

;;; Code:

(require 'eglot)
(require 'sysml-mode)

(defvar sysml-eglot-render-changed-functions nil
  "Functions called when the server reports a document's rendering changed.
Each is called with the document's file name and the version the server
analysed.  The server sends `opensysml/renderChanged' after every analysis
of an open document, whether or not anything shows a view of it, so the
client always handles it and only these functions act on it.")

(cl-defmethod eglot-handle-notification
  (_server (_method (eql opensysml/renderChanged)) &key textDocument version)
  "Run `sysml-eglot-render-changed-functions' for TEXTDOCUMENT at VERSION."
  (run-hook-with-args 'sysml-eglot-render-changed-functions
                      (eglot-uri-to-path (plist-get textDocument :uri))
                      version))

;;;; Bundled library documents

(defconst sysml-eglot--stdlib-content-method "opensysml/stdlibContent"
  "The request serving the text of a bundled library document.")

(defun sysml-eglot-stdlib-buffer (uri)
  "Return a read-only buffer showing the bundled library document URI.
Its text is asked of the current buffer's server; see
`sysml-library-buffer'."
  (let ((server (eglot-current-server)))
    (sysml-library-buffer
     uri server
     (lambda ()
       (plist-get (jsonrpc-request server sysml-eglot--stdlib-content-method (list :uri uri))
                  :text)))))

;;;; Formatting

(defun sysml-eglot--format-with-mode-offset (format &rest args)
  "Call FORMAT with ARGS, sending `sysml-indent-offset' as the indent width.
Eglot sends `tab-width' as a formatting request's tabSize, the width the
server indents by, while `sysml-mode' indents by `sysml-indent-offset'."
  (let ((tab-width (if (derived-mode-p 'sysml-base-mode) sysml-indent-offset tab-width)))
    (apply format args)))

(advice-add 'eglot-format :around #'sysml-eglot--format-with-mode-offset)

;;;; Semantic tokens

(defun sysml-eglot--keep-mode-faces ()
  "Leave what `sysml-mode' colours to it in an Eglot-managed model buffer.
The server colours every token, keywords included.  The mode already
colours the lexical ones, and keywords by category, which the server's
single \"keyword\" type would flatten, so those types get no face here.
Modifiers are left off for the same reason: the server marks an
`abstract' declaration's name, not the keyword."
  (when (derived-mode-p 'sysml-base-mode)
    (setq-local eglot-semantic-token-types
                (seq-difference eglot-semantic-token-types
                                '("keyword" "comment" "string" "number"))
                eglot-semantic-token-modifiers
                (seq-difference eglot-semantic-token-modifiers
                                '("declaration" "definition" "readonly" "abstract")))))

(add-hook 'eglot-managed-mode-hook #'sysml-eglot--keep-mode-faces)

;;;; Registration

(defun sysml-eglot--server-command (&optional interactive project)
  "Return the command to start the language server with under Eglot.
A build is looked for in the root of PROJECT, the project Eglot starts
the server for, or of the current buffer's when Eglot passes none.  The
executable is named as its own host names it, since Eglot starts it there.
When `sysml-lsp-server-path' is unset, none is found and INTERACTIVE is
non-nil, the bare executable name is returned, which has \\[eglot] prompt
for the program to run."
  (let ((root (if project (project-root project) (sysml--project-root))))
    (if (and interactive
             (not sysml-lsp-server-path)
             (not (sysml--find-server root t)))
        (cons sysml--server-executable sysml-lsp-server-args)
      (pcase-let ((`(,server . ,args) (sysml--server-command root t)))
        (cons (file-local-name server) args)))))

(add-to-list 'eglot-server-programs
             '((sysml-mode kerml-mode) . sysml-eglot--server-command))

(provide 'sysml-eglot)

;;; sysml-eglot.el ends here
