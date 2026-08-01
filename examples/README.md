# Systemica Examples

Comprehensive demos for SysML v2 execution environment features.

## Behavioral Execution

### Action Execution (Petri-Net Token Flow)
**File:** `action-executor-demo.sysml`

Demonstrates action workflows with:
- Sequential execution (initial → action → final)
- Fork/Join parallelism (3 concurrent paths)
- Decision/Merge branching (guarded conditionals)
- ObjectFlow data routing (pin-to-pin transfer)

**REPL Commands:**
```bash
%load examples/action-executor-demo.sysml
%action SimpleSequential
%step              # Advance tokens one step
%tokens            # Show active tokens
%continue          # Run to completion
%stop              # Exit debugging
```

### State Machine Execution (Event-Driven)
**File:** `state-machine-demo.sysml`

Demonstrates state machine workflows with:
- TimeEvent transitions (traffic light timing)
- ChangeEvent conditions (temperature monitoring)
- Guard conditions (conditional transitions)
- Hierarchical states (composite/nested states)
- Entry/exit behaviors (state actions)
- Transition effects (executed during transition)

**REPL Commands:**
```bash
%load examples/state-machine-demo.sysml
%state TrafficLightSimple
%advance           # Process next event
%current           # Show state, stack, data, time
%events            # Show event queue
%stop              # Exit debugging
```

### Combined Action & State Workflows
**File:** `combined-behavioral-demo.sysml`

Demonstrates coordination between actions and states:
- State entry actions (inline expressions)
- Parallel action workflows (fork/join/merge)
- Conditional action branching (decision guards)
- Hierarchical state machines (3-level nesting)
- Real-world order processing system

**REPL Commands:**
```bash
%load examples/combined-behavioral-demo.sysml
%state DataProcessor        # State with entry actions
%action ParallelProcessing  # Fork/join workflow
%state MultilevelWorkflow   # Hierarchical states
```

## Structural Modeling

### Basic REPL Usage
**File:** `repl-behavioral-demo.sysml`

Demonstrates REPL basics:
- Part definitions with attributes
- Instantiation and slot inspection
- Expression evaluation
- Calculation invocation
- Constraint evaluation
- Requirement validation

**REPL Commands:**
```bash
%load examples/repl-behavioral-demo.sysml
%instantiate Wheel
%slots Wheel
%calc add 10 20
%constraint ValidSpeed
%requirement SafetyReq
```

### Behavioral Parsing
**File:** `phase-c-behavioral-bodies.sysml`

Demonstrates behavioral AST parsing:
- Action bodies with control flow nodes
- State machine definitions
- Transition syntax (succession edges)
- Entry/exit behavior syntax

## Documentation

- **Action Executor Design:** `ACTION-EXECUTOR-DEMO.md`
- **Action Executor README:** `README-ACTION-EXECUTOR.md`
- **Runtime Demo:** `runtime_repl_demo.md`

## Quick Reference

### Action Debugging Commands
| Command | Description |
|---------|-------------|
| `%action <name>` | Start action debugging session |
| `%step` | Advance all tokens one step |
| `%continue` | Run action to completion |
| `%tokens` | Show active tokens with location + data |
| `%break <node>` | Set breakpoint on node |
| `%stop` | Stop debugging session |

### State Machine Debugging Commands
| Command | Description |
|---------|-------------|
| `%state <name>` | Start state machine debugging |
| `%events` | Show event queue length |
| `%current` | Show current state, stack, data, time |
| `%advance` | Process next event from queue |
| `%stop` | Stop debugging session |

### Runtime Commands
| Command | Description |
|---------|-------------|
| `%instantiate <name>` | Create instance from part definition |
| `%slots <name>` | Show instance slots with values |
| `%instances` | List all instances |
| `%eval <expr>` | Evaluate expression |
| `%calc <name> [args...]` | Invoke calculation with arguments |
| `%constraint <name>` | Evaluate constraint |
| `%requirement <name>` | Evaluate requirement |

## Test Files

- `test-parse.go` — Standalone parser test
- `runtime-demo/` — Runtime execution tests

## Getting Started

1. **Build the REPL:**
   ```bash
   go build -o sysml ./cmd/sysml
   ```

2. **Load an example:**
   ```bash
   ./sysml
   sysml> %load examples/action-executor-demo.sysml
   ```

3. **Try action debugging:**
   ```bash
   sysml> %action ForkJoinExample
   sysml> %step
   sysml> %tokens
   sysml> %continue
   ```

4. **Try state machine debugging:**
   ```bash
   sysml> %state TrafficLightSimple
   sysml> %advance
   sysml> %current
   sysml> %events
   ```

## Architecture

**Action Executor** — Petri-net token-flow semantics:
- Tokens carry control + data through graph
- Fork creates parallel tokens (concurrency)
- Join performs barrier synchronization
- Decision routes based on guard evaluation
- ObjectFlow transfers data between pins

**State Executor** — Event-driven state machine:
- TimeEvent schedules future transitions
- ChangeEvent polls conditions
- Guard evaluation for conditional transitions
- Hierarchical states with LCA-based entry/exit
- Entry/exit/effect behaviors

See [ARCHITECTURE.md](../docs/ARCHITECTURE.md) for complete design.

## Contributing

Examples should:
- Focus on single feature or workflow
- Include REPL usage comments
- Show expected output in comments
- Follow SysML v2 syntax conventions
- Test before committing

## License

Apache 2.0 — see [LICENSE](../LICENSE)
