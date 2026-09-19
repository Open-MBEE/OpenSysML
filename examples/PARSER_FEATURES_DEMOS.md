# Parser Features Demos

Demonstrations of the SysML v2 / KerML parser features that let every file of the official standard library parse cleanly. "Parses cleanly" is the claim here — not that the constructs are semantically implemented.

## Overview

These demos showcase parser improvements across multiple development sessions (Sessions 2-5). Each demo file focuses on a specific category of features with working examples extracted from real stdlib usage patterns.

**Parser Status:** 96/96 bundled library files parse cleanly (94 official SysML v2 standard library files plus two OpenSysML extensions), gated by `internal/workspace/libs/stdlib_conformance_test.go`.

## Demo Files

### Original Demos (Sessions 2-3)

#### 1. Relationships Demo (`parser_features_demo_relationships.kerml`)

**New relationship keywords:**
- `inverse of` - Bidirectional relationship inverses
- `unions` - Union relationships that combine multiple features
- `chains` - Feature chaining for multi-level navigation

**Example:**
```kerml
end feature parent: Node[1];
end feature child: Node[0..*] inverse of parent;

feature dataPath chains source.target;
```

#### 2. Modifiers Demo (`parser_features_demo_modifiers.kerml`)

**Visibility modifiers:**
- `public` / `protected` / `private` - Access control
- `const` - Immutable features
- `var` - Features whose value may change

**Post-multiplicity modifiers:**
- `ordered` - Maintains insertion order
- `nonunique` - Allows duplicates

**Example:**
```kerml
protected feature thisParticipant :>> self;
const feature version: String[1];
var feature currentConnections: Integer[1];
feature connections: ConnectionPort[1..*] nonunique;
```

#### 3. Binding Demo (`parser_features_demo_binding.kerml`)

**Binding connector patterns:**
- Anonymous with multiplicity: `binding [mult] target = value`
- Named declaration: `binding name [mult] of [mult] source = [mult] value`
- With 'of' keyword: `binding name of [mult] target = [mult] value`
- Feature chains as ends: `binding [1] config.host = serverAddress`

**Example:**
```kerml
binding edgeBinding [1] of [0..*] base.edges = [0..*] boundaryEdges;
binding startBinding of [1] startEvent.timestamp = [1] endEvent.timestamp;
```

#### 4. Connectors & Succession Demo (`parser_features_demo_connectors.sysml`)

**Connection patterns:**
- Named connections with typing
- `connect` keyword: `connection :Type connect [1] a to [1] b`

This demo uses SysML usage notation (`connection def`, `connect`, `action`), so it is a `.sysml`
file. The KerML side of the same features (`connector`, `succession`, connector ends) is covered by
demo 6, `parser_features_demo_advanced_connectors.kerml`.

**Succession patterns:**
- Named: `succession name first x then y`
- Anonymous: `succession first x then y`
- Multiplicities on the ends: `succession first [1] x then [1] y`

#### 5. Default Values Demo (`parser_features_demo_defaults.kerml`)

**Default keyword:**
- Alternative to `=` for value assignment
- Clearer intent for default values

### Advanced Demos (Sessions 4-5)

#### 6. Advanced Connectors Demo (`parser_features_demo_advanced_connectors.kerml`)

**Features from Tasks 58-86:**
- Connector end modifiers with `end` keyword
- Connector `references` keyword
- Single-end connector typing with `to`
- Named succession with identifier multiplicities

**Example:**
```kerml
// End modifier with shortname and multiplicity
end self2 [1] feature target: Node;

// Connector with references
connector DataFlow {
    from [1] producer references outputPort
    to [1] consumer references inputPort;
}

// Single-end typing
private connector [0..1] transitionLink to [1..*] trigger;
```

#### 7. Action Semantics Demo (`parser_features_demo_action_semantics.sysml`)

**Features from Tasks 72, 93:**
- `assign` statements
- `perform` actions
- `while` loops
- `if/else` statements
- Namespace-level succession with `then`

**Example:**
```sysml
action IterateSequence {
    assign index := 1;
    then while index <= 10 {
        assign var := index;
        then assign index := index + 1;
    }
}

// Namespace succession
action Step1 { assign data := 1; }
then action Step2 { assign result := data; }
```

#### 8. Advanced Bodies Demo (`parser_features_demo_advanced_bodies.kerml`)

**Features from Tasks 75, 80, 83, 85, 87, 91:**
- Return statements in predicate bodies
- Anonymous return parameters
- Body params with multiplicity after type
- Shorthand body param syntax (no `in` keyword)
- Bool bodies with return statements
- General members in predicate bodies

**Example:**
```kerml
// Anonymous return
abstract predicate BooleanEvaluation {
    return : Boolean[1];
}

// Shorthand body param
feature hasPositive = vertices->exists{p : Point; p.x > 0};

// Bool with return
bool earlierCheck : Boolean {
    in t1: Transfer [1];
    return t1First = true;
}
```

