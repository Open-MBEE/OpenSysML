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
;;   server serves through `opensysml/stdlibContent';
;; - `sysml-eglot-run-element', which runs an executable element as the
;;   server's Run and Evaluate code lenses do.  Eglot shows no code lenses.
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

;;;; Running elements

;; A Run or Evaluate lens's command is the client's to carry out: the server
;; serves no `workspace/executeCommand', so `eglot-execute' cannot run it.

(defconst sysml-eglot--run-element-command "opensysml.runElement"
  "The client command a Run or Evaluate code lens invokes.")

(defun sysml-eglot--run-lenses ()
  "Return the current buffer's Run and Evaluate code lenses."
  (let ((server (eglot-current-server)))
    (unless server
      (user-error "No language server manages this buffer; start it with M-x eglot"))
    (unless (eglot-server-capable :codeLensProvider)
      (user-error "This language server offers no code lenses"))
    ;; The lenses are matched to point, so the server must have the buffer's text.
    (eglot--signal-textDocument/didChange)
    (seq-filter (lambda (lens)
                  (equal (plist-get (plist-get lens :command) :command)
                         sysml-eglot--run-element-command))
                (jsonrpc-request server :textDocument/codeLens
                                 (list :textDocument (list :uri (eglot-path-to-uri buffer-file-name)))))))

(defun sysml-eglot--lens-at-point (lenses)
  "Return the one of LENSES on the name at point, or nil.
A lens's range is the name of the element it runs."
  (seq-find (lambda (lens)
              (pcase-let ((`(,beg . ,end) (eglot-range-region (plist-get lens :range))))
                (<= beg (point) end)))
            lenses))

(defun sysml-eglot--read-lens (lenses)
  "Read one of LENSES from the minibuffer by its title."
  (let ((titles (mapcar (lambda (lens) (plist-get (plist-get lens :command) :title)) lenses)))
    (nth (seq-position titles (completing-read "Run: " titles nil t)) lenses)))

(defun sysml-eglot--folders (server)
  "Return the file names of SERVER's workspace folders."
  (mapcar (lambda (folder) (eglot-uri-to-path (plist-get folder :uri)))
          (eglot-workspace-folders server)))

(defun sysml-eglot--find-tool (folders)
  "Return the `sysml' executable to run elements with, or nil.
The language server is `sysml-lsp' on the variable `exec-path', and the
project root the first of the workspace FOLDERS; see `sysml-find-tool'."
  (sysml-find-tool (executable-find "sysml-lsp") (car folders)))

(defun sysml-eglot-run-element (&optional choose)
  "Run or evaluate an executable element of the current buffer with `sysml'.
The elements are those the server offers a Run or Evaluate code lens
on: each action, state, calc, constraint and requirement `sysml' can
run by name, in a file on disk.  The one whose name point is on is
run; elsewhere, or with the prefix argument CHOOSE, one is read from
the minibuffer.  See `sysml-run-element' for how it is run."
  (interactive "P")
  (let* ((lenses (sysml-eglot--run-lenses))
         (lens (cond ((null lenses)
                      (user-error "The server offers nothing to run in this buffer"))
                     ((and (not choose) (sysml-eglot--lens-at-point lenses)))
                     (t (sysml-eglot--read-lens lenses))))
         (args (car (append (plist-get (plist-get lens :command) :arguments) nil)))
         (uri (plist-get args :uri))
         (folders (sysml-eglot--folders (eglot-current-server))))
    (sysml-run-element (plist-get args :kind)
                       (plist-get args :element)
                       (and uri (sysml-run-paths (eglot-uri-to-path uri) folders))
                       (sysml-eglot--find-tool folders))))

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

(add-to-list 'eglot-server-programs '((sysml-mode kerml-mode) "sysml-lsp"))

(provide 'sysml-eglot)

;;; sysml-eglot.el ends here
