# Specification Examples Gate

## Overview

**Corpus:** the textual-notation examples printed in the OMG SysML v2.1 Beta 2 and KerML 1.1 Beta 2
specifications themselves, extracted by `./scripts/download-spec-examples.sh` from the two PDFs of
release `2026-08`, commit `fb97b754f29588b8e9c7a35f370880cd15eb29e7` (`scripts/pilot-pin.sh`,
`PILOT_RELEASE_COMMIT`).

| Root | Source | Clauses | Directory | Files |
|---|---|---|---|---:|
| `sysml` | `doc/2a-OMG_Systems_Modeling_Language.pdf` | 7, 8.4 | `examples/spec-examples/sysml` | 341 `.sysml` |
| `kerml` | `doc/1-Kernel_Modeling_Language.pdf` and the `// KerML` examples of the SysML clauses | 7 | `examples/spec-examples/kerml` | 170 `.kerml` |

**Gate:** `TestSpecExamplesDiagnostics` in `tests/corpus/spec_examples_test.go`, a per-file ratchet
over `tests/corpus/testdata/spec_examples_expected.txt` (397 of the 511 files report at least one
diagnostic of ours). Run `go test -count=1 ./tests/corpus -run TestSpecExamples`.
**Regenerate:** `go test ./tests/corpus -run TestSpecExamplesDiagnostics -update-spec-examples`, after
adjudicating every moved line here.
**Required in CI:** `OPENSYSML_REQUIRE_SPEC_EXAMPLES=1` in both `.circleci/config.yml` and
`.github/workflows/pr.yml`; the gate skips when the corpus is absent and the variable unset.
**Referee:** the root `spec-examples` of `tools/referee/diff` (`go run -C tools ./cmd/pilot-diff`),
recorded in `docs/project/pilot-differential-baseline.json` and summarised in
[pilot-differential.md](pilot-differential.md); the grammar-coverage census also searches the root.

**Labels:** this is an engineering record. The short tags `SYSML21-nnn` and `KERML11-nnn` are the
resolution identifiers the Beta 2 documents print inline where an issue resolution changed the text; a
clause number alone names the subclause of the document the file's header comment cites.

## Why these examples

The examples are the OMG's own statement of what the notation must accept, so a disagreement between
an implementation and them is a specification-compliance finding by construction. They are not in any
other corpus: the training corpus and the three pilot corpora are separate material. They are also
fragments more often than not, which makes the completion rules below the whole question.

## Extraction rules

`tools/oracle/specexamples` reads the `pdftotext -layout` rendering of each PDF and nothing else, so
anyone with the PDF and poppler reproduces the corpus byte for byte (`TestExtractIsDeterministic`).

1. **Scope.** Blocks are taken from the clauses listed above; the page running headers (page number
   beside the document title) and the numbered subclause headings give each block its page and
   clause. Clauses 8.2 and 8.3 of SysML are grammar and OCL, clause 9 is the library listing, and the
   KerML clause 8 is likewise grammar: none of those contains a notation example to accept.
2. **A block** is a maximal run of layout lines indented by at most 12 columns whose first line opens a
   declaration (keyword, `//` or `/*` comment) and which closes when its braces balance at a line end.
   Prose lines (sentence case, no notation punctuation) before the first and after the last code line
   are trimmed; a prose line inside a block ends it. Lines indented deeper than 12 columns are the
   right-hand side of the graphical/textual notation tables and are never examples.
3. **KerML in SysML.** A block whose first comment line is `// KerML` is a KerML example printed in the
   SysML document and is written under `kerml/` with the SysML clause number.
4. **Excluded** blocks are recorded in `examples/spec-examples/manifest.txt` with the reason, which is
   one of: `abridged with ...` (the text prints an ellipsis), `unbalanced braces` (the example
   continues a previous one or is cut by the page), or a notation table. The run records 24 exclusions
   (20 SysML, 4 KerML); every one is an abridged or unbalanced fragment.

## Completion rules

The published bytes are never edited, following the errata-overlay principle
([errata-overlay.md](errata-overlay.md)): every completion is declared in the second header line of the
file and is one of the following, applied mechanically by `tools/oracle/specexamples/complete.go`.

