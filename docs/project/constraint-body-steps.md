# Constraint-body steps and the `all T` extent

This record derives, from KerML 1.0 (formal/2026-03-01) and SysML v2 Part 1 (formal/2026-03-02),
what two runtime surfaces must do: a constraint body that states action statements before its
result expression, and the extent expression `all T`. Library text is quoted from the bundled
copies under `internal/workspace/libs/stdlib/`. UML, fUML and PSSM are not cited as authority;
nothing below rests on them. Where the specifications are silent the record says so and names the
rule the runtime applies in their place as tool-defined. The compliance rows that point here are
the *constraint body's action statements* row and the *extent expressions* row of the
[compliance mapping](spec-compliance.md).

## 1. Statements in a constraint body

### What a constraint body is

- **SysML v2 §7.20.2.** A constraint definition is a KerML predicate and a constraint usage a
  KerML Boolean expression; "the body of a constraint definition or usage is notated like a
  calculation body (see 7.19.2), except that the result expression must be Boolean".
- **SysML v2 §7.19.2.** A calculation body is notated like an action body, with an optional
  result expression at the end; the body's members are features — and steps — of the
  calculation.
- **KerML §7.4.8.1–§7.4.8.2.** A function is a behavior, so its body's steps are steps of every
  performance of it; "the result expression ... is implicitly bound to the result parameter".
- **KerML §7.4.8.4, §8.4.4.8.1.** A predicate is a function whose result is Boolean; each
  evaluation of a constraint is one performance of it, and the verdict is that performance's
  result (`checkFunctionResultBindingConnector`).
- **SysML v2 §8.4.16.1–§8.4.16.2; `Constraints.sysml`.** A constraint usage is checked through
  `ConstraintCheck`: an asserted constraint is one whose every performance's result is true,
  a negated one whose every result is false.

So a constraint body stating `attribute y : Real = 0; assign y := 5; y > 3` is one Boolean
function: `y` is a feature of each performance, the assignment is a step of it, and the verdict
is the value of `y > 3` in that performance.

### What the specifications settle

1. **The steps are performed for the verdict.** They are steps of the function (KerML §7.4.8),
   so evaluating the constraint performs them; a verdict that skipped them would be the result of
   a different function.
2. **Locals belong to the performance.** A feature declared in the body is a feature of the
   performance (KerML §7.4.7.1, steps and features of a behavior are featured by its
   performances), so each check has its own values and nothing carries over between two checks.
3. **An untargeted assignment writes the constraint's own performance.** SysML v2 §7.17.9: an
   assignment "sets the value of a referent feature of a target occurrence"; with no target
   written, the target is the default. `Actions.sysml` declares
   `in target : Occurrence[1] default that as Occurrence` on `assignmentActions`, documented
   "the default target for assignmentActions is its featuring instance (if that is an
   Occurrence)" — here the constraint performance. SysML v2 §8.3.17.5
   (`AssignmentActionUsage::targetArgument`, `referent`) makes the referent a feature *of that
   target*. An untargeted `assign mass := …` where `mass` is a feature of the constrained part,
   not of the performance, therefore names no feature of its target.
4. **The constraint's parameters are features of the performance.** A write to one changes that
   performance's value of it, not the argument expression or the caller's binding.

### What they do not settle

