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

(defvar tramp-methods)
(defvar tramp-default-host-alist)
(declare-function tramp-cleanup-all-connections "tramp-cmds")

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
  (let ((sysml-lsp-server-path "/nonexistent/sysml-lsp")
        (current-prefix-arg nil))
    (should-error (sysml-eglot--server-command) :type 'user-error)
    ;; Not the bare name: Eglot would start another server from `exec-path'.
    (should-error (sysml-eglot--server-command t) :type 'user-error)))

(ert-deftest sysml-eglot-offers-a-stale-configured-path-to-the-prefix-prompt ()
  (let ((sysml-lsp-server-path "/nonexistent/sysml-lsp")
        (sysml-lsp-server-args '("-strict"))
        (current-prefix-arg '(4)))
    (should (equal (sysml-eglot--server-command t)
                   '("/nonexistent/sysml-lsp" "-strict")))
    (should-error (sysml-eglot--server-command) :type 'user-error)))

(ert-deftest sysml-eglot-prompts-past-a-stale-configured-path ()
  (let* ((file (sysml-test--fixture "package P;\n"))
         (buffer (find-file-noselect file))
         (sysml-lsp-server-path "/nonexistent/sysml-lsp")
         (sysml-lsp-server-args nil)
         (current-prefix-arg '(4))
         offered)
    (unwind-protect
        (with-current-buffer buffer
          (cl-letf (((symbol-function 'read-shell-command)
                     (lambda (_prompt initial &rest _)
                       (setq offered initial)
                       "/opt/other/sysml-lsp -strict")))
            (should (equal (nth 3 (eglot--guess-contact t))
                           '("/opt/other/sysml-lsp" "-strict"))))
          (should (equal offered "/nonexistent/sysml-lsp")))
      (kill-buffer buffer)
      (delete-directory (file-name-directory file) t))))

(ert-deftest sysml-eglot-leaves-a-missing-server-to-the-prompt-when-interactive ()
  (let* ((empty (file-name-as-directory (make-temp-file "empty" t)))
         (sysml-lsp-server-path nil)
         (sysml-lsp-server-args '("-strict"))
         (exec-path nil))
    (unwind-protect
        (progn
          (should (equal (sysml-eglot--server-command t (cons 'transient empty))
                         '("sysml-lsp" "-strict")))
          (should-error (sysml-eglot--server-command nil (cons 'transient empty))
                        :type 'user-error))
      (delete-directory empty t))))

(ert-deftest sysml-eglot-names-a-remote-build-as-its-host-does ()
  (skip-unless (not (eq system-type 'windows-nt)))
  (require 'tramp)
  ;; A TRAMP method that reaches this machine through a local shell.
  (add-to-list 'tramp-methods
               '("sysmlmock"
                 (tramp-login-program "sh")
                 (tramp-login-args (("-i")))
                 (tramp-remote-shell "/bin/sh")
                 (tramp-remote-shell-args ("-c"))
                 (tramp-connection-timeout 10)))
  (let* ((tramp-default-host-alist `(("\\`sysmlmock\\'" nil ,(system-name))))
         (root (file-name-as-directory
                (make-temp-file (concat "/sysmlmock::" temporary-file-directory "proj") t)))
         (exe (expand-file-name "bin/sysml-lsp" root))
         (sysml-lsp-server-path nil)
         (sysml-lsp-server-args nil))
    (unwind-protect
        (progn
          (should (file-remote-p exe))
          (make-directory (file-name-directory exe))
          (write-region "" nil exe)
          (set-file-modes exe #o755)
          (should (equal (sysml-eglot--server-command nil (cons 'transient root))
                         (list (file-local-name exe)))))
      (delete-directory root t)
      (tramp-cleanup-all-connections))))

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

;;;; Running elements

(defun sysml-eglot-test--lens (line end title element kind)
  "Return a Run lens titled TITLE naming ELEMENT of KIND.
Its range is on LINE, from column 15 to column END."
  (list :range (list :start (list :line line :character 15)
                     :end (list :line line :character end))
        :command (list :title title :command "opensysml.runElement"
                       :arguments (vector (list :uri "file:///ws/m.sysml"
                                                :element element :kind kind)))))

(defmacro sysml-eglot-test--with-lenses (lenses &rest body)
  "Run BODY in a model buffer whose server answers LENSES, a vector.
`started' holds the arguments `sysml-run-element' was called with."
  (declare (indent 1))
  `(let (started)
     (cl-letf (((symbol-function 'eglot-current-server) (lambda () 'server))
               ((symbol-function 'eglot-server-capable) (lambda (&rest _) t))
               ((symbol-function 'eglot--signal-textDocument/didChange) #'ignore)
               ((symbol-function 'sysml-eglot--folders) (lambda (_) '("/ws")))
               ;; Eglot asks the server whether its project is remote.
               ((symbol-function 'eglot-uri-to-path)
                (lambda (uri) (string-remove-prefix "file://" uri)))
               ((symbol-function 'jsonrpc-request)
                (lambda (_server method _params)
                  (should (eq method :textDocument/codeLens))
                  ,lenses))
               ((symbol-function 'sysml-eglot--find-tool) (lambda (_) "/bin/sysml"))
               ((symbol-function 'sysml-run-element)
                (lambda (&rest args) (setq started args))))
       (with-temp-buffer
         (insert "package Demo {\n    action def Fly;\n    action def Land;\n}\n")
         (setq buffer-file-name "/ws/m.sysml")
         (unwind-protect
             (progn ,@body)
           (setq buffer-file-name nil))))))

(defconst sysml-eglot-test--lenses
  (vector
   ;; A reference-count lens carries no command until it is resolved.
   (list :range '(:start (:line 1 :character 15) :end (:line 1 :character 18))
         :data '(:element "Demo::Fly"))
   (sysml-eglot-test--lens 1 18 "Run Fly" "Demo::Fly" "action")
   (sysml-eglot-test--lens 2 19 "Run Land" "Demo::Land" "action"))
  "The lenses of the buffer `sysml-eglot-test--with-lenses' makes.")

(ert-deftest sysml-eglot-runs-the-element-at-point ()
  (sysml-eglot-test--with-lenses sysml-eglot-test--lenses
    (goto-char (point-min))
    (search-forward "Land")
    (sysml-eglot-run-element)
    (should (equal started '("action" "Demo::Land" ("/ws") "/bin/sysml")))))

(ert-deftest sysml-eglot-reads-the-element-away-from-a-name ()
  (sysml-eglot-test--with-lenses sysml-eglot-test--lenses
    (goto-char (point-min))
    (cl-letf (((symbol-function 'completing-read)
               (lambda (_prompt elements &rest _)
                 (should (equal elements '("Demo::Fly" "Demo::Land")))
                 "Demo::Fly")))
      (sysml-eglot-run-element))
    (should (equal started '("action" "Demo::Fly" ("/ws") "/bin/sysml")))))

(ert-deftest sysml-eglot-reads-the-element-with-a-prefix-argument ()
  (sysml-eglot-test--with-lenses sysml-eglot-test--lenses
    (goto-char (point-min))
    (search-forward "Land")
    (cl-letf (((symbol-function 'completing-read) (lambda (&rest _) "Demo::Fly")))
      (sysml-eglot-run-element t))
    (should (equal (cadr started) "Demo::Fly"))))

(ert-deftest sysml-eglot-tells-elements-sharing-a-name-apart ()
  (sysml-eglot-test--with-lenses
      (vector (sysml-eglot-test--lens 1 18 "Run Fly" "Demo::Fly" "action")
              (sysml-eglot-test--lens 2 19 "Run Fly" "Other::Fly" "action"))
    (goto-char (point-min))
    (cl-letf (((symbol-function 'completing-read) (lambda (&rest _) "Other::Fly")))
      (sysml-eglot-run-element))
    (should (equal (cadr started) "Other::Fly"))))

(ert-deftest sysml-eglot-says-when-nothing-can-be-run ()
  (sysml-eglot-test--with-lenses (vector (aref sysml-eglot-test--lenses 0))
    (should-error (sysml-eglot-run-element) :type 'user-error)
    (should-not started)))

(ert-deftest sysml-eglot-does-not-run-without-a-server ()
  (with-temp-buffer
    (should-error (sysml-eglot-run-element) :type 'user-error)))

(ert-deftest sysml-eglot-finds-the-tool-beside-the-server ()
  (let* ((dir (file-name-as-directory (make-temp-file "bin" t)))
         (exec-path (list dir)))
    (unwind-protect
        (progn
          (dolist (f '("sysml-lsp" "sysml"))
            (write-region "" nil (expand-file-name f dir))
            (set-file-modes (expand-file-name f dir) #o755))
          (should (equal (sysml-eglot--find-tool nil) (expand-file-name "sysml" dir))))
      (delete-directory dir t))))

(ert-deftest sysml-eglot-finds-the-tool-beside-the-configured-server ()
  (let* ((dir (file-name-as-directory (make-temp-file "bin" t)))
         (sysml-lsp-server-path (expand-file-name "sysml-lsp" dir))
         (exec-path nil))
    (unwind-protect
        (progn
          (dolist (f '("sysml-lsp" "sysml"))
            (write-region "" nil (expand-file-name f dir))
            (set-file-modes (expand-file-name f dir) #o755))
          (should (equal (sysml-eglot--find-tool nil) (expand-file-name "sysml" dir))))
      (delete-directory dir t))))

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

(defun sysml-eglot-test--severities ()
  "Return the LSP severity of each diagnostic the server last published here."
  (flymake-start)
  (mapcar (lambda (diagnostic)
            (plist-get (alist-get 'eglot-lsp-diag (flymake-diagnostic-data diagnostic))
                       :severity))
          (flymake-diagnostics)))

(ert-deftest sysml-eglot-live-takes-strict-conformance-from-the-workspace-configuration ()
  (let ((configuration (default-value 'eglot-workspace-configuration)))
    (unwind-protect
        (progn
          (setq-default eglot-workspace-configuration '(:sysml (:strictConformance t)))
          (sysml-eglot-test--with-session
              (sysml-test--fixture "package P {\n    part def A;\n    part def A;\n}\n")
            ;; Two duplicate names: errors (1) when strict, warnings (2) otherwise.
            (sysml-test--wait (lambda () (equal (sysml-eglot-test--severities) '(1 1))))
            ;; A changed value reaches a running server without a restart.
            (setq-default eglot-workspace-configuration
                          '(:sysml (:strictConformance :json-false)))
            (eglot-signal-didChangeConfiguration (eglot-current-server))
            (sysml-test--wait (lambda () (equal (sysml-eglot-test--severities) '(2 2))))))
      (setq-default eglot-workspace-configuration configuration))))

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

(ert-deftest sysml-eglot-live-goes-to-a-library-definition ()
  (sysml-eglot-test--with-session
      (sysml-test--fixture "package P {\n    private import ScalarValues::*;\n    attribute x : Integer;\n}\n")
    (goto-char (point-min))
    (search-forward ": Integer")
    (backward-char 2)
    (let* ((item (car (xref-backend-definitions 'eglot (xref-backend-identifier-at-point 'eglot))))
           (marker (xref-location-marker (xref-item-location item)))
           (buffer (marker-buffer marker)))
      (unwind-protect
          (with-current-buffer buffer
            (should (sysml-library-uri-p sysml-library-uri))
            (should-not buffer-file-name)
            (should buffer-read-only)
            ;; Only an item with a match length is offered for replacement.
            (should-not (xref-match-length item))
            (should (string-match-p "datatype Integer" (xref-item-summary item)))
            (goto-char marker)
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

(ert-deftest sysml-eglot-live-runs-the-element-at-point ()
  (let ((sysml (expand-file-name "bin/sysml" sysml-test--repo)))
    (unless (file-executable-p sysml)
      (sysml-test--require-server)
      (ert-skip (format "%s is missing; run `make build'" sysml)))
    (sysml-eglot-test--with-session
        (sysml-test--fixture "package Demo {\n    action def Fly;\n    part def Wing;\n}\n")
      (let ((exec-path (cons (file-name-directory sysml) exec-path))
            (root (file-name-directory buffer-file-name))
            started)
        (cl-letf (((symbol-function 'compilation-start)
                   (lambda (command &rest _) (setq started (list command default-directory)))))
          ;; An edit not yet sent: the lens must still be found where the name now is.
          (goto-char (point-min))
          (insert "\n")
          (search-forward "Fly")
          (cl-letf (((symbol-function 'save-some-buffers) (lambda (&rest _) (save-buffer))))
            (sysml-eglot-run-element)))
        (should (equal (car started)
                       (mapconcat #'shell-quote-argument
                                  (list sysml "-action" "Demo::Fly" (directory-file-name root))
                                  " ")))
        (should (equal (cadr started) root))))))

(provide 'sysml-eglot-test)

;;; sysml-eglot-test.el ends here