| Completion | Rule | Files |
|---|---|---:|
| `wrapped in a package` | The example is placed in `package '<Document> <clause> example <n>' { ... }`, since every notation example is written as the body of some namespace. | 486 |
| `wrapped in an action definition` | Applied instead when a top-level statement of the example opens with an action-node keyword (`merge`, `decide`, `join`, `fork`, `send`, `accept`, `assign`, `terminate`, `if`, `while`, `loop`, `for`), a `then` target succession, or `first x;` with no `then`: SysML 8.2.2.17.1 admits those only as `ActionBodyItem`s, never as package members. | 25 |
| `imports <FQN>, ...` | For each undeclared bare name the resolver reports, a `private import Package::name;` is added when exactly one standard-library package *declares* a direct member of that name (a member visible through an import does not count); a member whose *short name* matches counts only when the reference follows `[`, the unit position of a quantity such as `0[kg]`. Names declared by more than one package (`size`, `min`, `sum`) stay unresolved. The OpenSysML extension libraries are never candidates. | 59 |

What the completion does **not** do, and therefore what the reporting counts contain: it does not
supply definitions the prose assumes from an earlier example (`Vehicle`, `Engine`, `Fuel`, ...), it
does not import a library package wholesale, and it does not reorder or split anything.

## Results

Pilot `2026-08` (`0.62.0`), both validators, fresh cache:

| Files | Fully agreeing | Ours | Pilot | Agreed | Severity-only | Only ours | Only pilot |
|---:|---:|---:|---:|---:|---:|---:|---:|
| 511 | 149 | 1089 | 3395 | 1037 | 2 | 50 | 2356 |

Of the pilot-only diagnostics, 1359 are the pilot's EMF `unmapped` records ("The feature 'type' of
... is required"), 587 its `kind-mismatch` typing constraints ("A usage must be typed by
definitions.") and 50 multiplicity constraints: all three follow an unresolved cross-example name on
the same lines, where we report the unresolved reference once and stop. Those 1996 are the same
finding counted differently, not disagreements about the examples. The remaining pilot-only
categories are 200 syntax, 156 unresolved-reference and 4 units; the 50 ours-only diagnostics are
38 unresolved-reference, 7 syntax, 3 kind-mismatch and 2 unmapped. Every file behind those two
groups is adjudicated below.

## Adjudication

Each row names the clause, the file, what the two implementations say, which side the text supports,
and the class: **erratum** (the printed example disagrees with the grammar or library of the same
document), **pilot** (the pilot departs from the text), **ours** (we do), **fragment** (the example
needs context the text does not print) or **cascade** (a secondary diagnostic of a fragment). A
`SYSML21-nnn` tag is given where the text itself carries one at the example.

### Printed examples the grammar does not admit (erratum)

