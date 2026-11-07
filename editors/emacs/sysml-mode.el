;;; sysml-mode.el --- Major modes for SysML v2 and KerML  -*- lexical-binding: t; -*-

;; Copyright The OpenSysML Authors
;; SPDX-License-Identifier: Apache-2.0

;; Author: The OpenSysML Authors
;; URL: https://github.com/Open-MBEE/OpenSysML
;; Version: 0.1.0
;; Package-Requires: ((emacs "29.1"))
;; Keywords: languages

;;; Commentary:

;; Major modes for SysML v2 (`.sysml') and KerML (`.kerml') source.
;;
;; Highlighting is generated from OpenSysML's lexer (see
;; `sysml-mode-keywords.el'), so every reserved word is coloured and the
;; list cannot drift from the language.  Indentation follows the rule
;; `sysml-lsp' formats by: one level per open brace, and one more for a
;; line that continues the statement above it.
;;
;; `sysml-lsp.el' connects the modes to the OpenSysML language server
;; through lsp-mode.

;;; Code:

(require 'seq)
(require 'sysml-mode-keywords)

(defgroup sysml nil
  "SysML v2 and KerML editing."
  :group 'languages
  :prefix "sysml-"
  :link '(url-link "https://github.com/Open-MBEE/OpenSysML"))

(defcustom sysml-indent-offset 4
  "Columns per nesting level.
The default matches the language server's formatter."
  :type 'natnum
  :safe #'natnump)

;;;; Faces

(defgroup sysml-faces nil
  "Faces used to highlight SysML v2 and KerML."
  :group 'sysml
  :group 'faces)

(defface sysml-declaration-keyword-face
  '((t :inherit font-lock-keyword-face))
  "Face for keywords that declare an element, such as `part' and `def'.")

(defface sysml-control-keyword-face
  '((t :inherit font-lock-keyword-face))
  "Face for behavioural keywords, such as `then', `accept' and `transition'.")

(defface sysml-modifier-keyword-face
  '((t :inherit font-lock-type-face))
  "Face for modifiers, such as `abstract', `in' and `private'.")

(defface sysml-relationship-keyword-face
  '((t :inherit font-lock-builtin-face))
  "Face for relationship keywords, such as `specializes' and `import'.")

(defface sysml-operator-keyword-face
  '((t :inherit font-lock-operator-face))
  "Face for word operators, such as `and', `implies' and `istype'.")

(defface sysml-other-keyword-face
  '((t :inherit font-lock-keyword-face))
  "Face for reserved words outside the other keyword categories.")

(defface sysml-contextual-keyword-face
  '((t :inherit font-lock-preprocessor-face))
  "Face for words read as syntax in one position only, such as `chain'.
The lexer does not reserve them, so they are highlighted only where they
introduce syntax.")

(defface sysml-constant-face
  '((t :inherit font-lock-constant-face))
  "Face for `true', `false' and `null'.")

(defface sysml-declaration-name-face
  '((t :inherit font-lock-type-face))
  "Face for the name a definition, package, namespace or alias introduces.")

(defface sysml-type-reference-face
  '((t :inherit font-lock-type-face))
  "Face for the type a feature is typed by, specializes or redefines.")

(defface sysml-namespace-face
  '((t :inherit font-lock-constant-face))
  "Face for a namespace segment of a qualified name, as in `ScalarValues::'.")

(defface sysml-quoted-name-face
  '((t :inherit font-lock-variable-name-face))
  "Face for a quoted name, such as `'left wheel''.")

(defface sysml-number-face
  '((t :inherit font-lock-number-face))
  "Face for numeric literals.")

(defface sysml-operator-face
  '((t :inherit font-lock-operator-face))
  "Face for symbolic operators.")

(defface sysml-escape-face
  '((t :inherit font-lock-escape-face))
  "Face for a backslash escape inside a string or quoted name.")

(defconst sysml--category-faces
  '((constant . sysml-constant-face)
    (declaration . sysml-declaration-keyword-face)
    (control . sysml-control-keyword-face)
    (modifier . sysml-modifier-keyword-face)
    (relationship . sysml-relationship-keyword-face)
    (operator . sysml-operator-keyword-face)
    (other . sysml-other-keyword-face))
  "The face each keyword category of `sysml-mode-keyword-groups' gets.")

;;;; Syntax

(defconst sysml-base-mode-syntax-table
  (let ((table (make-syntax-table)))
    ;; `//' starts a note to end of line (style a), `/* */' a block comment
    ;; (style b, so the two do not close each other).
    (modify-syntax-entry ?/ ". 124" table)
    (modify-syntax-entry ?* ". 23b" table)
    (modify-syntax-entry ?\n ">" table)
    (modify-syntax-entry ?\r ">" table)
    (modify-syntax-entry ?\" "\"" table)
    ;; A quoted name is a string to the syntax table, so its contents are never
    ;; read as code; font-lock gives it its own face.
    (modify-syntax-entry ?' "\"" table)
    (modify-syntax-entry ?\\ "\\" table)
    (modify-syntax-entry ?_ "_" table)
    (dolist (c '(?: ?> ?< ?= ?+ ?- ?% ?^ ?~ ?! ?& ?| ?? ?@ ?# ?$ ?. ?, ?\;))
      (modify-syntax-entry c "." table))
    table)
  "Syntax table for `sysml-base-mode'.")

(defconst sysml--syntax-propertize
  (syntax-propertize-rules
   ;; `//* ... */' is a multi-line note, which reads as a block comment once
   ;; its first slash is plain punctuation.  The lexer looks for `//*' before
   ;; `//', so this must win over the line-comment reading.
   ("\\(/\\)/\\*" (1 (unless (nth 8 (save-excursion (syntax-ppss (match-beginning 0))))
                       (string-to-syntax ".")))))
  "Function to apply syntax properties to the multi-line note opener.")

(defun sysml--syntactic-face (state)
  "Return the face for the string or comment described by STATE.
A quoted name is a string to the syntax table, but is a name."
  (cond
   ((eq (nth 3 state) ?') 'sysml-quoted-name-face)
   ((nth 3 state) 'font-lock-string-face)
   (t 'font-lock-comment-face)))

(defun sysml--match-note-opener (limit)
  "Find the next `//*' note opener before LIMIT, for font-lock.
Its first slash is punctuation to the syntax table, so the syntactic pass
leaves it unfaced; match it so it reads as part of the note."
  (let (found)
    (while (and (not found) (search-forward "//*" limit t))
      (let ((slash (- (point) 3)))
        (setq found (and (nth 4 (save-excursion (syntax-ppss (point))))
                         (not (nth 8 (save-excursion (syntax-ppss slash))))))
        (when found
          (set-match-data (list slash (1+ slash))))))
    found))

;;;; Font-lock

(defconst sysml--identifier "[A-Za-z_][A-Za-z0-9_]*"
  "Regexp matching an unquoted name.")

(defconst sysml--name
  (concat "\\(?:" sysml--identifier "\\|'\\(?:[^'\\\n]\\|\\\\.\\)*'\\)")
  "Regexp matching an unquoted or quoted name.")

(defun sysml--words (words)
  "Return a regexp matching any of WORDS as a whole symbol."
  (concat "\\_<" (regexp-opt words t) "\\_>"))

(defconst sysml--type-reference
  (concat "\\(::>\\|:>>\\|:>\\|:\\)[ \t]*"
          "\\(?:" sysml--identifier "::\\)*"
          "\\(" sysml--name "\\)")
  "Regexp matching a typing or specialization operator and the type after it.")

(defconst sysml--declaration-name
  (concat "\\_<" (regexp-opt sysml-mode-declaration-introducers)
          "\\_>[ \t]+\\(" sysml--name "\\)")
  "Regexp matching a declaration introducer and the name after it.")

(defun sysml--in-text-p (pos)
  "Return non-nil when POS is inside a comment, string or quoted name."
  (save-excursion (nth 8 (syntax-ppss pos))))

(defun sysml--matcher (regexp &optional accept)
  "Return a font-lock matcher for REGEXP that skips matches inside text.
A match starting in a comment or string is skipped, as is one ACCEPT,
called with the match data set, rejects."
  (lambda (limit)
    (let (found)
      (while (and (not found) (re-search-forward regexp limit t))
        (setq found (and (not (sysml--in-text-p (match-beginning 0)))
                         (or (null accept) (funcall accept))))
        (unless found
          (goto-char (1+ (match-beginning 0)))))
      found)))

(defconst sysml--match-declaration-name (sysml--matcher sysml--declaration-name)
  "Font-lock matcher for the name a declaration introduces.")

(defconst sysml--match-type-reference
  (sysml--matcher sysml--type-reference
                  ;; A bare `:' that is the second half of `::' types nothing.
                  (lambda () (not (and (string= (match-string 1) ":")
                                       (eq (char-before (match-beginning 1)) ?:)))))
  "Font-lock matcher for a typing or specialization and its type.")

(defun sysml--font-lock-keywords (language)
  "Return the font-lock keywords for LANGUAGE, `sysml' or `kerml'."
  (append
   '((sysml--match-note-opener (0 'font-lock-comment-delimiter-face)))
   ;; The name a declaration introduces, e.g. `part def Vehicle', and the type
   ;; after `:', `:>', `:>>' or `::>', e.g. `part engine : Engine'.  Both
   ;; override a quoted name's face: `'Left Wheel'' is a name, and this says
   ;; which kind.  Leading namespace segments get the namespace face below.
   `((,sysml--match-declaration-name (1 'sysml-declaration-name-face t))
     (,sysml--match-type-reference
      (1 'sysml-operator-face)
      (2 'sysml-type-reference-face t)))
   (mapcar (lambda (group)
             (let ((face (alist-get (car group) sysml--category-faces)))
               (unless face
                 (error "No face for keyword category %s" (car group)))
               `(,(sysml--words (cdr group)) (1 ',face))))
           sysml-mode-keyword-groups)
   `((,(sysml--words (alist-get language sysml-mode-contextual-words))
      (1 'sysml-contextual-keyword-face))
     ;; A namespace segment of a qualified name, e.g. `ScalarValues::Real'.
     (,(concat "\\_<\\(" sysml--identifier "\\)::")
      (1 'sysml-namespace-face))
     ;; Numbers, including the lexer's leading-dot reals such as `.5', which
     ;; go first: a match that is partly faced already is skipped whole.
     ("\\(?:^\\|[^.0-9A-Za-z_]\\)\\(\\.[0-9]+\\(?:[eE][+-]?[0-9]+\\)?\\)\\_>"
      (1 'sysml-number-face))
     ("\\_<[0-9]+\\(?:\\.[0-9]+\\)?\\(?:[eE][+-]?[0-9]+\\)?\\_>"
      (0 'sysml-number-face))
     (":>>\\|::>\\|:>\\|:=\\|=>\\|->\\|\\.\\.\\|\\*\\*\\|[=+*<>!&|^~?@%-]\\|/\\(?:[^/*]\\|$\\)"
      (0 'sysml-operator-face))
     ;; Escapes inside strings and quoted names.
     ("\\\\."
      (0 (when (nth 3 (syntax-ppss)) 'sysml-escape-face) prepend)))))

;;;; Indentation

;; The rule is the formatter's (internal/core/format/format.go): a line is
;; indented one level per brace open before it, and one level more when it
;; continues the statement above -- when the line break falls after a token
;; that cannot end a phrase, or before an infix token.

(defconst sysml--punctuation
  (sort (list "===" "!==" "::>" ":>>" "??" "==" "!=" "@@" "<=" ">=" ".." "**" "->"
              ".?" "::" ":=" ":>" "=>" "?" "|" "&" "@" "<" ">" "+" "-" "*" "/" "%"
              "^" "~" "." "#" "(" ")" "[" "]" "," "$" "=" "{" "}" ";" ":")
        (lambda (a b) (> (length a) (length b))))
  "The lexer's punctuation tokens, longest first.")

(defconst sysml--infix
  '("|" "&" "==" "!=" "===" "!==" "<" ">" "<=" ">=" ".." "+" "-" "*" "/" "%"
    "**" "^" "." "->" ".?" "," "::" "=" ":" ":=" ":>" ":>>" "::>" "=>")
  "Tokens joining two operands or clauses.
A line break on either side of one falls inside a phrase.  Mirrors
`infix' in the formatter.")

(defconst sysml--incomplete-before
  (append sysml--infix '("(" "[" "?" "??" "~" "@" "@@" "#" "$"))
  "Tokens that cannot end a phrase.  Mirrors `incompleteBefore' in the formatter.")

(defun sysml--punctuation-at (pos)
  "Return the punctuation token starting at POS, or nil if none does."
  (seq-find (lambda (tok)
              (string= tok (buffer-substring-no-properties
                            pos (min (point-max) (+ pos (length tok))))))
            sysml--punctuation))

(defun sysml--last-code-token ()
  "Return the last code token before point when it is punctuation, else nil.
Comments are skipped: a trailing comment interrupts nothing."
  (save-excursion
    (forward-comment (- (buffer-size)))
    (let ((end (point)))
      (skip-syntax-backward ".()")
      ;; A run of punctuation always starts at a token boundary, so lexing it
      ;; left to right, longest match first, finds its last token.
      (let (tok)
        (while (< (point) end)
          (setq tok (sysml--punctuation-at (point)))
          (forward-char (if tok (length tok) 1)))
        tok))))

(defun sysml--first-token-infix-p ()
  "Return non-nil when the line at point opens with an infix token."
  (and (not (looking-at-p "//\\|/\\*"))
       ;; `.5' is a number, not `.' followed by one.
       (not (looking-at-p "\\.[0-9]"))
       (member (sysml--punctuation-at (point)) sysml--infix)))

(defun sysml--continues-p ()
  "Return non-nil when the line at point carries on the statement above it.
Point is at the line's first non-blank character."
  (and (not (looking-at-p "[{}])]"))
       (or (sysml--first-token-infix-p)
           (member (sysml--last-code-token) sysml--incomplete-before))))

(defun sysml--brace-depth (ppss)
  "Return how many of the brackets open in PPSS are braces."
  (seq-count (lambda (pos) (eq (char-after pos) ?{)) (nth 9 ppss)))

(defun sysml--indentation ()
  "Return the column for the current line, or nil to leave it as written."
  (save-excursion
    (back-to-indentation)
    (let ((ppss (syntax-ppss)))
      ;; A multi-line comment or note is the author's text, kept verbatim.
      (unless (nth 8 ppss)
        (let ((depth (sysml--brace-depth ppss)))
          (when (looking-at-p "}")
            (setq depth (max 0 (1- depth))))
          (when (sysml--continues-p)
            (setq depth (1+ depth)))
          (* depth sysml-indent-offset))))))

(defun sysml-indent-line ()
  "Indent the current line as the language server's formatter would."
  (interactive)
  (let ((column (sysml--indentation)))
    (when column
      (if (<= (current-column) (current-indentation))
          (indent-line-to column)
        (save-excursion (indent-line-to column))))))

;;;; imenu

(defconst sysml--imenu-generic-expression
  `(("Packages"
     ,(concat "^[ \t]*\\(?:[a-z]+[ \t]+\\)*\\(?:package\\|namespace\\)[ \t]+\\("
              sysml--name "\\)")
     1)
    ("Definitions"
     ,(concat "^[ \t]*\\(?:[a-z]+[ \t]+\\)*\\([a-z]+[ \t]+def\\)[ \t]+\\("
              sysml--name "\\)")
     2))
  "Index of packages and definitions, for imenu without a language server.")

;;;; Modes

(define-derived-mode sysml-base-mode prog-mode "SysML-base"
  "Parent mode of `sysml-mode' and `kerml-mode'."
  :abbrev-table nil
  :syntax-table sysml-base-mode-syntax-table
  (setq-local syntax-propertize-function sysml--syntax-propertize)
  (setq-local comment-start "// ")
  (setq-local comment-end "")
  (setq-local comment-start-skip "\\(?://+\\*?\\|/\\*+\\)[ \t]*")
  (setq-local comment-end-skip "[ \t]*\\*+/")
  (setq-local comment-multi-line t)
  (setq-local indent-line-function #'sysml-indent-line)
  (setq-local indent-tabs-mode nil)
  (setq-local electric-indent-chars (append '(?{ ?} ?\;) electric-indent-chars))
  (setq-local imenu-generic-expression sysml--imenu-generic-expression)
  (setq-local font-lock-syntactic-face-function #'sysml--syntactic-face))

;;;###autoload
(define-derived-mode sysml-mode sysml-base-mode "SysML"
  "Major mode for editing SysML v2 models.

\\{sysml-mode-map}"
  (setq-local font-lock-defaults (list (sysml--font-lock-keywords 'sysml))))

;;;###autoload
(define-derived-mode kerml-mode sysml-base-mode "KerML"
  "Major mode for editing KerML models.

\\{kerml-mode-map}"
  (setq-local font-lock-defaults (list (sysml--font-lock-keywords 'kerml))))

;;;###autoload
(add-to-list 'auto-mode-alist '("\\.sysml\\'" . sysml-mode))
;;;###autoload
(add-to-list 'auto-mode-alist '("\\.kerml\\'" . kerml-mode))

(provide 'sysml-mode)

;;; sysml-mode.el ends here
