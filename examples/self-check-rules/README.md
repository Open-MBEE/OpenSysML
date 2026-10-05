# Self-check rules: your own constraints in `-self-check`

`-self-check` applies the bundled `SysMLValidation` constraints to every
reflectively classified element of a model. `-self-check-package <QualifiedName>`
adds the `constraint def`s of a package your own files declare to the same walk —
the bundled rules still run, the flag implies `-self-check`, and it is repeatable.
A rule file is an ordinary input, named beside the model.

[`rules.sysml`](rules.sysml) is such a file: `Acme::ModelingRules` states two
house rules, "every `PartDefinition` has documentation" and "every `PortUsage`
declares a direction". A rule is a `constraint def` whose first `in` parameter is
typed by a `SysML::…` or `KerML::…` metaclass — `SysML::PartDefinition`,
`SysML::PortUsage` — and whose body is a Boolean expression over the element's
reflective features (`name`, `qualifiedName`, `documentation`, `ownedMember`,
`isComposite`, …). It applies to every element the metaclass conforms to. The
bundled [SysMLValidation](../../internal/workspace/libs/stdlib/OpenSysML%20Libraries/SysMLValidation.sysml)
package is written the same way and is the reference.

[`rover.sysml`](rover.sysml) is a small rover where every port declares a
direction and one part def, `SensorBus`, carries no `doc`.

Build the binary once from the repository root:

```bash
make build-sysml
```

## Running the rules

```bash
./bin/sysml examples/self-check-rules/rover.sysml examples/self-check-rules/rules.sysml -self-check-package Acme::ModelingRules
```

```text
✓ package Rover
✓ package Acme
examples/self-check-rules/rover.sysml:10:2: error: Acme::ModelingRules::partDefinitionHasDocumentation fails for Rover::SensorBus
Self-model check: 10 elements, 60 checks, 1 violations, 0 unevaluated
```

Exit status 1: one of the 60 constraint applications failed, named by the rule's
qualified name and the element it rejected. Each element of a rule package is
checked by nothing — `Acme` declares rules, not a model — so the element count
covers `rover.sysml`'s declarations plus the enclosing `Acme` package only.

The bundled checks alone find nothing wrong with the rover:

```bash
./bin/sysml examples/self-check-rules/rover.sysml -self-check
```

```text
✓ package Rover
Self-model check: 9 elements, 55 checks, 0 violations, 0 unevaluated
```

Exit status 0.

A package name that declares nothing is refused before anything is evaluated
(exit 2); a rule whose first `in` parameter is not metaclass-typed is skipped
with a warning and the run goes on, as is a package that yields no applicable
constraint at all. A rule the reflective model cannot evaluate — a feature it
does not derive, say — is counted as unevaluated, not as a violation.
