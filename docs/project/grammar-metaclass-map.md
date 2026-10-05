# Grammar-to-metaclass Map

## Overview

**Grammars:** `KerML.xtext`, `KerMLExpressions.xtext` and `SysML.xtext` from the pinned SysML v2 Pilot Implementation
**Provision:** `./scripts/download-pilot-grammars.sh`
**Run:** `go run -C tools ./cmd/grammar-metaclass-map` (writes `build/grammar-metaclass-map/`)
**Baseline:** [grammar-metaclass-map-baseline.json](grammar-metaclass-map-baseline.json), the compact production totals and findings
**Status:** advisory only — no CI gate uses these counts

This tool relates each grammar production to the metaclasses OpenSysML recognizes. It extracts
returns, type-setting actions, feature assignments, unassigned delegates, fragments, possible
created metaclasses and grammar anchors. It checks named types, assignment features and enum
literals against one OpenSysML workspace, then compares each located coverage citation's
reflective metaclass with the production's possible creates.

The differential inherits grammar coverage's literal-presence evidence, including its
position-agnostic search and first-occurrence citations. The source location is a heuristic:
the declaration header is the span from `DeclSpan.Offset` to `NameSpan.Offset`, or to the
first `;` or `{` for symbols without a name span. Relationships OpenSysML does not retain as
symbols, expression operators, imports and relationship keywords after a declaration name
therefore land in an undecided bucket rather than being reported as disagreements.

## Differential buckets

- **agree** — the reflective metaclass matches one of the production's possible creates.
- **disagree** — the located element has a metaclass absent from the production's creates,
  and no other production with the same anchor explains the citation.
- **undecided** — evidence cannot decide the comparison. The reason is exactly one of:
  `no-input`, `no-element`, `no-anchor`, `unlocated`, `no-element-at-anchor`,
  `no-reflective-metaclass` or `anchor-shared`.

## Current figures

The tables below are a snapshot of the branch point that wrote this page, not the current
baseline. The current figures are in [the baseline JSON](grammar-metaclass-map-baseline.json).

| Grammar | Productions | Rules | Fragments | Enums | Terminals | Defaulted | Datatypes | Feature checks found/missing | Enum literals found/missing |
|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|
| `KerML.xtext` | 176 | 112 | 61 | 3 | 0 | 1 | 2 | 244 / 0 | 7 / 0 |
| `KerMLExpressions.xtext` | 108 | 95 | 4 | 0 | 9 | 28 | 24 | 91 / 49 | 0 / 0 |
| `SysML.xtext` | 443 | 343 | 89 | 11 | 0 | 87 | 88 | 407 / 6 | 17 / 0 |
| **Total** | **727** | **550** | **154** | **14** | **9** | **116** | **114** | **742 / 55** | **24 / 0** |

| Grammar | Agree | Disagree | Undecided |
|---|---:|---:|---:|
| `KerML.xtext` | 26 | 5 | 145 |
| `KerMLExpressions.xtext` | 0 | 1 | 107 |
| `SysML.xtext` | 36 | 13 | 394 |
| **Total** | **62** | **19** | **646** |

Undecided reasons: `no-input` 244, `no-element` 229, `no-element-at-anchor` 73,
`no-anchor` 62, `anchor-shared` 36, `no-reflective-metaclass` 2, `unlocated` 0.
Reflective type statuses are 123 Ecore, 1,137 KerML, 412 SysML and 0 missing.

## Findings

The snapshot has 0 missing types, 55 missing assignment features, 0 missing enum literals,
and 0 defaulted non-datatype rules. The complete findings are listed in the baseline JSON.
They describe differences between the grammar and the reflective model; they do not change
parsing or validation behavior.

## Disagreements (observations)

Each row is a located input-presence citation whose OpenSysML metaclass differs from the
grammar's possible create. These are observations, not fixes.

