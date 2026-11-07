;;; sysml-diagram.el --- Model views drawn from the OpenSysML language server  -*- lexical-binding: t; -*-

;; Copyright The OpenSysML Authors
;; SPDX-License-Identifier: Apache-2.0

;; Author: The OpenSysML Authors
;; URL: https://github.com/Open-MBEE/OpenSysML
;; Version: 0.1.0
;; Package-Requires: ((emacs "29.1") (lsp-mode "9.0"))
;; Keywords: languages

;;; Commentary:

;; `sysml-diagram-show' opens a buffer showing one view of the model being
;; edited, drawn from the server's `opensysml/render': a tree of the
;; elements the view shows, the edges between them, and what the
;; rendering could not represent.  It is the Emacs counterpart of the VS
;; Code extension's diagram panel.
;;
;; - The view is picked from the views the document declares, plus the
;;   pseudo-views that render a document declaring none.
;; - RET or a click on an element visits its declaration; moving point in
;;   the model highlights the element whose declaration holds it.
;; - The buffer redraws when the server says the document changed, and
;;   only while it is visible.  A rendering that fails keeps the last good
;;   one on screen, marked stale, with the error in the header line.
;; - With `mmdc' (the Mermaid CLI) installed and a graphical frame,
;;   `sysml-diagram-image' shows the Mermaid artifact as a picture.
;;
;; The methods are documented in docs/reference/lsp.md.  This file reads
;; their payloads as lsp-mode parses them, with `lsp-get'.

;;; Code:

(require 'lsp-mode)
(require 'sysml-mode)
(require 'sysml-lsp)

(defgroup sysml-diagram nil
  "Model views drawn from the OpenSysML language server."
  :group 'sysml
  :prefix "sysml-diagram-")

(defcustom sysml-diagram-mmdc-program "mmdc"
  "The Mermaid CLI `sysml-diagram-image' renders with."
  :type 'string
  :risky t)

(defcustom sysml-diagram-display-action
  '((display-buffer-reuse-window display-buffer-in-side-window)
    (side . right)
    (window-width . 0.4))
  "How `sysml-diagram-show' displays the view buffer.
See `display-buffer' for the format."
  :type display-buffer--action-custom-type)

(defface sysml-diagram-kind-face
  '((t :inherit font-lock-keyword-face))
  "Face for an element's kind, such as `part def'.")

(defface sysml-diagram-name-face
  '((t :inherit font-lock-type-face))
  "Face for an element's name.")

(defface sysml-diagram-detail-face
  '((t :inherit shadow))
  "Face for an element's detail, such as its type.")

(defface sysml-diagram-heading-face
  '((t :inherit bold))
  "Face for section headings.")

(defface sysml-diagram-highlight-face
  '((t :inherit highlight :extend t))
  "Face for the element whose declaration holds point in the model.")

(defface sysml-diagram-stale-face
  '((t :inherit shadow))
  "Face a view is shown in while it is out of date.")

(defconst sysml-diagram--render-method "opensysml/render")
(defconst sysml-diagram--views-method "opensysml/views")
(defconst sysml-diagram--capability :openSysmlRender
  "The experimental capability a server serving the render methods advertises.")

(defconst sysml-diagram--pseudo-views
  '(("#tree" . "Model tree (no view declared)")
    ("#interconnection" . "Interconnections (no view declared)")
    ("#state" . "State machines (no view declared)")
    ("#action" . "Action flows (no view declared)")
    ("#table" . "Element table (no view declared)"))
  "The pseudo-views the server renders a document through as if it declared one.")

;;;; Per-buffer state

(defvar-local sysml-diagram--source nil
  "In a view buffer, the model buffer it shows.")

(defvar-local sysml-diagram--view nil
  "In a view buffer, the view shown, as sent in the render request's `view'.")

(defvar-local sysml-diagram--form nil
  "In a view buffer, the form asked for: nil for the structured view, \"text\".")

(defvar-local sysml-diagram--result nil
  "In a view buffer, the last rendering drawn.")

(defvar-local sysml-diagram--error nil
  "In a view buffer, why the rendering shown is out of date, or nil.")