| Clause | File | Example text | Grammar | Pilot | Ours | Class |
|---|---|---|---|---|---|---|
| SysML 8.4.13.4 | `sysml/8.4.13.4/1`–`9.sysml` | `succession s1 first a1[1..1] then j[1..1];` | `ConnectorEnd` (8.2.2.13.1) puts the multiplicity *before* the end name as `OwnedCrossMultiplicityMember`; nothing follows the reference | rejects (`no viable alternative at input 'then'`) | accepts silently | erratum; ours is also a gap, see below |
| SysML 7.13.2 | `sysml/7.13.2/8.sysml` | `connect ( [1] hub ::> mainSwitch[1], [1] device ::> sensorFeed )` | the same: `ConnectorEnd` (8.2.2.13.1) opens with the `OwnedCrossMultiplicityMember`; nothing follows the referenced feature | rejects (`missing ',' at '['`, `no viable alternative at input ','`) | rejects (`a connector end's multiplicity precedes it … nothing follows that feature`, once, at the `[1]` after `mainSwitch`) | erratum, both right |
| SysML 8.4.8.2, 8.4.18.1, 8.4.18.2, 8.4.20.2, 8.4.21.2 | `sysml/8.4.8.2/1`, `8.4.18.1/1`, `8.4.18.2/3`, `8.4.20.2/1`, `8.4.21.2/1.sysml` | `case cs2 specializes Cases::cases { ... }` | a usage specialises with `subsets`/`:>` (8.2.2.5.3 `FeatureSpecialization`); `specializes` is `OwnedSpecialization` of a definition | rejects (`no viable alternative at input 'specializes'`) | rejects (`only a definition may specialize; found a usage`) | erratum, both right |
| SysML 8.4.19.1, 8.4.19.2, 8.4.20.1 | `sysml/8.4.19.1/1`, `8.4.19.2/1`, `8.4.20.1/1.sysml` | `analysis case def AC specializes ...`, `verification case V specializes ...` | the keywords are `analysis def` (8.2.2.22) and `verification def` (8.2.2.23); the text writes the metaclass name | rejects (`no viable alternative at input 'case'`) | accepts the keyword, then rejects `specializes` on the usage | erratum; ours also a gap (accepts `analysis case def`) |
| SysML 8.4.15.1, 8.4.15.2 | `sysml/8.4.15.1/2`, `8.4.15.2/3.sysml` | `binding result = resultExpr.result; // Implied` followed by a bare `resultExpr` | `binding` is KerML; SysML writes `bind` (8.2.2.13.2); the trailing name is a placeholder | rejects (`no viable alternative at input 'binding'`) | rejects (`unresolved reference: resultExpr`) | erratum (semantics illustration, not notation) |
| SysML 7.21.2 | `sysml/7.21.2/2.sysml` | `constraint massIsPositive { attribute mass : MassValue; mass > 0[kg]; }` | `CalculationBodyPart` (8.2.2.19.1) ends with a `ResultExpressionMember`, which has no `;` | rejects (`extraneous input ';' expecting '}'`) | accepts silently | erratum; ours also a gap |
| SysML 7.18.3, 8.4.14.3 | `sysml/7.18.3/4.sysml`, `sysml/8.4.14.3/1.sysml` | `transition off_on first off if isEnabled accept TurnOn via commPort do action ... then on;` | `TransitionUsage` (8.2.2.18.1) orders trigger (`accept`) before guard (`if`) before effect (`do`) | rejects | rejects (clause order) | erratum, both right |
| SysML 8.4.13.9 | `sysml/8.4.13.9/1.sysml` | `action if1 if test1 then action then1 { ... }` | `IfNode` (8.2.2.17.7) has no `then`: the body follows the expression directly | rejects | rejects (`expected a name`) | erratum, both right |
| SysML 8.4.14.3 | `sysml/8.4.14.3/2.kerml` | `alias s1 of SC::s1;` inside a `step` body | `AliasMember` (KerML 8.2.3.3.1) is `'alias' ( '<' NAME '>' )? NAME? 'for' [QualifiedName] RelationshipBody`, and `TypeBodyElement` (8.2.4.1.1) admits it: the member form is right, the keyword is `for` | rejects (`mismatched input 'of' expecting 'for'`) | rejects (`expected 'for' in alias`), then reads the alias and the rest of the body | erratum, both right |
| KerML 7.3.4.8 | `kerml/7.3.4.8/1.kerml` | `featuring engine_by_Vehicle of engine featured by Vehicle;`, `featuring power featured by engine { doc /* ... */ }` | `TypeFeaturing` (8.2.4.3.7) is `'featuring' ( Identification 'of' )? [QualifiedName] 'by' [QualifiedName] RelationshipBody`: the member form spells `by` alone, as the 7.3.4.8 prose does not; the relationship body is admitted | rejects (`extraneous input 'featured' expecting 'by'`) | rejects (`expected 'by' ... not 'featured by'`), then reads the relationship and its body | erratum, both right |
| KerML 7.2.4.3, SysML 7.4.3 | `kerml/7.2.4.3/1.kerml`, `sysml/7.4.3/1.sysml` | `behavior setX(c : C, newX : Real) { language "alf" /* ... */ }`, `action def setX(c : C, newX : Real) { ... }` | no production puts a parenthesised parameter list on a declaration; parameters are `ParameterMember`s of the body (KerML 8.2.4.4.2, SysML 8.2.2.5.2). The rep-less `TextualRepresentation = ( 'rep' Identification )? 'language' STRING_VALUE REGULAR_COMMENT` (KerML 8.2.3.3.3) is admitted in both notations | rejects (`no viable alternative at input '('`) | rejects (`a declaration takes no parenthesised parameter list`), then reads the body with its textual representation | erratum, both right |
| SysML 8.4.14.3 | `sysml/8.4.14.3/2.kerml` | `alias s1 of SC::s1;` inside a `step` body | `AliasMember` (KerML 8.2.3.3.1) is `'alias' ( '<' NAME '>' )? NAME? 'for' [QualifiedName] RelationshipBody`, and `TypeBodyElement` (8.2.4.1.1) admits it: the member form is right, the keyword is `for` | rejects (`mismatched input 'of' expecting 'for'`) | rejects (`expected 'for' in alias`), then reads the alias and the rest of the body | erratum, both right |
| KerML 7.3.4.8 | `kerml/7.3.4.8/1.kerml` | `featuring engine_by_Vehicle of engine featured by Vehicle;`, `featuring power featured by engine { doc /* ... */ }` | `TypeFeaturing` (8.2.4.3.7) is `'featuring' ( Identification 'of' )? [QualifiedName] 'by' [QualifiedName] RelationshipBody`: the member form spells `by` alone, as the 7.3.4.8 prose does not; the relationship body is admitted | rejects (`extraneous input 'featured' expecting 'by'`) | rejects (`expected 'by' ... not 'featured by'`), then reads the relationship and its body | erratum, both right |
| KerML 7.2.4.3, SysML 7.4.3 | `kerml/7.2.4.3/1.kerml`, `sysml/7.4.3/1.sysml` | `behavior setX(c : C, newX : Real) { language "alf" /* ... */ }`, `action def setX(c : C, newX : Real) { ... }` | no production puts a parenthesised parameter list on a declaration; parameters are `ParameterMember`s of the body (KerML 8.2.4.4.2, SysML 8.2.2.5.2). The rep-less `TextualRepresentation = ( 'rep' Identification )? 'language' STRING_VALUE REGULAR_COMMENT` (KerML 8.2.3.3.3) is admitted in both notations | rejects (`no viable alternative at input '('`) | rejects (`a declaration takes no parenthesised parameter list`), then reads the body with its textual representation | erratum, both right |
| SysML 8.4.13.5 | `sysml/8.4.13.5/1.sysml` | `action snd send p via src to tgt;` | `SendNode` (8.2.2.17.5) reads `OccurrenceUsagePrefix ActionUsageDeclaration? 'send'`, with no `action` keyword, while `SendNodeDeclaration` in the same clause reads `ActionNodeUsageDeclaration? 'send'`, which has one | rejects (`no viable alternative at input 'send'`) | accepts | grammar text inconsistent; ours follows `SendNodeDeclaration`, the form `AcceptNode` and `TerminateNode` use |