| Production | Evidence location | OpenSysML metaclass | Grammar metaclass |
|---|---|---|---|
| `KerML.xtext::LibraryPackage` | `internal/workspace/libs/stdlib/Domain Libraries/Analysis/AnalysisTooling.sysml:1` | `KerML::Kernel::Package` | `SysML::LibraryPackage` |
| `KerML.xtext::TypeFeatureMember` | `internal/workspace/libs/stdlib/Kernel Libraries/Kernel Semantic Library/KerML.kerml:124` | `KerML::Core::Feature` | `SysML::OwningMembership` |
| `KerML.xtext::Subclassification` | `internal/workspace/libs/stdlib/Kernel Libraries/Kernel Semantic Library/Occurrences.kerml:729` | `KerML::Core::Specialization` | `SysML::Subclassification` |
| `KerML.xtext::FeatureValue` | `internal/workspace/libs/stdlib/Domain Libraries/Analysis/SampledFunctions.sysml:54` | `SysML::Systems::ReferenceUsage` | `SysML::FeatureValue` |
| `KerML.xtext::ReturnFeatureMember` | `internal/workspace/libs/stdlib/Domain Libraries/Analysis/SampledFunctions.sysml:54` | `SysML::Systems::ReferenceUsage` | `SysML::ReturnParameterMembership` |
| `KerMLExpressions.xtext::NamedArgument` | `internal/workspace/libs/stdlib/Domain Libraries/Analysis/SampledFunctions.sysml:54` | `SysML::Systems::ReferenceUsage` | `SysML::Feature` |
| `SysML.xtext::LibraryPackage` | `internal/workspace/libs/stdlib/Domain Libraries/Analysis/AnalysisTooling.sysml:1` | `KerML::Kernel::Package` | `SysML::LibraryPackage` |
| `SysML.xtext::VariantUsageMember` | `examples/sysml-v2-training/36. Variability/Variation Definitions.sysml:26` | `SysML::Systems::AttributeUsage` | `SysML::VariantMembership` |
| `SysML.xtext::FeatureValue` | `internal/workspace/libs/stdlib/Domain Libraries/Analysis/SampledFunctions.sysml:54` | `SysML::Systems::ReferenceUsage` | `SysML::FeatureValue` |
| `SysML.xtext::PerformActionUsage` | `internal/workspace/libs/stdlib/Systems Library/Actions.sysml:538` | `SysML::Systems::ActionUsage` | `SysML::PerformActionUsage` |
| `SysML.xtext::AcceptNode` | `internal/workspace/libs/stdlib/Kernel Libraries/Kernel Semantic Library/StatePerformances.kerml:143` | `KerML::Kernel::Succession` | `SysML::AcceptActionUsage` |
| `SysML.xtext::SendNode` | `examples/sysml-v2-training/21. Asynchronous Messaging/Messaging Example.sysml:31` | `SysML::Systems::ActionUsage` | `SysML::SendActionUsage` |
| `SysML.xtext::ExhibitStateUsage` | `examples/sysml-v2-training/26. State Exhibition/State Exhibition Example.sysml:8` | `SysML::Systems::StateUsage` | `SysML::ExhibitStateUsage` |
| `SysML.xtext::ReturnParameterMember` | `internal/workspace/libs/stdlib/Domain Libraries/Analysis/SampledFunctions.sysml:54` | `SysML::Systems::ReferenceUsage` | `SysML::ReturnParameterMembership` |
| `SysML.xtext::AssertConstraintUsage` | `internal/workspace/libs/stdlib/Domain Libraries/Analysis/SampledFunctions.sysml:40` | `SysML::Systems::ConstraintUsage` | `SysML::AssertConstraintUsage` |
| `SysML.xtext::SubjectUsage` | `internal/workspace/libs/stdlib/Domain Libraries/Analysis/TradeStudies.sysml:45` | `SysML::Systems::PartUsage` | `SysML::ReferenceUsage` |
| `SysML.xtext::ObjectiveMember` | `internal/workspace/libs/stdlib/Domain Libraries/Analysis/TradeStudies.sysml:146` | `SysML::Systems::RequirementUsage` | `SysML::ObjectiveMembership` |
| `SysML.xtext::IncludeUseCaseUsage` | `examples/sysml-v2-training/35. Use Cases/Use Case Usage Example.sysml:12` | `SysML::Systems::UseCaseUsage` | `SysML::IncludeUseCaseUsage` |
| `SysML.xtext::ViewRenderingMember` | `internal/workspace/libs/stdlib/OpenSysML Libraries/MOSA.sysml:305` | `SysML::Systems::RenderingUsage` | `SysML::ViewRenderingMembership` |

## Reproducing

```bash
./scripts/download-pilot-grammars.sh
./scripts/download-training-examples.sh
./scripts/download-pilot-corpora.sh
go run -C tools ./cmd/grammar-metaclass-map
go test -C tools -count=1 ./census/grammar/... ./census/metaclassmap/...

# refresh the compact baseline
go run -C tools ./cmd/grammar-metaclass-map -baseline docs/project/grammar-metaclass-map-baseline.json
```

The outputs are deterministic: grammars follow file-name order, productions follow declaration
order, lookup findings follow their grammar references, and summaries sort set-valued output.
Two runs against the same grammars and corpus produce byte-identical reports.

**No CI gate.** The map is advisory and does not claim parser execution, specification
conformance or a correctness score.