(defvar-local sysml-diagram--pending nil
  "In a view buffer, non-nil while a render request is in flight.")

(defvar-local sysml-diagram--again nil
  "In a view buffer, non-nil when a refresh was asked for while one was pending.")

(defvar-local sysml-diagram--highlight nil
  "In a view buffer, the overlay marking the element at point in the model.")

(defvar-local sysml-diagram--buffer nil
  "In a model buffer, its view buffer.")

;;;; Talking to the server

(defun sysml-diagram--workspace (source)
  "Return the `sysml-lsp' workspace of SOURCE, or nil."
  (with-current-buffer source
    (seq-find (lambda (ws)
                (eq (lsp--client-server-id (lsp--workspace-client ws)) 'sysml-lsp))
              (lsp-workspaces))))

(defun sysml-diagram--supported-p (workspace)
  "Return non-nil when WORKSPACE serves the render methods."
  (let ((experimental (lsp:server-capabilities-experimental?
                       (lsp--workspace-server-capabilities workspace))))
    (and experimental (eq (lsp-get experimental sysml-diagram--capability) t))))

(defun sysml-diagram--check (source)
  "Signal a `user-error' unless SOURCE's server serves the render methods."
  (let ((workspace (sysml-diagram--workspace source)))
    (unless workspace
      (user-error "No OpenSysML language server is running for %s; start it with M-x lsp"
                  (buffer-name source)))
    (unless (sysml-diagram--supported-p workspace)
      (user-error "This sysml-lsp does not serve diagrams; rebuild it with `make build'"))))

(defun sysml-diagram--text-document (source)
  "Return the TextDocumentIdentifier of SOURCE."
  (with-current-buffer source
    (list :uri (lsp--buffer-uri))))

(defun sysml-diagram--views (source)
  "Return the picker entries for SOURCE: (VALUE LABEL SUPPORTED REASON)."
  (let ((listing (condition-case err
                     (with-current-buffer source
                       (lsp-request sysml-diagram--views-method
                                    (list :textDocument (sysml-diagram--text-document source))))
                   ;; The listing is the picker's content, not the view: a
                   ;; failure there still leaves the pseudo-views.
                   (error (message "Listing the views of %s failed: %s"
                                   (buffer-name source) (error-message-string err))
                          nil))))
    (append
     (mapcar (lambda (info)
               (let ((name (lsp-get info :name)))
                 (list name (format "%s — %s" name (lsp-get info :kind))
                       (eq (lsp-get info :supported) t) (lsp-get info :reason))))
             (and listing (lsp-get listing :views)))
     (mapcar (lambda (pseudo) (list (car pseudo) (cdr pseudo) t nil))
             sysml-diagram--pseudo-views))))

(defun sysml-diagram--default-view (views)
  "Return the view to show first from the picker entries VIEWS.
A document declaring one drawable view shows it; one declaring several or
none shows its model tree, as the VS Code panel does."
  (let ((declared (seq-filter (lambda (v) (and (nth 2 v) (not (string-prefix-p "#" (car v)))))
                              views)))
    (if (= (length declared) 1) (car (car declared)) "#tree")))

(defun sysml-diagram--read-view (source)
  "Ask which view of SOURCE to show, and return it."
  (let* ((views (sysml-diagram--views source))
         (labels (mapcar (lambda (v) (cons (nth 1 v) v)) views))
         (default (seq-find (lambda (v) (equal (car v) (sysml-diagram--default-view views))) views))
         (choice (cdr (assoc (completing-read
                              "View: "
                              (lambda (string pred action)
                                (if (eq action 'metadata)
                                    `(metadata (annotation-function
                                                . ,(lambda (label)
                                                     (let ((v (cdr (assoc label labels))))
                                                       (unless (nth 2 v)
                                                         (concat "  (not drawable: " (nth 3 v) ")")))))
                                               (display-sort-function . identity))
                                  (complete-with-action action labels string pred)))
                              nil t nil nil (nth 1 default))
                             labels))))
    (unless (nth 2 choice)
      (user-error "%s cannot be drawn: %s" (car choice) (nth 3 choice)))
    (car choice)))

(defun sysml-diagram--request (buffer)
  "Ask the server for BUFFER's rendering, redrawing BUFFER when it answers."
  (with-current-buffer buffer
    (let ((source sysml-diagram--source)
          (params (append (list :textDocument (sysml-diagram--text-document sysml-diagram--source))
                          (unless (string-empty-p sysml-diagram--view)
                            (list :view sysml-diagram--view))
                          (when sysml-diagram--form
                            (list :form sysml-diagram--form)))))
      (setq sysml-diagram--pending t)
      (with-current-buffer source
        (lsp-request-async
         sysml-diagram--render-method params
         (lambda (result) (sysml-diagram--settle buffer result nil))
         :mode 'detached
         :error-handler (lambda (err)
                          (sysml-diagram--settle buffer nil (lsp:json-error-message err))))))))

(defun sysml-diagram--settle (buffer result error)
  "Draw RESULT, or keep the last view and show ERROR, in BUFFER."
  (when (buffer-live-p buffer)
    (with-current-buffer buffer
      (setq sysml-diagram--pending nil)
      (if error
          (setq sysml-diagram--error error)
        (setq sysml-diagram--result result
              sysml-diagram--error nil))
      (sysml-diagram--draw)
      (when sysml-diagram--again
        (setq sysml-diagram--again nil)
        (sysml-diagram-refresh)))))

(defun sysml-diagram--on-changed (file _version)
  "Redraw the view of FILE, when it is visible."
  (when-let* ((source (find-buffer-visiting file))
              (buffer (buffer-local-value 'sysml-diagram--buffer source)))
    (when (buffer-live-p buffer)
      (with-current-buffer buffer
        (sysml-diagram-refresh)))))

(add-hook 'sysml-lsp-render-changed-functions #'sysml-diagram--on-changed)

;;;; Drawing

(defun sysml-diagram--node-line (node)
  "Return the text of NODE's line, without indentation."
  (let ((detail (lsp-get node :detail)))
    (concat (propertize (lsp-get node :kind) 'face 'sysml-diagram-kind-face)
            " "
            (propertize (lsp-get node :name) 'face 'sysml-diagram-name-face)
            (if (or (null detail) (string-empty-p detail))
                ""
              (concat " " (propertize (format "(%s)" detail) 'face 'sysml-diagram-detail-face))))))

(defun sysml-diagram--insert-element (text origin id)
  "Insert TEXT as one element line, visiting ORIGIN when activated.
ID is the node it draws, or nil for a table row."
  (let ((start (point)))
    (insert text)
    (when origin
      (make-text-button start (point)
                        'sysml-diagram-origin origin
                        'sysml-diagram-node id
                        'action #'sysml-diagram--visit-button
                        'follow-link t
                        'face nil
                        'help-echo "RET or mouse-1: visit the declaration"))
    (when id
      (put-text-property start (point) 'sysml-diagram-node id))
    (insert "\n")))

(defun sysml-diagram--insert-tree (nodes)
  "Insert NODES as a tree, children under the node naming them as parent."
  (let ((children (make-hash-table :test #'equal))
        (known (make-hash-table :test #'equal))
        roots)
    (seq-doseq (node nodes)
      (puthash (lsp-get node :id) t known))
    (seq-doseq (node nodes)
      (let ((parent (lsp-get node :parent)))
        (if (and parent (gethash parent known))
            (push node (gethash parent children))
          (push node roots))))
    (cl-labels ((walk (node depth)
                  (insert (make-string (* 2 depth) ?\s))
                  (sysml-diagram--insert-element (sysml-diagram--node-line node)
                                                 (lsp-get node :origin)
                                                 (lsp-get node :id))
                  (dolist (child (reverse (gethash (lsp-get node :id) children)))
                    (walk child (1+ depth)))))
      (dolist (root (reverse roots))
        (walk root 0)))))

(defun sysml-diagram--insert-edges (nodes edges)
  "Insert EDGES between NODES, one per line, grouped by kind."
  (let ((names (make-hash-table :test #'equal)))
    (seq-doseq (node nodes)
      (puthash (lsp-get node :id) (lsp-get node :name) names))
    (insert "\n" (propertize "Edges" 'face 'sysml-diagram-heading-face) "\n")
    (seq-doseq (edge edges)
      (let ((label (lsp-get edge :label)))
        (sysml-diagram--insert-element
         (concat "  "
                 (propertize (lsp-get edge :kind) 'face 'sysml-diagram-kind-face) " "
                 (propertize (gethash (lsp-get edge :from) names (lsp-get edge :from))
                             'face 'sysml-diagram-name-face)
                 " → "
                 (propertize (gethash (lsp-get edge :to) names (lsp-get edge :to))
                             'face 'sysml-diagram-name-face)
                 (if (or (null label) (string-empty-p label))
                     ""
                   (concat " " (propertize (format "[%s]" label) 'face 'sysml-diagram-detail-face))))
         (lsp-get edge :origin) nil)))))

(defun sysml-diagram--insert-table (columns rows)
  "Insert ROWS under COLUMNS, aligned."
  (let* ((columns (append columns nil))
         (cells (mapcar (lambda (row) (append (lsp-get row :cells) nil)) (append rows nil)))
         (widths (seq-map-indexed
                  (lambda (column i)
                    (apply #'max (string-width column)
                           (mapcar (lambda (row) (string-width (or (nth i row) ""))) cells)))
                  columns))
         (format-row (lambda (row)
                       (string-trim-right
                        (mapconcat #'identity
                                   (seq-map-indexed
                                    (lambda (width i)
                                      (let ((cell (or (nth i row) "")))
                                        (concat cell (make-string (- width (string-width cell)) ?\s))))
                                    widths)
                                   "  ")))))
    (insert (propertize (funcall format-row columns) 'face 'sysml-diagram-heading-face) "\n")
    (insert (mapconcat (lambda (w) (make-string w ?-)) widths "  ") "\n")
    (seq-mapn (lambda (row line)
                (sysml-diagram--insert-element line (lsp-get row :origin) nil))
              rows (mapcar format-row cells))))

(defun sysml-diagram--header ()
  "Return the header line of the current view buffer."
  (let ((result sysml-diagram--result))
    (concat
     (propertize (if (string-empty-p sysml-diagram--view) "(declared view)" sysml-diagram--view)
                 'face 'sysml-diagram-heading-face)
     (when result (format " — %s" (lsp-get result :kind)))
     (when sysml-diagram--form (format " [%s]" sysml-diagram--form))
     (when sysml-diagram--error
       (concat "  " (propertize (format "stale: %s" sysml-diagram--error) 'face 'error))))))

(defconst sysml-diagram--tabular-kinds '("table" "matrix")
  "The rendering kinds whose result is columns and rows rather than a diagram.")

(defun sysml-diagram--draw ()
  "Redraw the current view buffer from its last rendering."
  (let ((inhibit-read-only t)
        (result sysml-diagram--result)
        (line (line-number-at-pos))
        (column (current-column)))
    (erase-buffer)
    (setq header-line-format (sysml-diagram--header))
    (cond
     ((null result)
      (insert (if sysml-diagram--error
                  (format "No rendering yet: %s\n" sysml-diagram--error)
                "Rendering...\n")))
     (t
      (let ((stated (lsp-get result :stated)))
        (unless (or (null stated) (string-empty-p stated))
          (insert (propertize stated 'face 'shadow) "\n\n")))
      (cond
       ((equal sysml-diagram--form "text")
        (insert (lsp-get result :artifact)))
       ((member (lsp-get result :kind) sysml-diagram--tabular-kinds)
        (sysml-diagram--insert-table (lsp-get result :columns) (lsp-get result :rows)))
       (t
        (let ((nodes (lsp-get result :nodes))
              (edges (lsp-get result :edges)))
          (if (seq-empty-p nodes)
              (insert (propertize "Nothing this view exposes is shown by this kind of rendering.\n"
                                  'face 'shadow))
            (sysml-diagram--insert-tree nodes))
          (unless (seq-empty-p edges)
            (sysml-diagram--insert-edges nodes edges)))))
      (let ((notices (lsp-get result :notices)))
        (unless (or (equal sysml-diagram--form "text") (seq-empty-p notices))
          (insert "\n" (propertize "Not represented" 'face 'sysml-diagram-heading-face) "\n")
          (seq-doseq (notice notices)
            (insert "  - " notice "\n"))))))
    (when sysml-diagram--error
      (add-face-text-property (point-min) (point-max) 'sysml-diagram-stale-face t))
    (goto-char (point-min))
    (forward-line (1- line))
    (move-to-column column)
    (setq sysml-diagram--highlight nil)
    (sysml-diagram--follow-source)))

;;;; Navigation

(defun sysml-diagram--origin-region (origin)
  "Return the (START . END) of ORIGIN's selection range in the current buffer."
  (let ((range (or (lsp-get origin :selectionRange) (lsp-get origin :range))))
    (cons (lsp--position-to-point (lsp-get range :start))
          (lsp--position-to-point (lsp-get range :end)))))

(defun sysml-diagram--visit (origin)
  "Visit the declaration ORIGIN locates, selecting its name.
A declaration in the bundled library is shown read-only, its text asked
of the server of the view's model buffer."
  (let* ((uri (lsp-get origin :uri))
         (source sysml-diagram--source)
         (buffer (if (sysml-lsp-stdlib-uri-p uri)
                     (if (buffer-live-p source)
                         (with-current-buffer source (sysml-lsp-stdlib-buffer uri))
                       (user-error "The model this view shows is no longer visited"))
                   (let ((file (lsp--uri-to-path uri)))
                     (or (find-buffer-visiting file) (find-file-noselect file))))))
    (pop-to-buffer buffer '((display-buffer-reuse-window display-buffer-use-some-window)))
    (let ((region (sysml-diagram--origin-region origin)))
      (push-mark (point) t)
      (goto-char (car region))
      (pulse-momentary-highlight-region (car region) (cdr region)))))

(defun sysml-diagram--visit-button (button)
  "Visit the declaration BUTTON draws."
  (sysml-diagram--visit (button-get button 'sysml-diagram-origin)))

(defun sysml-diagram-visit ()
  "Visit the declaration of the element at point."
  (interactive nil sysml-diagram-mode)
  (let ((origin (get-text-property (point) 'sysml-diagram-origin)))
    (unless origin
      (user-error "This element has no declaration to visit"))
    (sysml-diagram--visit origin)))

(defun sysml-diagram--node-at (source pos)
  "Return the id of the innermost node of the current view holding POS in SOURCE."
  (let ((file (buffer-file-name source))
        (nodes (and sysml-diagram--result (lsp-get sysml-diagram--result :nodes)))
        found found-size)
    (with-current-buffer source
      (seq-doseq (node nodes)
        (when-let* ((origin (lsp-get node :origin))
                    ((and file (file-equal-p (lsp--uri-to-path (lsp-get origin :uri)) file)))
                    (range (lsp-get origin :range))
                    (start (lsp--position-to-point (lsp-get range :start)))
                    (end (lsp--position-to-point (lsp-get range :end))))
          (when (and (<= start pos) (< pos end)
                     (or (null found-size) (< (- end start) found-size)))
            (setq found (lsp-get node :id)
                  found-size (- end start))))))
    found))

(defun sysml-diagram--follow-source ()
  "Highlight the element declared around point in the model."
  (when-let* ((source sysml-diagram--source)
              ((buffer-live-p source)))
    (let* ((pos (with-current-buffer source
                  (if-let* ((window (get-buffer-window source)))
                      (window-point window)
                    (point))))
           (id (sysml-diagram--node-at source pos)))
      (if (null id)
          (when sysml-diagram--highlight
            (delete-overlay sysml-diagram--highlight))
        (let ((match (sysml-diagram--find-node id)))
          (when match
            (unless (overlayp sysml-diagram--highlight)
              (setq sysml-diagram--highlight (make-overlay 1 1))
              (overlay-put sysml-diagram--highlight 'face 'sysml-diagram-highlight-face))
            (move-overlay sysml-diagram--highlight
                          (car match)
                          (min (point-max) (1+ (cdr match))))))))))

(defun sysml-diagram--find-node (id)
  "Return (BEG . END) of the line run drawing node ID, or nil."
  (let ((pos (point-min)) found)
    (while (and (not found) pos (< pos (point-max)))
      (if (equal (get-text-property pos 'sysml-diagram-node) id)
          (setq found (cons pos (or (next-single-property-change pos 'sysml-diagram-node)
                                    (point-max))))
        (setq pos (next-single-property-change pos 'sysml-diagram-node))))
    found))

(defun sysml-diagram--source-moved ()
  "In a model buffer, highlight the element at point in its view."
  (when-let* ((buffer sysml-diagram--buffer)
              ((buffer-live-p buffer))
              ((get-buffer-window buffer)))
    (with-current-buffer buffer
      (sysml-diagram--follow-source))))

;;;; Commands

(defun sysml-diagram-refresh ()
  "Ask the server for a fresh rendering of this view.
Nothing is requested while the buffer is hidden; showing it again
refreshes it."
  (interactive nil sysml-diagram-mode)
  (cond
   ((not (buffer-live-p sysml-diagram--source))
    (setq sysml-diagram--error "the model buffer was killed")
    (sysml-diagram--draw))
   ((not (get-buffer-window (current-buffer) t)))
   (sysml-diagram--pending (setq sysml-diagram--again t))
   ((not (sysml-diagram--workspace sysml-diagram--source))
    (setq sysml-diagram--error "the language server is not running")
    (sysml-diagram--draw))
   (t (sysml-diagram--request (current-buffer)))))

(defun sysml-diagram-pick-view ()
  "Show another view of the model."
  (interactive nil sysml-diagram-mode)
  (setq sysml-diagram--view (sysml-diagram--read-view sysml-diagram--source)
        sysml-diagram--result nil
        sysml-diagram--error nil)
  (sysml-diagram--draw)
  (sysml-diagram-refresh))

(defun sysml-diagram-toggle-text ()
  "Toggle between the structured view and the server's text rendering."
  (interactive nil sysml-diagram-mode)
  (setq sysml-diagram--form (if sysml-diagram--form nil "text"))
  (sysml-diagram-refresh))

(defvar-local sysml-diagram--shown nil
  "In a view buffer, non-nil while some window shows it.")

(defun sysml-diagram--window-changed (_frame)
  "Refresh view buffers that just became visible.
A hidden view asks for nothing, so one shown again may be out of date."
  (dolist (buffer (buffer-list))
    (when (eq (buffer-local-value 'major-mode buffer) 'sysml-diagram-mode)
      (with-current-buffer buffer
        (let ((shown (and (get-buffer-window buffer t) t)))
          (when (and shown (not sysml-diagram--shown))
            (sysml-diagram-refresh))
          (setq sysml-diagram--shown shown))))))

(defun sysml-diagram--killed ()
  "Detach a killed view buffer from its model."
  (when (buffer-live-p sysml-diagram--source)
    (with-current-buffer sysml-diagram--source
      (setq sysml-diagram--buffer nil)
      (remove-hook 'post-command-hook #'sysml-diagram--source-moved t))))

(defvar-keymap sysml-diagram-mode-map
  :doc "Keymap for `sysml-diagram-mode'."
  :parent (make-composed-keymap button-buffer-map special-mode-map)
  "RET" #'sysml-diagram-visit
  "g" #'sysml-diagram-refresh
  "v" #'sysml-diagram-pick-view
  "t" #'sysml-diagram-toggle-text
  "i" #'sysml-diagram-image
  "n" #'forward-button
  "p" #'backward-button)

(define-derived-mode sysml-diagram-mode special-mode "SysML-View"
  "Major mode of a buffer showing one view of a SysML model.

\\{sysml-diagram-mode-map}"
  (setq-local revert-buffer-function (lambda (&rest _) (sysml-diagram-refresh)))
  (setq-local truncate-lines t)
  (add-hook 'kill-buffer-hook #'sysml-diagram--killed nil t))

;;;###autoload
(defun sysml-diagram-show (&optional pick)
  "Show a view of the model in the current buffer beside it.
The first time, this shows the one view the document declares, or its
model tree.  With prefix argument PICK, or from the view buffer with
\\<sysml-diagram-mode-map>\\[sysml-diagram-pick-view], choose the view."
  (interactive "P" sysml-base-mode)
  (unless (derived-mode-p 'sysml-base-mode)
    (user-error "Visit a .sysml or .kerml file to show a view of it"))
  (let ((source (current-buffer)))
    (sysml-diagram--check source)
    (let* ((existing (and (buffer-live-p sysml-diagram--buffer) sysml-diagram--buffer))
           (buffer (or existing
                       (generate-new-buffer (format "*SysML View: %s*" (buffer-name source)))))
           (view (cond (pick (sysml-diagram--read-view source))
                       (existing nil)
                       (t (sysml-diagram--default-view (sysml-diagram--views source))))))
      (setq sysml-diagram--buffer buffer)
      (add-hook 'post-command-hook #'sysml-diagram--source-moved nil t)
      (with-current-buffer buffer
        (unless existing
          (sysml-diagram-mode)
          (setq sysml-diagram--source source))
        (when view
          (setq sysml-diagram--view view
                sysml-diagram--result nil
                sysml-diagram--error nil))
        (sysml-diagram--draw))
      (display-buffer buffer sysml-diagram-display-action)
      (with-current-buffer buffer
        (sysml-diagram-refresh)))))

;;;; Image

(defun sysml-diagram-image ()
  "Show the current view as a picture drawn by the Mermaid CLI.
Needs `sysml-diagram-mmdc-program' and a frame that can display SVG."
  (interactive nil sysml-diagram-mode)
  (unless (display-images-p)
    (user-error "This frame cannot display images"))
  (unless (image-type-available-p 'svg)
    (user-error "This Emacs was built without SVG support"))
  (let ((mmdc (executable-find sysml-diagram-mmdc-program)))
    (unless mmdc
      (user-error "Cannot find %s; install it with `npm install -g @mermaid-js/mermaid-cli'"
                  sysml-diagram-mmdc-program))
    (let* ((source sysml-diagram--source)
           (view sysml-diagram--view)
           (result (with-current-buffer source
                     (lsp-request sysml-diagram--render-method
                                  (append (list :textDocument (sysml-diagram--text-document source))
                                          (unless (string-empty-p view) (list :view view))))))
           (kind (lsp-get result :kind)))
      (when (member kind sysml-diagram--tabular-kinds)
        (user-error "A %s is not drawn; it is already shown as one" kind))
      (let ((svg (sysml-diagram--mermaid-to-svg mmdc (lsp-get result :artifact)))
            (buffer (get-buffer-create (format "*SysML Image: %s*" (buffer-name source)))))
        (with-current-buffer buffer
          (let ((inhibit-read-only t))
            (erase-buffer)
            (insert-image (create-image svg 'svg t))
            (special-mode)))
        (pop-to-buffer buffer)))))

(defun sysml-diagram--mermaid-to-svg (mmdc mermaid)
  "Return the SVG text MMDC draws for the Mermaid source MERMAID."
  (let* ((input (make-temp-file "sysml-view" nil ".mmd" mermaid))
         (output (concat (file-name-sans-extension input) ".svg"))
         (log (generate-new-buffer " *sysml-mmdc*")))
    (unwind-protect
        (let ((status (call-process mmdc nil log nil "--quiet" "-i" input "-o" output)))
          (unless (and (eql status 0) (file-exists-p output))
            (error "%s failed: %s" mmdc (with-current-buffer log (string-trim (buffer-string)))))
          (with-temp-buffer
            (insert-file-contents output)
            (buffer-string)))
      (delete-file input)
      (when (file-exists-p output) (delete-file output))
      (kill-buffer log))))

(add-hook 'window-buffer-change-functions #'sysml-diagram--window-changed)

(provide 'sysml-diagram)

;;; sysml-diagram.el ends here
