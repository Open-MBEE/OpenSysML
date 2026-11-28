;;; sysml-lsp-test.el --- Tests for sysml-lsp  -*- lexical-binding: t; -*-

;; Copyright The OpenSysML Authors
;; SPDX-License-Identifier: Apache-2.0

;;; Commentary:

;; Needs lsp-mode on `load-path'.  The live tests drive `bin/sysml-lsp';
;; see sysml-test.el.

;;; Code:

(require 'ert)
(require 'ert-x)
(require 'sysml-lsp)
(require 'sysml-test)

;;;; Server resolution

(ert-deftest sysml-lsp-prefers-the-configured-path ()
  (let* ((exe (make-temp-file "sysml-lsp")))
    (unwind-protect
        (progn
          (set-file-modes exe #o755)
          (let ((sysml-lsp-server-path exe))
            (should (equal (sysml-lsp--find-server) exe))))
      (delete-file exe))))

(ert-deftest sysml-lsp-refuses-a-configured-path-that-is-not-executable ()
  (let ((sysml-lsp-server-path "/nonexistent/sysml-lsp"))
    (should-not (sysml-lsp--find-server))
    (should-error (sysml-lsp--server-command) :type 'user-error)))

(ert-deftest sysml-lsp-finds-the-project-build ()
  (let* ((root (file-name-as-directory (make-temp-file "proj" t)))
         (exe (expand-file-name "bin/sysml-lsp" root)))
    (unwind-protect
        (progn
          (make-directory (file-name-directory exe))
          (write-region "" nil exe)
          (set-file-modes exe #o755)
          (cl-letf (((symbol-function 'sysml-lsp--project-root) (lambda () root)))
            (let ((sysml-lsp-server-path nil)
                  (exec-path nil)
                  (sysml-project-builds (list (cons root t))))
              (should (equal (sysml-lsp--find-server) exe)))
            (let ((sysml-lsp-server-path nil)
                  (exec-path nil)
                  (sysml-project-builds (list (cons root nil))))
              (should-not (sysml-lsp--find-server))
              (should-not (sysml-lsp--find-tool)))))
      (delete-directory root t))))

(ert-deftest sysml-lsp-says-which-build-it-may-not-start ()
  ;; lsp-mode reports a client whose server is missing as no server at all.
  (let* ((root (file-name-as-directory (make-temp-file "proj" t)))
         (exe (expand-file-name "bin/sysml-lsp" root))
         (sysml-lsp-server-path nil)
         (exec-path nil))
    (unwind-protect
        (cl-letf (((symbol-function 'sysml-lsp--project-root) (lambda () root)))
          (ert-with-message-capture said
            (should-not (sysml-lsp--server-found-p))
            (should (string-empty-p said)))
          (make-directory (file-name-directory exe))
          (write-region "" nil exe)
          (set-file-modes exe #o755)
          (let ((sysml-project-builds (list (cons root nil))))
            (ert-with-message-capture said
              (should-not (sysml-lsp--server-found-p))
              (should (string-search (file-name-directory exe) said))
              (should (string-search "not allowed" said))
              (should (string-search "sysml-project-builds" said))))
          (let ((sysml-project-builds (list (cons root t))))
            (ert-with-message-capture said
              (should (sysml-lsp--server-found-p))
              (should (string-empty-p said)))))
      (delete-directory root t))))

(ert-deftest sysml-lsp-starts-a-configured-server-in-a-refused-project ()
  (let* ((root (file-name-as-directory (make-temp-file "proj" t)))
         (exe (expand-file-name "bin/sysml-lsp" root))
         (tool (expand-file-name "bin/sysml" root))
         (sysml-lsp-server-path exe)
         (exec-path nil)
         (noninteractive t))
    (unwind-protect
        (cl-letf (((symbol-function 'sysml-lsp--project-root) (lambda () root))
                  ((symbol-function 'y-or-n-p)
                   (lambda (&rest _) (ert-fail "asked about the project build"))))
          (make-directory (file-name-directory exe))
          (dolist (f (list exe tool))
            (write-region "" nil f)
            (set-file-modes f #o755))
          (dolist (sysml-project-builds (list nil (list (cons root nil))))
            (ert-with-message-capture said
              (should (sysml-lsp--server-found-p))
              (should (equal (sysml-lsp--server-command) (list exe)))
              (should (equal (sysml-find-tool (sysml-lsp--find-server) root) tool))
              (should (equal said "")))))
      (delete-directory root t))))

(ert-deftest sysml-lsp-does-not-restart-a-build-no-longer-allowed ()
  ;; lsp-mode asks for the command again each time it starts the server.
  (let* ((root (file-name-as-directory (make-temp-file "proj" t)))
         (exe (expand-file-name "bin/sysml-lsp" root))
         (sysml-lsp-server-path nil)
         (exec-path nil)
         (sysml-project-builds (list (cons root t)))
         (connection (lsp--client-new-connection (gethash 'sysml-lsp lsp-clients)))
         (started nil))
    (unwind-protect
        (cl-letf (((symbol-function 'sysml-lsp--project-root) (lambda () root))
                  ((symbol-function 'make-process)
                   (lambda (&rest args) (push args started) (error "Started"))))
          (make-directory (file-name-directory exe))
          (write-region "" nil exe)
          (set-file-modes exe #o755)
          (should (funcall (plist-get connection :test?)))
          (should-error (funcall (plist-get connection :connect)
                                 #'ignore #'ignore "sysml-lsp" nil nil))
          (should (equal (plist-get (car started) :command) (list exe)))
          (setq started nil
                sysml-project-builds (list (cons root nil)))
          (should-not (funcall (plist-get connection :test?)))
          (should-error (funcall (plist-get connection :connect)
                                 #'ignore #'ignore "sysml-lsp" nil nil)
                        :type 'user-error)
          (should-not started))
      (delete-directory root t))))

(ert-deftest sysml-lsp-falls-back-to-exec-path ()
  (let* ((dir (file-name-as-directory (make-temp-file "path" t)))
         (exe (expand-file-name "sysml-lsp" dir)))
    (unwind-protect
        (progn
          (write-region "" nil exe)
          (set-file-modes exe #o755)
          (cl-letf (((symbol-function 'sysml-lsp--project-root) (lambda () nil)))
            (let ((sysml-lsp-server-path nil)
                  (exec-path (list dir)))
              (should (equal (sysml-lsp--find-server) exe)))))
      (delete-directory dir t))))

(ert-deftest sysml-lsp-passes-server-args ()
  (let ((sysml-lsp-server-path sysml-test--server)
        (sysml-lsp-server-args '("-strict")))
    (sysml-test--require-server)
    (should (equal (sysml-lsp--server-command) (list sysml-test--server "-strict")))))

;;;; Registration

(ert-deftest sysml-lsp-registers-both-languages ()
  (should (equal (alist-get 'sysml-mode lsp-language-id-configuration) "sysml"))
  (should (equal (alist-get 'kerml-mode lsp-language-id-configuration) "kerml"))
  (should (gethash 'sysml-lsp lsp-clients)))

(ert-deftest sysml-lsp-sends-strict-conformance ()
  (let ((sysml-lsp-strict-conformance t))
    (should (equal (json-encode (lsp-configuration-section "sysml"))
                   "{\"sysml\":{\"strictConformance\":true}}")))
  (let ((sysml-lsp-strict-conformance nil))
    (should (equal (json-encode (lsp-configuration-section "sysml"))
                   "{\"sysml\":{\"strictConformance\":false}}"))))

(ert-deftest sysml-lsp-formats-with-the-mode-offset ()
  (with-temp-buffer
    (sysml-mode)
    (setq-local sysml-indent-offset 3)
    (should (= (symbol-value (lsp--get-indent-width 'sysml-mode)) 3))))

(ert-deftest sysml-lsp-enables-semantic-tokens-per-buffer ()
  (with-temp-buffer
    (let ((sysml-lsp-semantic-tokens t))
      (sysml-mode)
      (should lsp-semantic-tokens-enable)
      (should (local-variable-p 'lsp-semantic-tokens-enable))))
  (with-temp-buffer
    (let ((sysml-lsp-semantic-tokens nil))
      (sysml-mode)
      (should-not (local-variable-p 'lsp-semantic-tokens-enable)))))

(ert-deftest sysml-lsp-handles-render-changed-itself ()
  "The client owns its server's notification, so none goes unhandled
when nothing shows a view."
  (should (eq (gethash "opensysml/renderChanged"
                       (lsp--client-notification-handlers (gethash 'sysml-lsp lsp-clients)))
              #'sysml-lsp--on-render-changed)))

(ert-deftest sysml-lsp-passes-render-changed-on ()
  (let* (seen
         (sysml-lsp-render-changed-functions
          (list (lambda (file version) (setq seen (list file version))))))
    (sysml-lsp--on-render-changed
     nil (lsp--read-json "{\"textDocument\":{\"uri\":\"file:///tmp/m.sysml\"},\"version\":8}"))
    (should (equal seen '("/tmp/m.sysml" 8)))))

;;;; Code lens commands

(ert-deftest sysml-lsp-handles-its-lens-commands ()
  "A lens command the client does not claim is sent to the server, which
serves no `workspace/executeCommand'."
  (let ((handlers (lsp--client-action-handlers (gethash 'sysml-lsp lsp-clients))))
    (should (eq (gethash "editor.action.showReferences" handlers) #'sysml-lsp--show-references))
    (should (eq (gethash "opensysml.runElement" handlers) #'sysml-lsp--run-element))))

(defun sysml-lsp-test--command (json)
  "Parse JSON as lsp-mode parses a lens's Command."
  (lsp--read-json json))

(ert-deftest sysml-lsp-shows-a-lens-references ()
  (let (shown)
    (cl-letf (((symbol-function 'lsp-show-xrefs)
               (lambda (xrefs _display references?) (setq shown (list (length xrefs) references?)))))
      (let ((file (make-temp-file "refs" nil ".sysml" "part def A;\npart a : A;\npart b : A;\n")))
        (unwind-protect
            (sysml-lsp--show-references
             (sysml-lsp-test--command
              (format "{\"title\":\"2 references\",\"command\":\"editor.action.showReferences\",\"arguments\":[\"file://%s\",{\"line\":0,\"character\":9},[{\"uri\":\"file://%s\",\"range\":{\"start\":{\"line\":1,\"character\":9},\"end\":{\"line\":1,\"character\":10}}},{\"uri\":\"file://%s\",\"range\":{\"start\":{\"line\":2,\"character\":9},\"end\":{\"line\":2,\"character\":10}}}]]}"
                      file file file)))
          (delete-file file))))
    (should (equal shown '(2 t)))))

(ert-deftest sysml-lsp-says-when-a-lens-counts-no-references ()
  (let (shown)
    (cl-letf (((symbol-function 'lsp-show-xrefs) (lambda (&rest _) (setq shown t))))
      (sysml-lsp--show-references
       (sysml-lsp-test--command
        "{\"title\":\"0 references\",\"command\":\"editor.action.showReferences\",\"arguments\":[\"file:///m.sysml\",{\"line\":0,\"character\":9},[]]}")))
    (should-not shown)))

(ert-deftest sysml-lsp-run-paths-are-every-folder-when-one-holds-the-file ()
  (cl-letf (((symbol-function 'lsp-session) (lambda () (make-lsp-session :folders '("/ws/" "/other")))))
    ;; The server indexes every root together, so a run loads them all.
    (should (equal (sysml-lsp--run-paths "/ws/sub/m.sysml") '("/ws/" "/other")))
    (should (equal (sysml-lsp--run-paths "/other/m.sysml") '("/ws/" "/other")))
    (should (equal (sysml-lsp--run-paths "/elsewhere/m.sysml") '("/elsewhere/m.sysml")))
    ;; A folder holds what lies under it, not a sibling sharing its prefix.
    (should (equal (sysml-lsp--run-paths "/otherwise/m.sysml") '("/otherwise/m.sysml")))))

(ert-deftest sysml-lsp-finds-the-tool-beside-the-server ()
  (let* ((dir (file-name-as-directory (make-temp-file "bin" t)))
         (server (expand-file-name "sysml-lsp" dir))
         (tool (expand-file-name "sysml" dir)))
    (unwind-protect
        (progn
          (dolist (f (list server tool))
            (write-region "" nil f)
            (set-file-modes f #o755))
          (let ((sysml-lsp-server-path server))
            (should (equal (sysml-lsp--find-tool) tool))))
      (delete-directory dir t))))

(ert-deftest sysml-lsp-does-not-run-a-lens-naming-nothing ()
  (let (started)
    (cl-letf (((symbol-function 'sysml-lsp--find-tool) (lambda () "/bin/sysml"))
              ((symbol-function 'sysml-lsp--run-paths) (lambda (_) '("/ws")))
              ((symbol-function 'compilation-start) (lambda (&rest _) (setq started t))))
      (dolist (json '("{\"command\":\"opensysml.runElement\",\"arguments\":[]}"
                      "{\"command\":\"opensysml.runElement\",\"arguments\":[{\"uri\":\"file:///ws/m.sysml\",\"element\":\"Demo::X\",\"kind\":\"part\"}]}"))
        (should-error (sysml-lsp--run-element (lsp--read-json json)) :type 'user-error)))
    (should-not started)))

(ert-deftest sysml-lsp-runs-a-lens-element ()
  (let (started)
    (cl-letf (((symbol-function 'sysml-lsp--find-tool) (lambda () "/bin/sysml"))
              ((symbol-function 'sysml-lsp--run-paths) (lambda (_) (list temporary-file-directory)))
              ((symbol-function 'save-some-buffers) #'ignore)
              ((symbol-function 'compilation-start)
               (lambda (command &rest _) (setq started (list command default-directory)))))
      (sysml-lsp--run-element
       (lsp--read-json "{\"title\":\"Run Fly\",\"command\":\"opensysml.runElement\",\"arguments\":[{\"uri\":\"file:///ws/m.sysml\",\"element\":\"Demo::Fly\",\"kind\":\"action\"}]}")))
    (should (equal (car started)
                   (mapconcat #'shell-quote-argument
                              (list "/bin/sysml" "-action" "Demo::Fly" temporary-file-directory) " ")))
    (should (equal (cadr started) (file-name-as-directory temporary-file-directory)))))

(defmacro sysml-lsp-test--with-modified-model (&rest body)
  "Run BODY with `buffer' visiting a model file whose change is not saved."
  (declare (indent 0))
  `(let* ((dir (make-temp-file "sysml-run" t))
          (buffer (find-file-noselect (expand-file-name "m.sysml" dir))))
     (unwind-protect
         (progn
           (with-current-buffer buffer
             (insert "action def Fly;\n"))
           ,@body)
       (with-current-buffer buffer
         (set-buffer-modified-p nil))
       (kill-buffer buffer)
       (delete-directory dir t))))

(defconst sysml-lsp-test--run-lens
  "{\"command\":\"opensysml.runElement\",\"arguments\":[{\"uri\":\"file:///ws/m.sysml\",\"element\":\"Demo::Fly\",\"kind\":\"action\"}]}"
  "A Run lens command naming an action.")

(ert-deftest sysml-lsp-does-not-run-a-model-left-unsaved ()
  (sysml-lsp-test--with-modified-model
    (let (started)
      (cl-letf (((symbol-function 'sysml-lsp--find-tool) (lambda () "/bin/sysml"))
                ((symbol-function 'sysml-lsp--run-paths) (lambda (_) (list temporary-file-directory)))
                ;; The save is declined.
                ((symbol-function 'save-some-buffers) #'ignore)
                ((symbol-function 'compilation-start) (lambda (&rest _) (setq started t))))
        (let ((err (should-error (sysml-lsp--run-element (lsp--read-json sysml-lsp-test--run-lens))
                                 :type 'user-error)))
          (should (string-match-p "m.sysml not saved" (cadr err)))))
      (should-not started))))

(ert-deftest sysml-lsp-runs-a-model-once-saved ()
  (sysml-lsp-test--with-modified-model
    (let (started)
      (cl-letf (((symbol-function 'sysml-lsp--find-tool) (lambda () "/bin/sysml"))
                ((symbol-function 'sysml-lsp--run-paths) (lambda (_) (list temporary-file-directory)))
                ((symbol-function 'save-some-buffers)
                 (lambda (&rest _) (with-current-buffer buffer (save-buffer))))
                ((symbol-function 'compilation-start) (lambda (&rest _) (setq started t))))
        (sysml-lsp--run-element (lsp--read-json sysml-lsp-test--run-lens)))
      (should started))))

(ert-deftest sysml-lsp-refuses-a-run-without-the-tool ()
  (cl-letf (((symbol-function 'sysml-lsp--find-tool) #'ignore)
            ((symbol-function 'sysml-lsp--run-paths) (lambda (_) '("/ws"))))
    (should-error
     (sysml-lsp--run-element
      (lsp--read-json "{\"command\":\"opensysml.runElement\",\"arguments\":[{\"uri\":\"file:///ws/m.sysml\",\"element\":\"Demo::Fly\",\"kind\":\"action\"}]}"))
     :type 'user-error)))

;;;; Live server

(defmacro sysml-lsp-test--with-session (file &rest body)
  "Visit FILE under a live `sysml-lsp' session and run BODY in its buffer."
  (declare (indent 1))
  `(progn
     (sysml-test--require-server)
     (let* ((file ,file)
            (lsp-session-file (make-temp-file "lsp-session"))
            (lsp-auto-guess-root t)
            (lsp-enable-file-watchers nil)
            ;; Under `emacs -Q' lsp-mode's autoloads are not loaded, so the
            ;; optional UI it would hook in is off; none of it is under test.
            (lsp-lens-enable nil)
            (lsp-modeline-workspace-status-enable nil)
            (lsp-modeline-code-actions-enable nil)
            (lsp-modeline-diagnostics-enable nil)
            (lsp-headerline-breadcrumb-enable nil)
            (lsp-ui-sideline-enable nil)
            (lsp-restart 'ignore)
            (sysml-lsp-server-path sysml-test--server)
            (buffer (find-file-noselect file)))
       (unwind-protect
           (with-current-buffer buffer
             (lsp)
             (sysml-test--wait (lambda ()
                                     (when-let* ((ws (car (lsp-workspaces))))
                                       (eq (lsp--workspace-status ws) 'initialized))))
             ,@body)
         ;; Close the document while the server still runs, then stop it, so
         ;; the next test does not join a workspace that is shutting down.
         (let ((ws (with-current-buffer buffer
                     (set-buffer-modified-p nil)
                     (car (lsp-workspaces)))))
           (kill-buffer buffer)
           (when ws
             (lsp-workspace-shutdown ws)
             (sysml-test--wait
              (lambda () (not (process-live-p (lsp--workspace-proc ws)))))))
         (delete-directory (file-name-directory file) t)
         (delete-file lsp-session-file)))))

(ert-deftest sysml-lsp-live-advertises-rendering ()
  (sysml-lsp-test--with-session (sysml-test--fixture "part def A;\n")
    (should (lsp-get (lsp:server-capabilities-experimental? (lsp--server-capabilities))
                     :openSysmlRender))))

(ert-deftest sysml-lsp-live-publishes-diagnostics ()
  (sysml-lsp-test--with-session (sysml-test--fixture "part def A {\n")
    (sysml-test--wait (lambda () (gethash (lsp--fix-path-casing (buffer-file-name))
                                              (lsp-diagnostics t))))))

(ert-deftest sysml-lsp-live-goes-to-definition ()
  (sysml-lsp-test--with-session
      (sysml-test--fixture "package P {\n    part def Engine;\n    part e : Engine;\n}\n")
    (goto-char (point-min))
    (search-forward ": Engine")
    (backward-char 2)
    (let* ((locs (lsp-request "textDocument/definition" (lsp--text-document-position-params)))
           (loc (if (vectorp locs) (aref locs 0) (if (listp locs) (car locs) locs))))
      (should (= (lsp:position-line (lsp:range-start (lsp:location-range loc))) 1)))))

(ert-deftest sysml-lsp-live-shows-a-library-declaration ()
  (sysml-lsp-test--with-session
      (sysml-test--fixture "package P {\n    private import ScalarValues::*;\n    attribute x : Integer;\n}\n")
    (goto-char (point-min))
    (search-forward ": Integer")
    (backward-char 2)
    (let* ((locs (lsp-request "textDocument/definition" (lsp--text-document-position-params)))
           (loc (if (vectorp locs) (aref locs 0) (if (listp locs) (car locs) locs)))
           (uri (lsp:location-uri loc))
           (buffer (progn
                     (should (sysml-library-uri-p uri))
                     (sysml-lsp-stdlib-buffer uri))))
      (unwind-protect
          (with-current-buffer buffer
            (should buffer-read-only)
            (should (derived-mode-p 'kerml-mode))
            (goto-char (lsp--position-to-point (lsp:range-start (lsp:location-range loc))))
            (should (string-match-p "datatype Integer"
                                    (buffer-substring (line-beginning-position) (line-end-position)))))
        (kill-buffer buffer)))))

(ert-deftest sysml-lsp-live-goes-to-a-library-definition ()
  (sysml-lsp-test--with-session
      (sysml-test--fixture "package P {\n    private import ScalarValues::*;\n    attribute x : Integer;\n}\n")
    (goto-char (point-min))
    (search-forward ": Integer")
    (backward-char 2)
    (let* ((item (car (xref-backend-definitions 'xref-lsp (xref-backend-identifier-at-point 'xref-lsp))))
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

(ert-deftest sysml-lsp-live-keeps-file-definitions ()
  (sysml-lsp-test--with-session
      (sysml-test--fixture "package P {\n    part def Engine;\n    part e : Engine;\n}\n")
    (goto-char (point-min))
    (search-forward ": Engine")
    (backward-char 2)
    (let ((location (xref-item-location
                     (car (xref-backend-definitions 'xref-lsp (xref-backend-identifier-at-point 'xref-lsp))))))
      (should (= (xref-location-line location) 2)))))

(ert-deftest sysml-lsp-live-change-notification-is-handled ()
  "Editing with no view open must not warn about an unknown notification."
  (let (warnings)
    (cl-letf* ((real-warn (symbol-function 'lsp-warn))
               ((symbol-function 'lsp-warn)
                (lambda (format-string &rest args)
                  (push (apply #'format format-string args) warnings)
                  (apply real-warn format-string args))))
      (let* ((changed nil)
             (sysml-lsp-render-changed-functions (list (lambda (&rest _) (setq changed t)))))
        (sysml-lsp-test--with-session (sysml-test--fixture "package P {\n    part def A;\n}\n")
          (goto-char (point-max))
          (insert "part def B;\n")
          (sysml-test--wait (lambda () changed)))))
    (should-not (seq-find (lambda (w) (string-match-p "Unknown notification" w)) warnings))))

(ert-deftest sysml-lsp-live-semantic-tokens-keep-keyword-faces ()
  (sysml-lsp-test--with-session
      (sysml-test--fixture "package P {\n    part def Engine;\n    part e : Engine;\n}\n")
    (should lsp-semantic-tokens-mode)
    (sysml-test--wait (lambda () lsp--semantic-tokens-cache))
    (font-lock-ensure)
    (goto-char (point-min))
    (search-forward "part e")
    (should (eq (get-text-property (match-beginning 0) 'face) 'sysml-declaration-keyword-face))))

(provide 'sysml-lsp-test)

;;; sysml-lsp-test.el ends here
