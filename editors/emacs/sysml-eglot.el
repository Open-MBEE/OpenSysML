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
;; The server is `sysml-lsp' on `exec-path'.

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

(add-to-list 'eglot-server-programs '((sysml-mode kerml-mode) "sysml-lsp"))

(provide 'sysml-eglot)

;;; sysml-eglot.el ends here
