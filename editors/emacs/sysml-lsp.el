;;; sysml-lsp.el --- lsp-mode client for the OpenSysML language server  -*- lexical-binding: t; -*-

;; Copyright The OpenSysML Authors
;; SPDX-License-Identifier: Apache-2.0

;; Author: The OpenSysML Authors
;; URL: https://github.com/Open-MBEE/OpenSysML
;; Version: 0.1.0
;; Package-Requires: ((emacs "29.1") (lsp-mode "9.0"))
;; Keywords: languages

;;; Commentary:

;; Registers `sysml-lsp' with lsp-mode for `sysml-mode' and `kerml-mode':
;; diagnostics, hover, go-to-definition, references, symbols, completion,
;; formatting, rename, quick fixes and semantic tokens.
;;
;; The server is found as the VS Code extension finds it: the
;; `sysml-lsp-server-path' setting, else `bin/sysml-lsp' in the project
;; root (an OpenSysML checkout that ran `make build'), else `sysml-lsp' on
;; `exec-path'.

;;; Code:

(require 'lsp-mode)
(require 'sysml-mode)

(defgroup sysml-lsp nil
  "The OpenSysML language server, through lsp-mode."
  :group 'sysml
  :prefix "sysml-lsp-"
  :link '(url-link "https://github.com/Open-MBEE/OpenSysML"))

(defconst sysml-lsp--executable
  (if (eq system-type 'windows-nt) "sysml-lsp.exe" "sysml-lsp")
  "File name of the language server.")

(defcustom sysml-lsp-server-path nil
  "Absolute path to the `sysml-lsp' executable.
When nil, `bin/sysml-lsp' in the project root is used if it exists, then
`sysml-lsp' on the variable `exec-path'."
  :type '(choice (const :tag "Find it" nil) file)
  :risky t)

(defcustom sysml-lsp-server-args nil
  "Extra arguments passed to the language server."
  :type '(repeat string)
  :risky t)

(lsp-defcustom sysml-lsp-strict-conformance nil
  "Judge documents as conforming SysML v2.
When non-nil, notation only OpenSysML accepts is reported as an error
instead of a warning.  Changing it re-publishes every open document's
diagnostics."
  :type 'boolean
  :safe #'booleanp
  :lsp-path "sysml.strictConformance")

