;;; sysml-diagram-test.el --- Tests for sysml-diagram  -*- lexical-binding: t; -*-

;; Copyright The OpenSysML Authors
;; SPDX-License-Identifier: Apache-2.0

;;; Commentary:

;; The drawing tests feed the view buffer renderings shaped as
;; docs/reference/lsp.md documents them.  The live tests drive a real
;; `sysml-lsp'; they skip without `bin/sysml-lsp', as sysml-lsp-test.el's do.

;;; Code:

(require 'ert)
(require 'sysml-diagram)
(require 'sysml-lsp-test)

(defun sysml-diagram-test--json (text)
  "Parse TEXT as lsp-mode parses a server's message."
  (lsp--read-json text))

(defun sysml-diagram-test--origin (line start end)
  "Return an origin JSON string declared on LINE, named from START to END."
  (format "{\"uri\":\"file:///m.sysml\",\"range\":{\"start\":{\"line\":%d,\"character\":0},\"end\":{\"line\":%d,\"character\":0}},\"selectionRange\":{\"start\":{\"line\":%d,\"character\":%d},\"end\":{\"line\":%d,\"character\":%d}}}"
          line (1+ line) line start line end))

(defmacro sysml-diagram-test--drawing (result &rest body)
  "Draw RESULT, a JSON string, in a view buffer and run BODY there."
  (declare (indent 1))
  `(with-temp-buffer
     (sysml-diagram-mode)
     (setq sysml-diagram--view "#tree"
           sysml-diagram--result (sysml-diagram-test--json ,result))
     (sysml-diagram--draw)
     ,@body))

(defconst sysml-diagram-test--tree
  (concat "{\"view\":\"\",\"kind\":\"tree\",\"stated\":\"no view declared\",\"form\":\"mermaid\","
          "\"artifact\":\"flowchart TD\","
          "\"nodes\":["
          "{\"id\":\"n0\",\"kind\":\"part def\",\"name\":\"Kit::Widget\",\"detail\":\"\",\"origin\":"
          (sysml-diagram-test--origin 0 9 15) "},"
          "{\"id\":\"n1\",\"kind\":\"part\",\"name\":\"cog\",\"detail\":\"Cog\",\"parent\":\"n0\",\"origin\":"
          (sysml-diagram-test--origin 1 9 12) "},"
          "{\"id\":\"n2\",\"kind\":\"part\",\"name\":\"gear\",\"detail\":\"\",\"parent\":\"n0\"}],"
          "\"edges\":[{\"from\":\"n1\",\"to\":\"n2\",\"label\":\"drives\",\"kind\":\"connection\"}],"
          "\"notices\":[\"something it could not show\"],\"version\":1}")
  "A tree rendering of a part with two children, one without an origin.")

(ert-deftest sysml-diagram-draws-a-tree ()
  (sysml-diagram-test--drawing sysml-diagram-test--tree
    (should (equal (buffer-substring-no-properties (point-min) (point-max))
                   (concat "no view declared\n\n"
                           "part def Kit::Widget\n"
                           "  part cog (Cog)\n"
                           "  part gear\n"
                           "\nEdges\n"
                           "  connection cog → gear [drives]\n"
                           "\nNot represented\n"
                           "  - something it could not show\n")))
    (should (string-match-p "#tree — tree" (format "%s" header-line-format)))))

(ert-deftest sysml-diagram-links-located-elements-only ()
  (sysml-diagram-test--drawing sysml-diagram-test--tree
    (goto-char (point-min))
    (search-forward "part cog")
    (should (get-text-property (match-beginning 0) 'sysml-diagram-origin))
    (search-forward "part gear")
    (should-not (get-text-property (match-beginning 0) 'sysml-diagram-origin))
    (should-error (sysml-diagram-visit) :type 'user-error)))

(ert-deftest sysml-diagram-keeps-the-last-rendering-when-one-fails ()
  (sysml-diagram-test--drawing sysml-diagram-test--tree
    (sysml-diagram--settle (current-buffer) nil "parse error")
    (should (string-match-p "part cog" (buffer-string)))
    (should (string-match-p "stale: parse error" (format "%s" header-line-format)))
    (should (memq 'sysml-diagram-stale-face
                  (ensure-list (get-text-property (point-min) 'face))))
    (sysml-diagram--settle (current-buffer) (sysml-diagram-test--json sysml-diagram-test--tree) nil)
    (should-not (string-match-p "stale" (format "%s" header-line-format)))))

(ert-deftest sysml-diagram-draws-a-table ()
  (sysml-diagram-test--drawing
      (concat "{\"view\":\"\",\"kind\":\"table\",\"stated\":\"\",\"form\":\"markdown\",\"artifact\":\"\","
              "\"nodes\":[],\"edges\":[],\"columns\":[\"Element\",\"Kind\"],"
              "\"rows\":[{\"cells\":[\"Widget\",\"part def\"],\"origin\":" (sysml-diagram-test--origin 0 9 15) "},"
              "{\"cells\":[\"cog\",\"part\"]}],\"notices\":[],\"version\":1}")
    (should (equal (buffer-substring-no-properties (point-min) (point-max))
                   "Element  Kind\n-------  --------\nWidget   part def\ncog      part\n"))
    (goto-char (point-min))
    (search-forward "Widget")
    (should (get-text-property (match-beginning 0) 'sysml-diagram-origin))))

(ert-deftest sysml-diagram-says-when-nothing-is-shown ()
  (sysml-diagram-test--drawing
      "{\"view\":\"\",\"kind\":\"state\",\"stated\":\"\",\"form\":\"mermaid\",\"artifact\":\"\",\"nodes\":[],\"edges\":[],\"notices\":[],\"version\":1}"
    (should (string-match-p "Nothing this view exposes" (buffer-string)))))

(ert-deftest sysml-diagram-shows-the-text-form-verbatim ()
  (with-temp-buffer
    (sysml-diagram-mode)
    (setq sysml-diagram--view "#tree"
          sysml-diagram--form "text"
          sysml-diagram--result (sysml-diagram-test--json
                                 "{\"view\":\"\",\"kind\":\"tree\",\"stated\":\"\",\"form\":\"text\",\"artifact\":\"part def A\\n  part b\\n\",\"nodes\":[],\"edges\":[],\"notices\":[\"x\"],\"version\":1}"))
    (sysml-diagram--draw)
    (should (equal (buffer-substring-no-properties (point-min) (point-max)) "part def A\n  part b\n"))))

(ert-deftest sysml-diagram-defaults-to-the-one-declared-view ()
  (should (equal (sysml-diagram--default-view '(("V::a" "a" t nil) ("#tree" "t" t nil))) "V::a"))
  (should (equal (sysml-diagram--default-view '(("V::a" "a" t nil) ("V::b" "b" t nil) ("#tree" "t" t nil)))
                 "#tree"))
  (should (equal (sysml-diagram--default-view '(("V::s" "s" nil "no") ("#tree" "t" t nil))) "#tree"))
  (should (equal (sysml-diagram--default-view '(("#tree" "t" t nil))) "#tree")))

(ert-deftest sysml-diagram-refuses-an-undrawable-view ()
  (cl-letf (((symbol-function 'sysml-diagram--views)
             (lambda (_) '(("V::seq" "V::seq — sequence" nil "sequence is not supported")
                           ("#tree" "Model tree" t nil))))
            ((symbol-function 'completing-read) (lambda (&rest _) "V::seq — sequence")))
    (should-error (sysml-diagram--read-view (current-buffer)) :type 'user-error)))

(ert-deftest sysml-diagram-listens-for-changes ()
  (should (memq #'sysml-diagram--on-changed sysml-lsp-render-changed-functions)))

(ert-deftest sysml-diagram-show-needs-a-model-buffer ()
  (with-temp-buffer
    (fundamental-mode)
    (should-error (sysml-diagram-show) :type 'user-error)))

;;;; Live server

(defun sysml-diagram-test--shown (source)
  "Show SOURCE's view and wait for its first rendering; return the view buffer."
  (with-current-buffer source
    (sysml-diagram-show))
  (let ((buffer (buffer-local-value 'sysml-diagram--buffer source)))
    (sysml-lsp-test--wait (lambda () (buffer-local-value 'sysml-diagram--result buffer)))
    buffer))

(ert-deftest sysml-diagram-live-renders-visits-and-follows ()
  (sysml-lsp-test--with-session
      (sysml-lsp-test--fixture "package P {\n    part def Engine {\n        attribute power;\n    }\n}\n")
    (let ((source (current-buffer))
          (buffer (sysml-diagram-test--shown (current-buffer))))
      (unwind-protect
          (progn
            (with-current-buffer buffer
              (should (string-match-p "part def P::Engine" (buffer-string)))
              (should (string-match-p "  attribute power" (buffer-string))))
            ;; Point in the model highlights the innermost declaration holding it.
            (with-current-buffer source
              (goto-char (point-min))
              (search-forward "power")
              (sysml-diagram--source-moved))
            (with-current-buffer buffer
              (should (string-match-p "attribute power"
                                      (buffer-substring (overlay-start sysml-diagram--highlight)
                                                        (overlay-end sysml-diagram--highlight)))))
            ;; Visiting an element selects its name in the model.
            (with-current-buffer buffer
              (goto-char (point-min))
              (search-forward "part def P::Engine")
              (backward-char 2)
              (sysml-diagram-visit))
            (should (eq (current-buffer) source))
            (should (looking-at-p "Engine")))
        (kill-buffer buffer)))))

(ert-deftest sysml-diagram-live-redraws-on-change ()
  (sysml-lsp-test--with-session (sysml-lsp-test--fixture "package P {\n    part def A;\n}\n")
    (let ((source (current-buffer))
          (buffer (sysml-diagram-test--shown (current-buffer))))
      (unwind-protect
          (progn
            (with-current-buffer source
              (goto-char (point-min))
              (search-forward "part def A;")
              (insert "\n    part def Added;"))
            (sysml-lsp-test--wait
             (lambda () (with-current-buffer buffer (string-match-p "P::Added" (buffer-string))))))
        (kill-buffer buffer)))))

(ert-deftest sysml-diagram-live-reports-a-failed-render ()
  (sysml-lsp-test--with-session (sysml-lsp-test--fixture "package P {\n    part def A;\n}\n")
    (let ((buffer (sysml-diagram-test--shown (current-buffer))))
      (unwind-protect
          (with-current-buffer buffer
            (setq sysml-diagram--view "#nosuch")
            (sysml-diagram-refresh)
            (sysml-lsp-test--wait (lambda () sysml-diagram--error))
            (should (string-match-p "no pseudo-view" sysml-diagram--error))
            (should (string-match-p "P::A" (buffer-string))))
        (kill-buffer buffer)))))

(provide 'sysml-diagram-test)

;;; sysml-diagram-test.el ends here