### The pilot departs from the text (pilot)

| Clause | File | Example text | Grammar | Pilot | Ours | Class |
|---|---|---|---|---|---|---|
| SysML 7.17.11 | `sysml/7.17.11/1.sysml` | `action test if speed < lowerLimit action increase : IncreaseSpeed { } else action main : MaintainSpeed { }` | `IfNode = ActionNodePrefix 'if' ...` and `ActionNodePrefix = OccurrenceUsagePrefix ActionNodeUsageDeclaration?` with `ActionNodeUsageDeclaration = 'action' UsageDeclaration?` (8.2.2.17.2, 8.2.2.17.7) | rejects (`no viable alternative at input 'if'`) | accepts; reports `lowerLimit` unresolved (fragment) | pilot |
| SysML 7.17.12 | `sysml/7.17.12/1.sysml` | `action advance while t < endTime { ... }` | `WhileLoopNode = ActionNodePrefix ( 'while' ... )`, as above | rejects (`no viable alternative at input 'while'`) | accepts; unresolved `t`, `endTime` (fragment) | pilot |
| SysML 7.26.2 | `sysml/7.26.2/2`, `3.sysml` | `filter not @SysML::ConnectionUsage;` | KerML 8.2.5.8.7 `ClassificationExpression` admits the unary `@ Type` form with the implicit `self` operand | rejects (`no viable alternative at input '@'`) | accepts | pilot |
| SysML 7.24.2, 7.26.2 | `sysml/7.24.2/1.sysml`, `sysml/7.26.2/1.sysml` | `import VerificationCases::*;` / `import Views::*;` as the first member of a definition body | `DefinitionBodyItem` (8.2.2.5.2) admits `Import`; the examples write it without a visibility keyword, as the training corpus does at package level | rejects (`no viable alternative at input '::'`) | accepts | pilot (the pilot's own corpora accept the same form at package level) |
| KerML 7.4.13 | `kerml/7.4.13/8.kerml` | `@Command; // This is INVALID.` | the text itself says the usage is invalid (a metadata annotation on nothing) | silent | rejects (`expected a namespace member`) | pilot (ours agrees with the text) |

### We depart from the text (ours)

| Clause | File | Example text | Grammar | Pilot | Ours | Class |
|---|---|---|---|---|---|---|
| KerML 7.4.9.3 | `kerml/7.4.9.3/1`–`3.kerml` | `sensors.{in s: Sensor; s.reading}` as a namespace member | an expression is not a `NamespaceMember` (8.2.2.3); the text shows the expression alone | rejects (`no viable alternative at input '}'`) | silent | ours (we accept an expression statement as a member) |
| SysML 7.17.4 | `sysml/7.17.4/7.sysml` (before the `then` rule above was added) | `then join1;` | `ActionTargetSuccessionMember` is an `ActionBodyItem` only (8.2.2.17.1) | rejects | silent at package level | ours (the completion now wraps it; the parser still accepts the form at package level) |
| SysML 8.4.13.8 | `sysml/8.4.13.8/1.sysml` | `action trm terminate occ;` as a package member | `TerminateNode` is an `ActionNode`, an `ActionBodyItem` only (8.2.2.17.1) | rejects (`no viable alternative`) | reports only the unresolved `occ` | ours (the same gap: an action node accepted at package level) |
| SysML 8.4.13.4, 7.21.2, 8.4.19.x | see the erratum table | `a1[1..1]` after an end name; `;` after a result expression; `analysis case def` | as above | rejects | silent | ours (accepted where the grammar has no production) |

### Fragments and cascades

The remaining files that differ are fragments: the text assumes definitions from an earlier example
or from prose (`Vehicle`, `Engine`, `Fuel`, `ComputeDynamics`, `vehicleMassRequirement`, `resultExpr`,
`f`, `g`, `x`, `t`, `endTime`, `lowerLimit`, `monitor`, `TimeoutSignal`, `UserAction`, `Pd`, `s1`, `val`,
`p`/`src`/`tgt`), both implementations report the unresolved name, and the pilot adds its typing and
EMF cascade on the same lines. Three cases deserve a note:

| Clause | File | What differs | Class |
|---|---|---|---|
| SysML 7.17.4 | `sysml/7.17.4/10.sysml` | The example begins `then decide decision1;`, continuing example 9 on the previous lines; we report that the leading `then` has nothing to sequence from, the pilot accepts it. | fragment (ours reads the text correctly) |
| SysML 7.17.5 | `sysml/7.17.5/1.sysml` | `isActive` is unresolved in both; our suggestion names `StateActivity::isActive` because that extension library is loaded, which the pilot has no counterpart for. | fragment (suggestion text only) |
| KerML 7.4.9.4 | `kerml/7.4.9.4/2.kerml` | The pilot reports `Must invoke a behavior or a behavioral feature` on an invocation of a feature the fragment leaves undeclared. | cascade |

### Erratum resolutions in the text

The Beta 2 documents print resolution tags at corrected examples. The extracted files carry the
corrected text, and the tags that fall inside clause 7 and 8.4 examples (among them `SYSML21-2`,
`SYSML21-314`, `SYSML21-413`, `SYSML21-477`) are kept as the comment lines they are printed as; none
of the disagreements above is at a tagged example, so every erratum class listed here is a printed
example the resolutions did not reach.

## Counts

| | SysML | KerML | Total |
|---|---:|---:|---:|
| Fixtures | 341 | 170 | 511 |
| Excluded blocks | 20 | 4 | 24 |
| Files reporting a diagnostic of ours | 261 | 136 | 397 |
| Files fully agreeing with the pilot | | | 149 |
| Files in the erratum table | 27 | 2 | 29 |
| Files in the pilot table | 6 | 1 | 7 |
| Files in the ours table | 2 | 3 | 5 |

## Launch-ready gaps

Each row is self-contained: clause, the extracted file that shows it, the behaviour the text requires,
and a probe to reproduce it. The ratchet pins the current verdicts so each fix moves exactly its own
lines. The in-flight column records whether other work on the same rule was found open at the time of
writing.

Closed: G1 (a cross feature before the `ref` of an end usage; `sysml/8.4.9.1/4.sysml` has no syntax
diagnostic, while the `mainSwitch[1]` of `sysml/7.13.2/8.sysml` is the erratum above, rejected by both
implementations), G2 (a terminate action usage naming the occurrence it terminates), G3 (an accept
payload with a value part, a `TriggerWhen`/`TriggerAt`/`TriggerAfter` invocation waited on as the
keyword form), G4 (`ordered`/`nonunique` without a bracketed multiplicity) and G5 (a directed
calculation parameter whose body ends in a result expression). Their six files now report only the
names the examples leave undeclared, which the pilot reports too (`sysml/7.17.8/2.sysml`, once it parsed,
received the completion imports for `ChangeSignal`, `Real`, `s` and `Iso8601DateTime`); `sysml/8.4.13.8/1.sysml` moved from
a syntax error given for the wrong reason to the package-level action node of G10.
G6, G7 and G8 (an `alias` in a type body, a relationship body on a `featuring`, a `language`
representation without `rep`) were not gaps: the grammar productions are implemented, and what the printed
examples add (`alias ... of`, `featured by` on the member form, a parenthesised parameter list) no production
admits and the pilot rejects, so they sit in the erratum table above; the later labels keep their numbers.

| # | Clause | File | Expected behaviour | Probe | In-flight overlap |
|---|---|---|---|---|---|
| G9 | KerML 8.2.2.3 `NamespaceMember` | `kerml/7.4.9.3/1`–`3.kerml` | A bare expression such as `sensors.{in s: Sensor; s.reading}` is not a namespace member and is reported as a syntax error, as the pilot does. | `sysml -validate`: a `syntax` diagnostic on line 4; today the files are clean. | none found |
| G10 | SysML 8.2.2.17.1 `ActionBodyItem` | `sysml/7.17.4/7.sysml` with the package wrapper, `sysml/8.4.13.8/1.sysml` | `then join1;` directly in a package is a syntax error, since target successions exist only in action bodies. | wrap the example in a package by hand and run `sysml -validate`: a `syntax` diagnostic; today it is clean. | none found |
| G11 | SysML 8.2.2.13.1 `ConnectorEnd` | `sysml/8.4.13.4/1`–`9.sysml` | A multiplicity *after* an end reference (`first a1[1..1] then j[1..1]`) has no production and is reported, as the pilot does; the project's nonstandard-notation policy decides whether as an error or a warning. | `sysml -validate`: a diagnostic on each succession line; today the files are clean. | none found |
| G12 | SysML 8.2.2.19.1 `ResultExpressionMember` | `sysml/7.21.2/2.sysml` | A `;` after a body's result expression (`mass > 0[kg];`) is reported, as the pilot does. | `sysml -validate`: a diagnostic on line 9; today the file is clean. | none found |
| G13 | SysML 8.2.2.22 / 8.2.2.23 keywords | `sysml/8.4.19.1/1`, `8.4.19.2/1`, `8.4.20.1/1.sysml` | `analysis case def` / `verification case def` are not SysML keywords (`analysis def`, `verification def` are) and are reported, as the pilot does. | `sysml -validate`: a `syntax` or `nonstandard-notation` diagnostic on the definition line; today only the usage `specializes` is reported. | none found |
| G14 | KerML 7.4.13 metadata examples; SysML 7.27.2 | `kerml/7.4.13/3`, `11`, `12`, `14`, `15.kerml`, `sysml/7.27.2/3.sysml` | The document interface record (`tests/model/interface_record_test.go`) is derivable for every file of the root: today `provisional supertypes of ... Save` refuses four files whose metadata definitions specialise an undeclared name, and two files are "written among other documents". Until then the root is left out of `corpusRoots` in `tests/model`, so the incremental and interface-record differentials do not run over it. | `go test ./tests/model -run TestInterfaceRecordDifferentialCorpora` with the root added to `corpusRoots`: 6 failures today. | none found |

The sweep for in-flight work covered the open pull requests and the remote branches without one that
touch `internal/syntax/parser`, `internal/check` and the corpus gates; none of them addresses the
productions above.
