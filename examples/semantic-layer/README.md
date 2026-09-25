# Semantic Layer Feature Demo

This demo showcases all semantic layer features implemented in the OpenSysML SysML v2 compiler.

## Features Demonstrated

### Track 1: Runtime Operators

**Equality operators (`==`, `!=`):**
- Deep equality for all value kinds (const, string, null, instance, sequence, set)
- Cross-kind comparison returns false (not error)

**Logical operators (`&`, `|`, `not`):**
- Short-circuit evaluation for `&` (and) and `|` (or)
- Boolean negation with `not`

**Arithmetic negation (`-`):**
- Unary minus for numeric values (int and real)
- Delegates to semantics layer for type-correct evaluation

**Qualified name lookup:**
- Multi-part names: `A::B::C`
- Nested namespace resolution
- Inheritance-aware member lookup

### Track 3: Feature Chain Resolution

**Member access chains:**
- Simple chains: `object.member`
- Nested chains: `object.member.submember`
- Type-aware resolution following relationships

**Feature chains in relationships:**
- Subsetting with chains
- Redefinition with chains
- Symbol storage in AST for downstream passes

### Track 4: Semantic Validation

**Typing conformance:**
- Validates subsetting relationships respect type hierarchies
- Uses `model.Conforms()` for inheritance checking

**Redefinition validation:**
- Inheritance check: member must be inherited
- Type conformance: redefined type must conform to original
- Multiplicity bounds: `lower >= original.lower`, `upper <= original.upper`

## Running the Demo

### Using the REPL

```bash
# Start the REPL and load the demo
go run ./cmd/sysml
%load examples/semantic-layer/demo.sysml

# Evaluate specific attributes
%eval equalityDemo.intEquality
%eval logicalDemo.andTrue
%eval negationDemo.negInt
%eval qualifiedDemo.usePi
%eval chainDemo.simpleValue
```

### Expected Output Examples

```sysml
%eval equalityDemo.intEquality
  = true

%eval logicalDemo.andTrue
  = true

%eval negationDemo.negInt
  = -42

%eval qualifiedDemo.usePi
  = 3.14159

%eval chainDemo.simpleValue
  = 42

%eval chainDemo.nestedValue
  = 100
```

## Demo Structure

The demo is organized into sections mirroring the implementation tracks:

1. **Runtime Operators** (lines 8-64)
   - Equality, logical, negation, qualified names
   - Combined operator expressions

2. **Feature Chain Resolution** (lines 66-130)
   - Simple and nested chains
   - Chains in relationships
   - Redefinition with chains

3. **Semantic Validation** (lines 132-175)
   - Typing conformance examples
   - Redefinition validation
   - Valid constraint scenarios

4. **Integration Example** (lines 177-214)
   - Realistic vehicle monitoring system
   - Combines all features in one scenario
   - Feature chains + operators + validation

5. **Edge Cases** (lines 216-232)
   - Complex operator combinations
   - Negation of comparisons
   - Parenthesized expressions

## Implementation Notes

### Operators

All operators follow SysML v2 syntax:
- **Logical AND:** `&` (not `&&`)
- **Logical OR:** `|` (not `||`)
- **Logical NOT:** `not` (not `!`)
- **Equality:** `==` and `!=`
- **Negation:** `-` (unary minus)

### Feature Chains

Feature chains use dot notation:
```sysml
part myNested : Nested {
    part inner : Container {
        attribute value = 100;
    }
}

attribute result = myNested.inner.value;  // 100
```

Resolution:
1. Extracts operand symbol (`myNested`)
2. Follows typing relationships to get type (`Nested`)
3. Walks chain members using `model.LookupMember` (inheritance-aware)
4. Stores intermediate symbols in AST (`part.Sym`)

### Validation

**Typing conformance** validates subsetting:
```sysml
part def Car specializes Vehicle { ... }
attribute myCar : Car subsets allVehicles : Vehicle[*];  // Valid
```

**Redefinition validation** checks:
```sysml
part def ElectricEngine specializes Engine {
    attribute redefines power[200..400];  // Valid if [200..400] ⊆ Engine.power bounds
}
```

## Testing

The demo file is validated during test runs:

```bash
# Parse validation
go test ./internal/syntax/parser/

# Resolution validation
go test ./internal/semantic/resolve/

# Runtime evaluation
go test ./internal/exec/runtime/

# Semantic validation
go test ./internal/check/passes/
```

All tests should pass with this demo loaded.

## Related Documentation

- [Architecture](../../docs/internals/architecture.md) - the pipeline this demo exercises
- [Spec compliance](../../docs/project/spec-compliance.md) - semantic rule to implementation to test
- [PR #11](https://github.com/Open-MBEE/OpenSysML/pull/11) - Pull request with all changes

## Contributing

To extend this demo:

1. Add new feature demonstrations in their respective sections
2. Ensure new examples parse cleanly in REPL
3. Add explanatory comments showing expected results
4. Update this README with new feature descriptions
