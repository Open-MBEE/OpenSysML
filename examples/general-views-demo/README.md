# GeneralView graphs

`vehicle.sysml` declares a vehicle's structure, its requirements, the verification cases that
verify them, and four `GeneralView`s over them:

| View | Filter | Drawn as |
| --- | --- | --- |
| `GeneralViews::requirementView` | `filter @SysML::RequirementDefinition or @SysML::RequirementUsage;` | a requirement graph: ids, text, satisfy, verify, derive, refine, allocate, specialization and typing edges |
| `GeneralViews::definitionView` | `expose VehicleStructure::**[@SysML::Definition or @SysML::Usage];` | a definition and usage graph: specialization, typing, composition and reference edges |
| `GeneralViews::packageView` | `filter @SysML::Package;` | a package graph: containment and imports |
| `GeneralViews::plainView` | none | the containment tree |

```bash
sysml vehicle.sysml -render GeneralViews::requirementView -render-form mermaid
sysml vehicle.sysml -render GeneralViews::definitionView -render-form dot -render-style cameo
sysml vehicle.sysml -render GeneralViews::packageView -render-form plantuml
```

`-render-overlay verdicts` runs the verification cases and colours each requirement by its
worst verdict: `vehicleMass` passes the light vehicle's test and fails the heavy one's, and
`emergencyStop`'s test is inconclusive.

```bash
sysml vehicle.sysml -render GeneralViews::requirementView -render-form dot \
    -render-palette okabe-ito -render-overlay verdicts
```

In the REPL, `%render GeneralViews::requirementView dot verdicts` does the same. Which filters
select which graph is recorded in
[View rendering forms](../../docs/project/view-rendering-forms.md#generalview-graphs).

`use-cases.sysml` needs only the standard library. Its `UseCaseViews::useCaseView`
(`filter @SysML::UseCaseUsage;`) and `UseCaseViews::caseDefinitionView`
(`expose VehicleUseCases::*[@SysML::CaseDefinition];`) are GeneralViews drawn as case diagrams —
the subject, actors, objective and includes of each case — exactly as a `CaseView` exposing the
same elements draws them, which needs `OpenSysMLRenderings`:

```bash
sysml use-cases.sysml -render UseCaseViews::useCaseView -render-form plantuml
```