- **Order of steps relative to the result.** KerML binds the result parameter to the result
  expression's result (§7.4.8.2, §8.4.4.8.1); it does not sequence the result expression after
  the other steps. Steps with no succession between them are unordered (KerML §7.4.7.2; the
  Systems Library's `subactions` are subperformances with no implied order). The result
  expression is unnamed, so no `then` can name it either. For
  `{ attribute y := 0; assign y := 5; y > 3 }` the specifications do not say whether `y > 3`
  reads 0 or 5.
- **Effects outside the performance.** An explicit target (`assign v.mass := …`), a `send`,
  a `perform` of another action, or a `terminate` acts on occurrences other than the
  performance. The specifications allow writing them in a calculation body and so in a
  constraint body; they do not say how a *check* — which tools run repeatedly, speculatively and
  in solvers — relates to those effects.

### What the runtime does

- **Tool-defined order.** The body's steps run in declaration order — inherited bodies first,
  in the order the specialization chain gives them, then the usage's own — and the conditions
  (the result expression and the nested `require`, `assume` and `assert constraint` members) are
  evaluated afterwards, in the state the steps left. This is a linearization the specifications
  permit but do not prescribe.
- **Locals and parameters** live in one fresh frame per check; the constraint's parameters are
  copied into it, so writing one changes this check's copy only.
- **Refused, with typed errors, until the open points above are decided:**
  - an untargeted assignment to a name that is no feature of the performance
    (`ErrConstraintExternalAssignment`, by point 3 above, not a policy choice);
  - explicit, chained or qualified assignment targets, `send`, `perform`, and `terminate`
    (`ErrConstraintEffect`);
  - stated flow: `first`/`then` successions and control nodes in a constraint body
    (`ErrStatementNotExecutable`), since honouring them would need a decision on how they
    combine with the unordered result expression.
- **Analysis and verification cases** keep their own procedure: their steps are the case's
  action flow, not a constraint body, and are unchanged.
- **The solver** does not encode a body's steps. A condition whose body states steps is refused
  as not translatable (`constraint body steps`); it is never translated as if the steps were
  absent.

## 2. The `all T` extent

### What the specifications say

- **KerML §7.4.9.2.** `all T` "evaluates to a sequence of all instances of the named type".
- **KerML §7.3.2.1.** The set of things a type classifies is its extent.
- **KerML §9.4.2.** `abstract function 'all' { return : Object[0..*]; }` — the result is not
  declared `ordered`; KerML gives the sequence no order.
- **KerML §7.4.6.3.** A binding connector makes the values of its ends the same: two usages a
  binding joins denote one object.
- **KerML §7.3.4.4.** A feature's values include those of the features that subset it.
- **SysML v2 §7.6.3.** A usage owned by a package has default multiplicity `0..*`.

### Where the model determines the extent

An instance belongs in `all T` when the model forces it to exist: a usage with a lower bound of
at least one whose featuring instance exists (the package-level object usages the run
materializes, and the composite features of existing objects, their own required features
recursively), objects created or written by the behaviors run so far, and the variants of a
variation. Three places where the runtime used to differ from what the model determines are
corrected:

1. **Namespace-owned bindings.** `bind a = b;` written in a package now makes `a` and `b` one
   object; a usage bound to a feature chain denotes the chain's object; ends with values of
   their own must agree (`BindingConflictError`). Bindings join usages into one equivalence
   class wherever in the model they are written.
2. **Held performances and connections.** Action, state, connection, interface, allocation and
   flow usages an object holds are reached by the extent walk, as its parts are. Constraint,
   requirement and calculation usages are not: reading one evaluates it rather than reaching an
   occurrence it holds.
3. **Namespace-level subsetting.** A package-level collection usage's objects include those of
   the usages subsetting it; anonymous members make up only the remaining lower bound, an
   abstract usage has none of its own, and a count outside the declared multiplicity is
   `ErrMultiplicityViolation`.

Destroyed objects are excluded; each object appears once; library-declared object usages
(`Time::universalClock`) are instances like any other.

### The tool-defined boundary

- **Order.** The specifications give none (§9.4.2). The runtime answers a deterministic
  order — document name, then declaration order, as the extent row of the compliance mapping
  states — so two runs over the same model and the same behavior answer the same sequence.
- **Instances no model element determines.** A package-level `part c : Car;` has multiplicity
  `0..*` (SysML v2 §7.6.3): the single object the run materializes for it is the runtime's
  choice, as is the object of a valueless `ref`, and the number of members of an open `[1..*]`
  beyond its lower bound. The runtime materializes the lower bound (one for a package-level
  object usage), and the extent reports those objects. A model that needs a specific count
  states it with a multiplicity.
- **Unbounded types.** The extent of a data type other than an enumeration (`all Integer`) is
  refused rather than enumerated.
