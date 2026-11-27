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
;; The server is found as under lsp-mode: the `sysml-lsp-server-path'
;; setting, else `bin/sysml-lsp' in the project root, else `sysml-lsp' on
;; `exec-path', and it is passed `sysml-lsp-server-args'.  Its settings are
;; set in `eglot-workspace-configuration' under `:sysml', for instance
;; (:sysml (:strictConformance t)).

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

(defun sysml-eglot--xref-library-match (make-match name uri range)
  "Call MAKE-MATCH with NAME, URI and RANGE unless URI is a library document.
A `sysml-stdlib:' URI names no file, so Eglot would locate the match in
an empty buffer; it is located in the document's library buffer instead."
  (if (sysml-library-uri-p uri)
      (let ((buffer (sysml-eglot-stdlib-buffer uri)))
        (pcase-let ((`(,beg . ,end) (with-current-buffer buffer (eglot-range-region range))))
          (sysml-library-xref-item buffer beg end)))
    (funcall make-match name uri range)))

(advice-add 'eglot--xref-make-match :around #'sysml-eglot--xref-library-match)

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
  "Read one of LENSES from the minibuffer by its element's qualified name.
A title holds the element's short name, which two packages may share;
no two lenses share the qualified name, the one given to `sysml'."
  (let ((elements (mapcar (lambda (lens)
                            (plist-get (seq-first (plist-get (plist-get lens :command) :arguments))
                                       :element))
                          lenses)))
    (nth (seq-position elements (completing-read "Run or evaluate: " elements nil t))
         lenses)))

(defun sysml-eglot--folders (server)
  "Return the file names of SERVER's workspace folders."
  (mapcar (lambda (folder) (eglot-uri-to-path (plist-get folder :uri)))
          (eglot-workspace-folders server)))

(defun sysml-eglot--running-server (server root)
  "Return the executable of SERVER on this machine, or nil.
A relative name is one in ROOT, the project root Eglot started SERVER in.
It is nil for a server on another host or reached over a socket."
  (when-let* (((not (file-remote-p default-directory)))
              (command (process-command (jsonrpc--process server)))
              ((consp command)))
    (let ((default-directory (or root default-directory)))
      (executable-find (car command)))))

(defun sysml-eglot--find-tool (server folders)
  "Return the `sysml' executable to run elements with, or nil.
The project root is the first of the workspace FOLDERS.  The language
server is the one SERVER runs, which may be one entered at the prompt
of \\[eglot], and otherwise the one found from the project root; see
`sysml-find-tool'."
  (let ((root (car folders)))
    (sysml-find-tool (or (sysml-eglot--running-server server root)
                         (sysml--find-server root))
                     root)))

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
         (server (eglot-current-server))
         (folders (sysml-eglot--folders server)))
    (sysml-run-element (plist-get args :kind)
                       (plist-get args :element)
                       (and uri (sysml-run-paths (eglot-uri-to-path uri) folders))
                       (sysml-eglot--find-tool server folders))))

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
When none is found and INTERACTIVE is non-nil, a command is returned for
\\[eglot] to prompt with rather than an error signaled: the bare
executable name when `sysml-lsp-server-path' is unset, and the configured
path under a prefix argument, which always prompts.  A configured path
that is not executable is otherwise an error, never passed over for
another server."
  (let* ((root (if project (project-root project) (sysml--project-root)))
         (server (sysml--find-server root t)))
    (cond
     (server (cons (file-local-name server) sysml-lsp-server-args))
     ((and interactive (not sysml-lsp-server-path))
      (cons sysml--server-executable sysml-lsp-server-args))
     ((and interactive current-prefix-arg)
      (cons sysml-lsp-server-path sysml-lsp-server-args))
     (t (sysml--server-command root t)))))

(add-to-list 'eglot-server-programs
             '((sysml-mode kerml-mode) . sysml-eglot--server-command))

(provide 'sysml-eglot)

;;; sysml-eglot.el ends here
