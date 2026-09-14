# Verdicts demo: what holds of an object, and of everything it holds

[`rover.sysml`](rover.sysml) is a small surface rover written for one question:
**which of the assertions about this object hold right now — of the object itself,
of each part it holds and of each member of a collection it holds — and for each
that does not, why?** The document query `Verdicts(...)` answers it as a table,
over the object as the model declares it and over the object a session holds
after it has driven.

The rover carries seven assertions of the four kinds a verdict row can have
(`constraint`, `requirement`, `satisfaction`, `verification`), placed so that one run
of the model touches all three outcomes. The transcripts below quote the lines that
carry the point; the binary also prints each projected cell under its row.

| Assertion | Where it sits | What decides it |
| --- | --- | --- |
| `assert constraint underLimit { mass <= 1000.0 }` | the rover | a value the rover holds |
| `requirement drivable { require constraint { soc > 30.0 } }` | the rover | `soc` bound to `battery.charge` through a nested part |
| `assert constraint aboveFloor { charge >= floor }` | the battery | the rover's battery redefines `floor` to `30.0`; the definition's default is `20.0` |
| `satisfy powerMargin by rover.battery` | the battery | `PowerMargin`, a requirement stated once over a `Battery` subject |
| `verification marginCheck` | the battery's requirement | `VerificationCases::PassIf(...)` over the subject the case binds |
| `assert constraint treadLeft { wear < 0.5 }` | each of six wheels | one row per wheel, `wheels[1]` to `wheels[6]` |
| `assert constraint warmEnough { output >= setpoint }` | the heater | `setpoint` is never valued — undecidable, not false |

Three queries read the verdicts, and one document lays them out:

```sysml
calc def Checks :> Query {          // every assertion, one row each
    in root : Element;
    Project(source = Verdicts(source = root),
            properties = ("path", "kind", "name", "verdict", "reason"))
}
calc def Findings :> Query {        // what does not hold, violated before undecided
    in root : Element;
    OrderBy(source = WhereFeature(source = Verdicts(source = root),
                                  'feature' = "verdict", operator = "!=", value = "holds"),
            property = "verdict", direction = "descending", missing = "last", multiple = "error")
}
calc def Verified :> Query {        // requirements and satisfactions with their verification outcomes
    in root : Element;
    Project(source = WhereFeature(source = Verdicts(source = root),
                                  'feature' = "kind", operator = "!=", value = "constraint"),
            properties = ("path", "kind", "name", "verdict", "verification"))
}
```

Build the binary once from the repository root:

```bash
make build-sysml
```

## 1. The rover as declared

Without a session, a parameter bound to `rover` binds the model element, and
`Verdicts` checks its *declared* object: the definition defaults, the usage's
redefinitions, nothing having run.

```bash
./bin/sysml examples/verdicts-demo/rover.sysml -run-query "RoverChecks::Checks root=rover"
```

```text
✓ Query RoverChecks::Checks returned 12 rows
  Columns: path, kind, name, verdict, reason
  Row 1: assert constraint underLimit on RoverChecks::rover: holds
  Row 2: requirement drivable on RoverChecks::rover: holds
  Row 3: assert constraint aboveFloor on RoverChecks::rover.battery: holds
  Row 4: satisfy powerMargin by rover.battery on RoverChecks::rover.battery: holds
  Row 5: verification RoverChecks::marginCheck on RoverChecks::rover.battery: holds
  Row 6: assert constraint treadLeft on RoverChecks::rover.wheels[1]: holds
  ...
  Row 11: assert constraint treadLeft on RoverChecks::rover.wheels[6]: holds
  Row 12: assert constraint warmEnough on RoverChecks::rover.heater: undecided
```

Twelve rows from one binding: the rover's own two assertions, the battery's
constraint, the satisfaction and the verification case about it, six wheels the
multiplicity `Wheel[6]` materialized, and the heater. The heater's row is
`undecided`, with the reason in its `reason` cell — `no value for feature
setpoint` — because a constraint whose operand has no value has not been
violated; it has not been decided.

## 2. The same query over the object a session holds