#### 9. Messages & Events Demo (`parser_features_demo_messages_events.sysml`)

**Features from Tasks 88-90:**
- `message` keyword (synonym for flow)
- `event occurrence` parameters
- `ref` with body members

These are SysML-only keywords, so this demo is a `.sysml` file. The KerML counterparts
(`interaction`/`flow`, features with bodies) are covered by demos 6 and 11.

**Example:**
```sysml
// Message flow definition
abstract flow def Message {
    attribute content: String;
}

// Event occurrence parameter
in event occurrence sourceEvent [1];

// Ref with body
ref payload [0..*] {
    attribute dataType: String;
}
```

#### 10. Declarations Demo (`parser_features_demo_declarations.kerml`)

**Features from Tasks 65-66, 73-74, 78:**
- Multiplicity declarations
- `classifier` keyword (canonical KerML term)
- `subclassifier` keyword
- Multiple typing with comma
- Subset/disjoint constraint statements

**Example:**
```kerml
// Multiplicity declaration
multiplicity exactlyOne [1..1] {
    doc /* Exactly one element */
}

// Classifier
abstract classifier Anything {
    feature self: Anything[1];
}

// Multiple typing
feature multiTyped: Type1, Type2, Type3;

// Subset statement
subset laterOccurrence.successors subsets earlierOccurrence.successors;
```

#### 11. Edge Cases Demo (`parser_features_demo_edge_cases.kerml`)

**Features from Tasks 62, 67, 70, 76, 79:**
- Keywords as feature names in expressions
- Identifier-based multiplicities
- `binding` connectors in a behavior body
- `step` usage with `do` as name
- Features with bodies

**Example:**
```kerml
// Keywords in expressions
feature entrySequence = state.entry;

// Identifier multiplicity
succession items [itemCount] first [1] initialize then [1] process;

// Binding connector
binding payload = accepter.payload;

// Step with keyword name
step do[1] subsets middle;
```

## Running the Demos

Test all demos:

```bash
go test ./examples -run '^TestAllParserFeaturesDemos$' -v
```

Test individual categories:

```bash
# Original demos
go test ./examples -run '^TestParserFeaturesDemos$' -v

# Advanced demos
go test ./examples -run '^TestParserFeaturesAdvancedConnectors$' -v
go test ./examples -run '^TestParserFeaturesActionSemantics$' -v
go test ./examples -run '^TestParserFeaturesAdvancedBodies$' -v
go test ./examples -run '^TestParserFeaturesMessagesEvents$' -v
go test ./examples -run '^TestParserFeaturesDeclarations$' -v
go test ./examples -run '^TestParserFeaturesEdgeCases$' -v
```

## Development Context

These features were implemented across five development sessions:

### Sessions 2-3 (Tasks 47-57)
- **Coverage:** 76.8% → 81.1% (+4.3pp, +4 files)
- **Features:** inverse of, unions, protected/readonly, nonunique, default keyword, binding syntax, constant modifier, succession first/then, chains relationship, connection connect keyword

### Sessions 4-5 (Tasks 58-94)
- **Coverage:** 81.1% → 100.0% (+18.9pp, +18 files)
- **Features:** 37 tasks implementing all remaining stdlib patterns
  - Connector enhancements (end modifiers, references, single-end typing, transition usage)
  - Action semantics (assign, perform, while, if, namespace succession)
  - Advanced bodies (predicate/bool returns, shorthand params, constraint members)
  - Messages & events (message/event keywords, ref with body)
  - Declarations (multiplicity, classifier/subclassifier, multiple typing, subset/disjoint)
  - Edge cases (keywords in expressions, identifier multiplicities, bind/require/step shortcuts)

**Final Status:** 100.0% (96/96 bundled library files parse cleanly)

## Real-World Usage

These patterns appear throughout the official standard library:

**Original features (Sessions 2-3):**
- Base.kerml: chains relationship, visibility modifiers
- Occurrences.kerml: inverse of, succession patterns
- Interfaces.sysml: protected references, nonunique ports
- ShapeItems.sysml: complex binding patterns, connection with connect
- CausationConnections.sysml: named succession with first/then

**Advanced features (Sessions 4-5):**
- Actions.sysml: action semantics, namespace succession, transition usage
- Flows.sysml: message/event keywords, ref with body
- Base.kerml: multiplicity declarations, classifier keyword
- Occurrences.kerml: subset/disjoint statements, double redefines
- StatePerformances.kerml: keywords in expressions, identifier multiplicities
- Views.sysml: require with members, multiple typing
- TransitionPerformances.kerml: single-end connector typing
- Performances.kerml: anonymous return parameters
- TradeStudies.sysml: body param nested members

## Architecture Notes

- Hand-written recursive descent parser (zero overhead, full error recovery)
- Grammar source: OMG pilot Xtext grammars (SysML.xtext + KerMLExpressions)
- Notation the reference accepts and OpenSysML still rejects is enumerated in [the pilot differential](../docs/project/pilot-differential.md)