(defcustom sysml-lsp-semantic-tokens t
  "Highlight names by what they resolve to, using the server's semantic tokens.
Keywords, comments, strings and numbers keep the faces `sysml-mode'
gives them; the server only adds faces to names.  This turns on
`lsp-semantic-tokens-enable' in SysML and KerML buffers alone."
  :type 'boolean)

(defvar sysml-lsp-render-changed-functions nil
  "Functions called when the server reports a document's rendering changed.
Each is called with the document's file name and the version the server
analysed.  The server sends `opensysml/renderChanged' after every analysis
of an open document, whether or not anything shows a view of it, so the
client always handles it and only these functions act on it.")

(defun sysml-lsp--on-render-changed (_workspace params)
  "Run `sysml-lsp-render-changed-functions' for the document PARAMS names."
  (let ((file (lsp--uri-to-path (lsp-get (lsp-get params :textDocument) :uri))))
    (run-hook-with-args 'sysml-lsp-render-changed-functions
                        file (lsp-get params :version))))

(defun sysml-lsp--project-root ()
  "Return the root of the current project, or nil."
  (or (lsp-workspace-root)
      (when-let* ((project (project-current)))
        (project-root project))))

(defun sysml-lsp--find-server ()
  "Return the language server executable, or nil when none is found."
  (cond
   (sysml-lsp-server-path
    (and (file-executable-p sysml-lsp-server-path) sysml-lsp-server-path))
   ((when-let* ((root (sysml-lsp--project-root))
                (candidate (expand-file-name (concat "bin/" sysml-lsp--executable) root)))
      (and (file-executable-p candidate) candidate)))
   (t (executable-find sysml-lsp--executable))))

(defun sysml-lsp--find-tool ()
  "Return the `sysml' executable to run Run and Evaluate lenses with, or nil.
See `sysml-find-tool'."
  (sysml-find-tool (sysml-lsp--find-server) (sysml-lsp--project-root)))

;;;; Commands the server's code lenses invoke

;; These are client commands: the server names them in a lens, and the editor
;; carries them out.  lsp-mode sends a command no handler claims to the server
;; as `workspace/executeCommand', which `sysml-lsp' does not serve.

(defun sysml-lsp--show-references (command)
  "Show the references a reference-count lens COMMAND carries.
Its arguments are the document URI, the position and the locations."
  (let ((arguments (append (lsp-get command :arguments) nil)))
    (unless (= (length arguments) 3)
      (user-error "This reference lens carries no references to show"))
    (let ((locations (nth 2 arguments)))
      (if (seq-empty-p locations)
          (message "No references")
        (lsp-show-xrefs (lsp--locations-to-xref-items locations) nil t)))))

(defun sysml-lsp--run-paths (file)
  "Return the model paths to load when running an element declared in FILE.
The folders are the session's; see `sysml-run-paths'."
  (sysml-run-paths file (lsp-session-folders (lsp-session))))

(defun sysml-lsp--run-element (command)
  "Run the element a Run or Evaluate lens COMMAND names, with `sysml'.
See `sysml-run-element'."
  (let* ((args (car (append (lsp-get command :arguments) nil)))
         (uri (and args (lsp-get args :uri))))
    (sysml-run-element (and args (lsp-get args :kind))
                       (and args (lsp-get args :element))
                       (and uri (sysml-lsp--run-paths (lsp--uri-to-path uri)))
                       (sysml-lsp--find-tool))))

;;;; Bundled library documents

(defconst sysml-lsp--stdlib-content-method "opensysml/stdlibContent"
  "The request serving the text of a bundled library document.")

(define-obsolete-variable-alias 'sysml-lsp-stdlib-uri 'sysml-library-uri "0.14.0")
(define-obsolete-function-alias 'sysml-lsp-stdlib-uri-p #'sysml-library-uri-p "0.14.0")

(defun sysml-lsp--workspace ()
  "Return the current buffer's `sysml-lsp' workspace, or nil."
  (seq-find (lambda (ws)
              (eq (lsp--client-server-id (lsp--workspace-client ws)) 'sysml-lsp))
            (lsp-workspaces)))

(defun sysml-lsp-stdlib-buffer (uri)
  "Return a read-only buffer showing the bundled library document URI.
Its text is asked of the current buffer's server; see
`sysml-library-buffer'."
  (sysml-library-buffer
   uri (sysml-lsp--workspace)
   (lambda () (lsp-get (lsp-request sysml-lsp--stdlib-content-method (list :uri uri)) :text))))

(defun sysml-lsp--library-xref-item (location)
  "Return an xref item for LOCATION, a place in a bundled library document."
  (let ((buffer (sysml-lsp-stdlib-buffer (lsp--location-uri location)))
        (range (lsp--location-range location)))
    (with-current-buffer buffer
      (sysml-library-xref-item buffer
                                (lsp--position-to-point (lsp:range-start range))
                                (lsp--position-to-point (lsp:range-end range))))))

(defun sysml-lsp--xref-library-items (to-xref-items locations)
  "Call TO-XREF-ITEMS with LOCATIONS, adding those in the bundled library.
A `sysml-stdlib:' URI names no file, so lsp-mode would drop its
locations; each is located in the document's library buffer instead.
They follow the locations in files."
  (if (not (sysml-lsp--workspace))
      (funcall to-xref-items locations)
    (let* ((locations (cond ((vectorp locations) (append locations nil))
                            ((and locations (lsp--location-uri locations)) (list locations))
                            (t locations)))
           (library (seq-filter (lambda (location)
                                  (sysml-library-uri-p (lsp--location-uri location)))
                                locations)))
      (nconc (funcall to-xref-items (vconcat (seq-difference locations library #'eq)))
             (mapcar #'sysml-lsp--library-xref-item library)))))

(advice-add 'lsp--locations-to-xref-items :around #'sysml-lsp--xref-library-items)

(defun sysml-lsp--server-command ()
  "Return the command to start the language server with."
  (let ((server (sysml-lsp--find-server)))
    (unless server
      (user-error "Could not find %s: build it with `make build' and set `sysml-lsp-server-path', or put it on `exec-path'"
                  sysml-lsp--executable))
    (cons server sysml-lsp-server-args)))

(defun sysml-lsp--enable-semantic-tokens ()
  "Turn on semantic tokens in this buffer when `sysml-lsp-semantic-tokens' is set."
  (when sysml-lsp-semantic-tokens
    (setq-local lsp-semantic-tokens-enable t)))

(add-to-list 'lsp-language-id-configuration '(sysml-mode . "sysml"))
(add-to-list 'lsp-language-id-configuration '(kerml-mode . "kerml"))

(dolist (mode '(sysml-mode kerml-mode))
  (add-to-list 'lsp--formatting-indent-alist (cons mode 'sysml-indent-offset)))

(add-hook 'sysml-base-mode-hook #'sysml-lsp--enable-semantic-tokens)
;; Buffers visited before this file loaded ran the hook without it.
(dolist (buffer (buffer-list))
  (with-current-buffer buffer
    (when (derived-mode-p 'sysml-base-mode)
      (sysml-lsp--enable-semantic-tokens))))

(lsp-register-client
 (make-lsp-client
  :new-connection (lsp-stdio-connection #'sysml-lsp--server-command
                                        (lambda () (and (sysml-lsp--find-server) t)))
  :activation-fn (lsp-activate-on "sysml" "kerml")
  :server-id 'sysml-lsp
  :synchronize-sections '("sysml")
  :initialization-options (lambda () (lsp-configuration-section "sysml"))
  :notification-handlers
  (lsp-ht ("opensysml/renderChanged" #'sysml-lsp--on-render-changed))
  :action-handlers
  (lsp-ht ("editor.action.showReferences" #'sysml-lsp--show-references)
          ("opensysml.runElement" #'sysml-lsp--run-element))
  ;; The server colours every token, keywords included.  `sysml-mode' already
  ;; colours the lexical ones -- and keywords by category, which the server's
  ;; single "keyword" type would flatten -- so those types get no face here
  ;; and keep the mode's.  Modifiers are left off for the same reason: the
  ;; server marks an `abstract' declaration's name, not the keyword.
  :semantic-tokens-faces-overrides
  '(:types (("keyword" . nil)
            ("comment" . nil)
            ("string" . nil)
            ("number" . nil))
    :modifiers (("declaration" . nil)
                ("definition" . nil)
                ("readonly" . nil)
                ("abstract" . nil)))))

(provide 'sysml-lsp)

;;; sysml-lsp.el ends here