In the REPL, `%instantiate rover` makes the session hold a rover, and from then
on the name `rover` in a query binding means that object, not the element.

```text
./bin/sysml
%load examples/verdicts-demo/rover.sysml
%instantiate rover
%run-query Findings root=rover
```

```text
✓ Query RoverChecks::Findings returned 1 row
  Row 1: assert constraint warmEnough on RoverChecks::rover.heater: undecided
```

Fresh from instantiation the held rover agrees with the declared one. Now drive
it and wear one wheel:

```text
%invoke rover drive km=40
%invoke rover.wheels[3] roll km=60
%run-query Findings root=rover
```

```text
✓ Query RoverChecks::Findings returned 3 rows
  Row 1: satisfy powerMargin by rover.battery on RoverChecks::rover.battery: violated
  Row 2: assert constraint treadLeft on RoverChecks::rover.wheels[3]: violated
  Row 3: assert constraint warmEnough on RoverChecks::rover.heater: undecided
```

The drive took the battery from `100.0` to `40.0`. Its own `aboveFloor`
(`40 >= 30`) still holds and the rover's `drivable` (`40 > 30`) still holds, but
the satisfaction of `powerMargin` (`40 - 30 >= 25`) no longer does, and only the
third wheel's `treadLeft` fails. `Findings` orders violations before the
undecided heater.

`Verified` shows the requirement-level rows with the outcome of the cases
verifying each:

```text
%run-query Verified root=rover
```

```text
✓ Query RoverChecks::Verified returned 3 rows
  Columns: path, kind, name, verdict, verification
  Row 1: requirement drivable on RoverChecks::rover: holds
    verification = (none)
  Row 2: satisfy powerMargin by rover.battery on RoverChecks::rover.battery: violated
    verification = "pass"
  Row 3: verification RoverChecks::marginCheck on RoverChecks::rover.battery: holds
    verification = "pass"
```

Row 2 is the point of the column: the requirement's verification case
**passed** while the satisfaction is **violated**. A verification case runs its
own body over the subject it binds — `subject b = rover.battery`, the battery as
designed, fully charged — so `marginCheck` reports on the design, and the
satisfaction row reports on the object as it stands after the drive. Both are
true; they answer different questions, and the table keeps them apart.

`%validate rover` is the same check reported as a REPL verdict list rather than
query rows; the two agree row for row.

## 3. The document

`RoverReport` binds `root = rover` in the model and lays the three queries out
as a table, a list and a table. Rendered over the held rover after the drive:

```text
%render-document RoverReport
```

```markdown
| path | kind | name | verdict | reason |
| --- | --- | --- | --- | --- |
| RoverChecks::rover | constraint | underLimit | holds |  |
| RoverChecks::rover | requirement | drivable | holds |  |
| RoverChecks::rover.battery | constraint | aboveFloor | holds |  |
| RoverChecks::rover.battery | satisfaction |  | violated | satisfaction satisfy powerMargin by rover.battery: require condition evaluated to false: b.charge - b.floor >= 25.0 |
| RoverChecks::rover.battery | verification | marginCheck | holds |  |
| RoverChecks::rover.wheels\[1\] | constraint | treadLeft | holds |  |
| RoverChecks::rover.wheels\[2\] | constraint | treadLeft | holds |  |
| RoverChecks::rover.wheels\[3\] | constraint | treadLeft | violated | constraint treadLeft: assertion evaluated to false: wear \< 0.5 |
...
| RoverChecks::rover.heater | constraint | warmEnough | undecided | constraint warmEnough: assertion evaluation failed: no value for feature setpoint |

## Findings

- satisfy powerMargin by rover.battery on RoverChecks::rover.battery: violated
- assert constraint treadLeft on RoverChecks::rover.wheels\[3\]: violated
- assert constraint warmEnough on RoverChecks::rover.heater: undecided
```

From the command line, `-instantiate` puts the held object behind the same
document, and `-doc-form html` renders it as a page:

```bash
./bin/sysml examples/verdicts-demo/rover.sysml -render-document RoverChecks::RoverReport -o rover-declared.md
./bin/sysml examples/verdicts-demo/rover.sysml -instantiate RoverChecks::rover \
    -render-document RoverChecks::RoverReport -doc-form html -o rover.html
```

The first renders the declared rover (every row `holds` but the heater); the
second the held one, fresh from instantiation. A document rendered inside a
session after `%invoke` sees the state the invocations left, as above.

## What a verdict table sees that an expression evaluator does not

`%eval` evaluates one expression in one context and returns one value:

```text
%eval in rover : battery.charge >= battery.floor
  = true
%eval in rover : heater.output >= heater.setpoint
error: evaluation failed: no value for feature heater.setpoint
```

Both answers are correct, and neither is the check. Compare what
`Verdicts(source = rover)` had to do to produce the twelve rows above:

1. **Find the assertions.** Nobody wrote the list. The sweep collects every
   `assert constraint` the rover's type declares or inherits, the requirement
   usage it carries, every `satisfy` whose subject is in the tree, and the
   verification cases whose objective verifies a requirement it found. Add an
   assertion to the model and the table grows; an evaluator only checks what
   you typed.
2. **Walk the object graph.** The rover holds a battery, six wheels and a
   heater; each is checked against *its own* type's assertions, and the row
   names the object by path — `rover.wheels[3]`, not "a wheel". The six wheel
   rows come from a multiplicity, and had `Wheel` held parts of its own they
   would be walked too. A walk that cannot finish is a typed error, not a
   shorter table.
3. **Evaluate against the right object.** `aboveFloor` reads `floor` as
   `30.0` on the rover's battery because the usage redefines it, not the
   definition's `20.0`; `drivable`'s `soc` is bound through a nested part;
   `PowerMargin`'s `b` is bound to the satisfying battery. Each assertion is
   evaluated on its carrier with its own bindings, without the reader having
   to spell any of them out.
4. **Distinguish false from undecided.** `wear < 0.5` on `wheels[3]` is
   `violated`; `output >= setpoint` on the heater is `undecided`, with the
   missing feature named. An evaluator returns `false` for one and an error
   for the other, and a script that treats the error as a failure — or skips
   it — has already misreported the heater.
5. **Carry the reason.** Every non-holding row says what was evaluated to
   what: the condition text, the object it was evaluated on, the feature that
   had no value. That is the `reason` column; a boolean has none.
6. **Include requirements and their verification.** `satisfy powerMargin`
   is a relationship, not an expression: its verdict comes from binding the
   requirement's subject to the battery and evaluating the requirement's
   `require` conditions there. The `verification` cell is a third source
   again — the `VerdictKind` the verifying case's body returned when run — and
   the table shows the satisfaction *violated* beside a verification that
   *passed*, which a single expression cannot express.
7. **Answer as rows.** The result is an ordered, typed row set with `path`,
   `kind`, `name`, `verdict`, `condition`, `reason` and `verification`
   properties, so `WhereFeature`, `OrderBy` and `Project` filter and shape it
   and a `Table` or `List` renders it. `Findings` is one `WhereFeature` away
   from `Checks`; it needed no second sweep, and nothing to be re-typed.
8. **Compare declared with held.** The same query, bound to the same name,
   answers over the design (section 1) or over the object a session holds after
   a run (section 2). Nothing in the query changes — only what `rover` is bound
   to.

An expression evaluator is what the verdict table is *built from*: every
`verdict` cell is one evaluation. What the table adds is knowing which
expressions to evaluate, on which objects, under which bindings, and what to
say when one of them cannot be evaluated at all.

## Where the pieces are

- The `Verdicts` operation and the `Verdict` properties:
  [`docs/manual/query-cookbook.md`](../../docs/manual/query-cookbook.md#which-constraints-and-requirements-hold).
- Object bindings in `%run-query`, `-run-query` and `-render-document`, and
  `-instantiate` with a document:
  [`docs/reference/cli.md`](../../docs/reference/cli.md),
  [`docs/reference/repl-commands.md`](../../docs/reference/repl-commands.md).
- `%validate` / `-validate=<object>`, the same sweep as a REPL report:
  [`docs/guide/05-checking.md`](../../docs/guide/05-checking.md).
