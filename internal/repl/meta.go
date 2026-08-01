package repl

import (
	"fmt"
	"os"
	"strings"

	"github.com/Open-MBEE/Systemica/internal/core/ast"
	"github.com/Open-MBEE/Systemica/internal/core/parser"
	"github.com/Open-MBEE/Systemica/internal/core/resolve"
	"github.com/Open-MBEE/Systemica/internal/core/runtime"
	"github.com/Open-MBEE/Systemica/internal/core/semantics"
	"github.com/Open-MBEE/Systemica/internal/core/source"
	"github.com/Open-MBEE/Systemica/internal/core/symbols"
)

// isMeta reports whether a trimmed input line is a meta command.
func isMeta(line string) bool {
	return strings.HasPrefix(strings.TrimSpace(line), "%")
}

var helpText = []string{
	"%help               show this help",
	"%list               list current session declarations",
	"%clear              reset the session",
	"%load <file>        read a file and submit its contents",
	"",
	"Runtime commands:",
	"%instantiate <name> create an instance of a part def",
	"%eval <expr>        evaluate an expression",
	"%slots <name>       show instance slots and values",
	"%instances          list all instantiated objects",
	"",
	"Behavioral commands:",
	"%calc <name> <args> invoke a calculation with arguments",
	"%constraint <name>  evaluate a constraint definition",
	"%requirement <name> evaluate a requirement definition",
	"",
	"Action debugging:",
	"%action <name>      start action executor debugging session",
	"%step               advance one token step",
	"%continue           run action to completion",
	"%tokens             show active tokens",
	"%break <node>       set breakpoint at node",
	"%stop               stop current debugging session",
	"",
	"State machine debugging:",
	"%state <name>       start state machine debugging session",
	"%events             show event queue",
	"%current            show current state and configuration",
	"%advance <time>     advance simulation time",
}

// runMeta executes a meta command line. Returns lines to print, whether to quit,
// and an error only for unrecoverable I/O (unknown commands print guidance).
func (s *Session) runMeta(line string) (out []string, quit bool, err error) {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return nil, false, nil
	}
	switch fields[0] {
	case "%help":
		return helpText, false, nil
	case "%list":
		decls := s.List()
		if len(decls) == 0 {
			return []string{"(empty session)"}, false, nil
		}
		return decls, false, nil
	case "%clear":
		s.Clear()
		return []string{"session cleared"}, false, nil
	case "%load":
		if len(fields) < 2 {
			return []string{"usage: %load <file>"}, false, nil
		}
		data, rerr := os.ReadFile(fields[1])
		if rerr != nil {
			return nil, false, fmt.Errorf("load %s: %w", fields[1], rerr)
		}
		r := s.Submit(string(data))
		return renderResult(r), false, nil
	case "%instantiate":
		if len(fields) < 2 {
			return []string{"usage: %instantiate <name>"}, false, nil
		}
		return s.doInstantiate(fields[1])
	case "%eval":
		if len(fields) < 2 {
			return []string{"usage: %eval <expression>"}, false, nil
		}
		expr := strings.TrimPrefix(line, "%eval")
		return s.doEval(strings.TrimSpace(expr))
	case "%slots":
		if len(fields) < 2 {
			return []string{"usage: %slots <name>"}, false, nil
		}
		return s.doSlots(fields[1])
	case "%instances":
		return s.doInstances()
	case "%calc":
		if len(fields) < 2 {
			return []string{"usage: %calc <name> [args...]"}, false, nil
		}
		return s.doCalc(fields[1:])
	case "%constraint":
		if len(fields) < 2 {
			return []string{"usage: %constraint <name>"}, false, nil
		}
		return s.doConstraint(fields[1])
	case "%requirement":
		if len(fields) < 2 {
			return []string{"usage: %requirement <name>"}, false, nil
		}
		return s.doRequirement(fields[1])
	// Action debugging
	case "%action":
		if len(fields) < 2 {
			return []string{"usage: %action <name>"}, false, nil
		}
		return s.doAction(fields[1])
	case "%step":
		return s.doStep()
	case "%continue":
		return s.doContinue()
	case "%tokens":
		return s.doTokens()
	case "%break":
		if len(fields) < 2 {
			return []string{"usage: %break <node>"}, false, nil
		}
		return s.doBreak(fields[1])
	case "%stop":
		return s.doStop()
	// State machine debugging
	case "%state":
		if len(fields) < 2 {
			return []string{"usage: %state <name>"}, false, nil
		}
		return s.doStateMachine(fields[1])
	case "%events":
		return s.doEvents()
	case "%current":
		return s.doCurrent()
	case "%advance":
		if len(fields) < 2 {
			return []string{"usage: %advance <time>"}, false, nil
		}
		return s.doAdvance(fields[1])
	default:
		return []string{fmt.Sprintf("unknown command %q (try %%help)", fields[0])}, false, nil
	}
}

// doInstantiate creates an instance of a part def.
func (s *Session) doInstantiate(name string) ([]string, bool, error) {
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, false, fmt.Errorf("runtime init: %w", err)
	}
	
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return []string{"error: no declarations loaded"}, false, nil
	}
	
	sym, ok := doc.Scope.LookupLocal(name)
	if !ok || sym == nil {
		return []string{fmt.Sprintf("error: symbol %q not found", name)}, false, nil
	}
	
	inst, err := ctx.Instantiate(sym)
	if err != nil {
		return []string{fmt.Sprintf("error: instantiation failed: %v", err)}, false, nil
	}
	
	s.instances[name] = inst
	return []string{
		fmt.Sprintf("✓ Created instance of %s", name),
		fmt.Sprintf("  ID: %d", inst.ID),
		fmt.Sprintf("  Use %%slots %s to inspect", name),
	}, false, nil
}

// doEval evaluates an expression.
func (s *Session) doEval(expr string) ([]string, bool, error) {
	// Try literal evaluation first (works even with empty session)
	literalResult, isLiteral := s.tryEvalLiteral(expr)
	if isLiteral {
		return literalResult, false, nil
	}
	
	// For feature references/complex expressions, need session context
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return []string{"error: no declarations loaded (literals work, but feature references need declarations)"}, false, nil
	}
	
	// Create runtime context
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}
	
	// Try simple feature reference lookup (e.g., "%eval x")
	if isSimpleIdentifier(expr) {
		sym, ok := doc.Scope.LookupLocal(expr)
		if ok && sym != nil {
			if usage, ok := sym.Decl.(*ast.Usage); ok && usage.Value != nil {
				val, err := ctx.Eval(usage.Value)
				if err != nil {
					return []string{fmt.Sprintf("error: evaluation failed: %v", err)}, false, nil
				}
				return []string{
					fmt.Sprintf("✓ %s", expr),
					fmt.Sprintf("  = %s", formatValue(val)),
				}, false, nil
			}
		}
		return []string{fmt.Sprintf("error: symbol %q not found", expr)}, false, nil
	}
	
	// Complex expression with feature refs - inject into session context
	tempSrc := s.joined() + fmt.Sprintf("\nattribute __eval__ = %s;", expr)
	p := parser.New(source.New("eval", []byte(tempSrc)))
	root := p.ParseFile()
	
	if len(p.Diagnostics) > 0 {
		lines := []string{"error: parse failed:"}
		for _, d := range p.Diagnostics {
			lines = append(lines, "  "+d.Message)
		}
		return lines, false, nil
	}
	
	// Find __eval__ attribute (should be last member)
	var evalUsage *ast.Usage
	for i := len(root.Members) - 1; i >= 0; i-- {
		if usage, ok := root.Members[i].(*ast.Usage); ok && usage.Value != nil {
			if usage.Ident.ShortName == "__eval__" {
				evalUsage = usage
				break
			}
		}
	}
	
	if evalUsage == nil || evalUsage.Value == nil {
		return []string{"error: could not parse expression"}, false, nil
	}
	
	val, err := ctx.Eval(evalUsage.Value)
	if err != nil {
		return []string{fmt.Sprintf("error: evaluation failed: %v", err)}, false, nil
	}
	
	return []string{
		fmt.Sprintf("✓ %s", expr),
		fmt.Sprintf("  = %s", formatValue(val)),
	}, false, nil
}

// tryEvalLiteral attempts to evaluate standalone literal expressions.
func (s *Session) tryEvalLiteral(expr string) ([]string, bool) {
	// Parse as standalone attribute
	src := fmt.Sprintf("attribute __lit__ = %s;", expr)
	p := parser.New(source.New("literal", []byte(src)))
	root := p.ParseFile()
	
	if len(p.Diagnostics) > 0 || len(root.Members) == 0 {
		return nil, false
	}
	
	usage, ok := root.Members[0].(*ast.Usage)
	if !ok || usage.Value == nil {
		return nil, false
	}
	
	// Use runtime context with empty model (no symbols needed for literals)
	emptyIdx := symbols.NewIndex()
	emptyModel := semantics.NewModel(resolve.New(emptyIdx))
	ctx := runtime.NewContext(emptyModel, resolve.New(emptyIdx), 100000)
	
	val, err := ctx.Eval(usage.Value)
	if err != nil {
		// Not evaluable as literal (needs session symbols)
		return nil, false
	}
	
	return []string{
		fmt.Sprintf("✓ %s", expr),
		fmt.Sprintf("  = %s", formatValue(val)),
	}, true
}


// isSimpleIdentifier checks if string is a single identifier (no operators/spaces).
func isSimpleIdentifier(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return false
	}
	// Simple heuristic: no spaces, operators, or parens
	for _, ch := range s {
		if ch == ' ' || ch == '+' || ch == '-' || ch == '*' || ch == '/' || 
		   ch == '(' || ch == ')' || ch == '.' {
			return false
		}
	}
	return true
}

// doSlots shows instance slots.
func (s *Session) doSlots(name string) ([]string, bool, error) {
	inst, ok := s.instances[name]
	if !ok {
		return []string{fmt.Sprintf("error: no instance named %q (use %%instantiate first)", name)}, false, nil
	}
	
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, false, fmt.Errorf("runtime init: %w", err)
	}
	
	lines := []string{
		fmt.Sprintf("Instance: %s (ID: %d)", name, inst.ID),
		"Slots:",
	}
	
	// Get effective features to know slot names
	features := ctx.FeaturesOf(inst.Type)
	if len(features) == 0 {
		lines = append(lines, "  (no features)")
		return lines, false, nil
	}
	
	for _, feat := range features {
		slot, err := inst.GetSlot(ctx, feat.Name)
		if err != nil {
			lines = append(lines, fmt.Sprintf("  %s: <error: %v>", feat.Name, err))
			continue
		}
		lines = append(lines, fmt.Sprintf("  %s = %s", feat.Name, formatValue(slot.Value)))
	}
	
	return lines, false, nil
}

// doInstances lists all instantiated objects.
func (s *Session) doInstances() ([]string, bool, error) {
	if len(s.instances) == 0 {
		return []string{"(no instances created)"}, false, nil
	}
	
	lines := []string{"Instances:"}
	for name, inst := range s.instances {
		lines = append(lines, fmt.Sprintf("  %s (ID: %d)", name, inst.ID))
	}
	return lines, false, nil
}

// formatValue renders a runtime value for display.
func formatValue(val runtime.Value) string {
	switch val.Kind {
	case runtime.ValConst:
		switch val.Const.Kind {
		case semantics.ValInt:
			return fmt.Sprintf("%d", val.Const.Int)
		case semantics.ValReal:
			return fmt.Sprintf("%.2f", val.Const.Real)
		case semantics.ValBool:
			return fmt.Sprintf("%v", val.Const.Bool)
		case semantics.ValInfinity:
			return "∞"
		default:
			return "<unknown const>"
		}
	case runtime.ValNull:
		return "null"
	case runtime.ValString:
		return fmt.Sprintf("%q", val.Str)
	case runtime.ValInstance:
		return fmt.Sprintf("Instance(ID: %d)", val.Instance)
	case runtime.ValSequence:
		return fmt.Sprintf("Sequence[%d]", val.Sequence.Size())
	case runtime.ValSet:
		return fmt.Sprintf("Set{%d}", val.Set.Size())
	default:
		return "<unknown>"
	}
}

// doCalc invokes a calculation with arguments.
func (s *Session) doCalc(args []string) ([]string, bool, error) {
	if len(args) == 0 {
		return []string{"usage: %calc <name> [args...]"}, false, nil
	}
	
	calcName := args[0]
	calcArgs := args[1:]
	
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return []string{"error: no declarations loaded"}, false, nil
	}
	
	sym, ok := doc.Scope.LookupLocal(calcName)
	if !ok || sym == nil {
		return []string{fmt.Sprintf("error: calc %q not found", calcName)}, false, nil
	}
	
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}
	
	// Parse arguments as literal expressions (no session context needed)
	argValues := make([]runtime.Value, len(calcArgs))
	for i, argStr := range calcArgs {
		// Use empty context for literal evaluation
		emptyIdx := symbols.NewIndex()
		emptyModel := semantics.NewModel(resolve.New(emptyIdx))
		literalCtx := runtime.NewContext(emptyModel, resolve.New(emptyIdx), 100000)
		
		// Parse as attribute inside a part (top-level attribute syntax not supported)
		src := fmt.Sprintf("part __dummy__ { attribute __arg__ = %s; }", argStr)
		p := parser.New(source.New("arg", []byte(src)))
		root := p.ParseFile()
		
		// Ignore parse diagnostics - literals might have unresolved types
		if len(root.Members) == 0 {
			return []string{fmt.Sprintf("error: failed to parse argument %q", argStr)}, false, nil
		}
		
		// Unwrap Membership if present
		member := root.Members[0]
		if membership, ok := member.(*ast.Membership); ok {
			member = membership.Member
		}
		
		// Extract attribute from part body
		partUsage, ok := member.(*ast.Usage)
		if !ok || partUsage.Kind != ast.UsagePart {
			return []string{fmt.Sprintf("error: argument %q: not a part usage", argStr)}, false, nil
		}
		
		if len(partUsage.Members) == 0 {
			return []string{fmt.Sprintf("error: argument %q: empty part body", argStr)}, false, nil
		}
		
		// Unwrap first member (attribute)
		attrMember := partUsage.Members[0]
		if attrMembership, ok := attrMember.(*ast.Membership); ok {
			attrMember = attrMembership.Member
		}
		
		usage, ok := attrMember.(*ast.Usage)
		if !ok {
			return []string{fmt.Sprintf("error: argument %q: first member not usage", argStr)}, false, nil
		}
		
		if usage.Value == nil {
			return []string{fmt.Sprintf("error: argument %q: usage has no value", argStr)}, false, nil
		}
		
		val, err := literalCtx.Eval(usage.Value)
		if err != nil {
			return []string{fmt.Sprintf("error: evaluation of argument %q failed: %v", argStr, err)}, false, nil
		}
		argValues[i] = val
	}
	
	// Invoke calculation via InvokeCalc
	result, err := ctx.InvokeCalc(sym, argValues, doc.Scope)
	if err != nil {
		return []string{fmt.Sprintf("error: calc invocation failed: %v", err)}, false, nil
	}
	
	return []string{
		fmt.Sprintf("✓ %s(%s)", calcName, strings.Join(calcArgs, ", ")),
		fmt.Sprintf("  = %s", formatValue(result)),
	}, false, nil
}

// doConstraint evaluates a constraint definition.
func (s *Session) doConstraint(name string) ([]string, bool, error) {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return []string{"error: no declarations loaded"}, false, nil
	}
	
	sym, ok := doc.Scope.LookupLocal(name)
	if !ok || sym == nil {
		return []string{fmt.Sprintf("error: constraint %q not found", name)}, false, nil
	}
	
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}
	
	passed, err := ctx.EvaluateConstraint(sym, doc.Scope)
	if err != nil || !passed {
		return []string{
			fmt.Sprintf("✗ Constraint %s failed", name),
			fmt.Sprintf("  Error: %v", err),
		}, false, nil
	}
	
	return []string{
		fmt.Sprintf("✓ Constraint %s passed", name),
	}, false, nil
}

// doRequirement evaluates a requirement definition.
func (s *Session) doRequirement(name string) ([]string, bool, error) {
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return []string{"error: no declarations loaded"}, false, nil
	}
	
	sym, ok := doc.Scope.LookupLocal(name)
	if !ok || sym == nil {
		return []string{fmt.Sprintf("error: requirement %q not found", name)}, false, nil
	}
	
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return []string{"error: " + err.Error()}, false, nil
	}
	
	passed, err := ctx.EvaluateRequirement(sym, doc.Scope)
	if err != nil || !passed {
		return []string{
			fmt.Sprintf("✗ Requirement %s failed", name),
			fmt.Sprintf("  Error: %v", err),
		}, false, nil
	}
	
	return []string{
		fmt.Sprintf("✓ Requirement %s satisfied", name),
	}, false, nil
}

// --- Action Debugging Commands ---

// doAction starts an action executor debugging session.
func (s *Session) doAction(name string) ([]string, bool, error) {
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, false, fmt.Errorf("runtime init: %w", err)
	}
	
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return []string{"error: no declarations loaded"}, false, nil
	}
	
	sym, ok := doc.Scope.LookupLocal(name)
	if !ok || sym == nil {
		return []string{fmt.Sprintf("error: action %q not found", name)}, false, nil
	}
	
	if sym.Kind != symbols.SymbolActionUsage && sym.Kind != symbols.SymbolActionDef {
		return []string{fmt.Sprintf("error: %q is not an action", name)}, false, nil
	}
	
	// Create executor
	exec, err := ctx.CreateActionExecutor(sym)
	if err != nil {
		return []string{fmt.Sprintf("error: failed to create executor: %v", err)}, false, nil
	}
	
	// Store session
	s.actionExec = &actionSession{
		name:     name,
		symbol:   sym,
		executor: exec,
	}
	
	// Display initial state
	tokens := exec.Tokens()
	return []string{
		fmt.Sprintf("✓ Started action executor for %q", name),
		fmt.Sprintf("  State: %s", exec.State()),
		fmt.Sprintf("  Tokens: %d", len(tokens)),
		"",
		"Use %step to advance, %tokens to inspect, %continue to run to completion",
	}, false, nil
}

// doStep advances the action executor one step.
func (s *Session) doStep() ([]string, bool, error) {
	if s.actionExec == nil {
		return []string{"error: no active action session (use %action <name> first)"}, false, nil
	}
	
	exec := s.actionExec.executor
	
	// Check if already completed
	if exec.State() == runtime.StateCompleted {
		return []string{"✓ Action already completed"}, false, nil
	}
	
	// Step
	err := exec.Step()
	if err != nil {
		return []string{fmt.Sprintf("error: step failed: %v", err)}, false, nil
	}
	
	// Display state
	tokens := exec.Tokens()
	out := []string{
		fmt.Sprintf("✓ Step complete"),
		fmt.Sprintf("  State: %s", exec.State()),
		fmt.Sprintf("  Tokens: %d", len(tokens)),
	}
	
	if exec.State() == runtime.StateCompleted {
		results := exec.Results()
		out = append(out, "", "✓ Action completed")
		if len(results) > 0 {
			out = append(out, "  Results:")
			for k, v := range results {
				out = append(out, fmt.Sprintf("    %s = %s", k, formatValue(v)))
			}
		}
	}
	
	return out, false, nil
}

// doContinue runs the action to completion.
func (s *Session) doContinue() ([]string, bool, error) {
	if s.actionExec == nil {
		return []string{"error: no active action session (use %action <name> first)"}, false, nil
	}
	
	exec := s.actionExec.executor
	
	// Check if already completed
	if exec.State() == runtime.StateCompleted {
		return []string{"✓ Action already completed"}, false, nil
	}
	
	// Run to completion
	err := exec.RunToCompletion()
	if err != nil {
		return []string{fmt.Sprintf("error: execution failed: %v", err)}, false, nil
	}
	
	// Display results
	results := exec.Results()
	out := []string{
		"✓ Action completed",
		fmt.Sprintf("  Final state: %s", exec.State()),
	}
	
	if len(results) > 0 {
		out = append(out, "  Results:")
		for k, v := range results {
			out = append(out, fmt.Sprintf("    %s = %s", k, formatValue(v)))
		}
	}
	
	return out, false, nil
}

// doTokens displays active tokens.
func (s *Session) doTokens() ([]string, bool, error) {
	if s.actionExec == nil {
		return []string{"error: no active action session (use %action <name> first)"}, false, nil
	}
	
	exec := s.actionExec.executor
	tokens := exec.Tokens()
	
	if len(tokens) == 0 {
		return []string{"No active tokens"}, false, nil
	}
	
	out := []string{fmt.Sprintf("Active tokens (%d):", len(tokens))}
	for _, tok := range tokens {
		locName := "<unknown>"
		if stateNode, ok := tok.Location.(*ast.StateNode); ok {
			if stateNode.Name != "" {
				locName = stateNode.Name
			}
		} else if tok.Location != nil {
			locName = fmt.Sprintf("%T", tok.Location)
		}
		
		out = append(out, fmt.Sprintf("  Token %d @ %s", tok.ID, locName))
		if len(tok.Data) > 0 {
			for k, v := range tok.Data {
				out = append(out, fmt.Sprintf("    %s = %s", k, formatValue(v)))
			}
		}
	}
	
	return out, false, nil
}

// doBreak sets a breakpoint.
func (s *Session) doBreak(nodeName string) ([]string, bool, error) {
	if s.actionExec == nil {
		return []string{"error: no active action session (use %action <name> first)"}, false, nil
	}
	
	exec := s.actionExec.executor
	exec.SetBreakpoint(nodeName)
	
	return []string{fmt.Sprintf("✓ Breakpoint set at node %q", nodeName)}, false, nil
}

// doStop stops the current debugging session.
func (s *Session) doStop() ([]string, bool, error) {
	if s.actionExec == nil && s.stateExec == nil {
		return []string{"error: no active debugging session"}, false, nil
	}
	
	sessionName := ""
	if s.actionExec != nil {
		sessionName = s.actionExec.name
		s.actionExec = nil
	} else if s.stateExec != nil {
		sessionName = s.stateExec.name
		s.stateExec = nil
	}
	
	return []string{fmt.Sprintf("✓ Stopped debugging session for %q", sessionName)}, false, nil
}

// --- State Machine Debugging Commands ---

// doStateMachine starts a state machine executor debugging session.
func (s *Session) doStateMachine(name string) ([]string, bool, error) {
	ctx, err := s.getOrCreateRuntime()
	if err != nil {
		return nil, false, fmt.Errorf("runtime init: %w", err)
	}
	
	doc := s.ws.Document(docName)
	if doc == nil || doc.Scope == nil {
		return []string{"error: no declarations loaded"}, false, nil
	}
	
	sym, ok := doc.Scope.LookupLocal(name)
	if !ok || sym == nil {
		return []string{fmt.Sprintf("error: state machine %q not found", name)}, false, nil
	}
	
	if sym.Kind != symbols.SymbolStateDef && sym.Kind != symbols.SymbolStateUsage {
		return []string{fmt.Sprintf("error: %q is not a state machine", name)}, false, nil
	}
	
	// Create executor
	exec, err := ctx.CreateStateExecutor(sym)
	if err != nil {
		return []string{fmt.Sprintf("error: failed to create executor: %v", err)}, false, nil
	}
	
	// Store session
	s.stateExec = &stateSession{
		name:     name,
		symbol:   sym,
		executor: exec,
	}
	
	// Display initial state
	currentState := exec.CurrentState()
	stateName := "<unknown>"
	if stateNode, ok := currentState.(*ast.StateNode); ok && stateNode.Name != "" {
		stateName = stateNode.Name
	}
	
	return []string{
		fmt.Sprintf("✓ Started state machine executor for %q", name),
		fmt.Sprintf("  Current state: %s", stateName),
		fmt.Sprintf("  Time: %.2f", exec.CurrentTime()),
		fmt.Sprintf("  Events: %d", exec.EventQueue().Len()),
		"",
		"Use %events to see queue, %current for state, %advance <time> to step",
	}, false, nil
}

// doEvents displays the event queue.
func (s *Session) doEvents() ([]string, bool, error) {
	if s.stateExec == nil {
		return []string{"error: no active state machine session (use %state <name> first)"}, false, nil
	}
	
	exec := s.stateExec.executor
	queue := exec.EventQueue()
	
	if queue.Len() == 0 {
		return []string{"Event queue empty"}, false, nil
	}
	
	// Note: EventQueue doesn't expose events directly, so just show count
	return []string{
		fmt.Sprintf("Event queue: %d events", queue.Len()),
		"Use %advance <time> to process next event",
	}, false, nil
}

// doCurrent shows current state and configuration.
func (s *Session) doCurrent() ([]string, bool, error) {
	if s.stateExec == nil {
		return []string{"error: no active state machine session (use %state <name> first)"}, false, nil
	}
	
	exec := s.stateExec.executor
	currentState := exec.CurrentState()
	stateStack := exec.StateStack()
	stateData := exec.StateData()
	
	stateName := "<unknown>"
	if stateNode, ok := currentState.(*ast.StateNode); ok && stateNode.Name != "" {
		stateName = stateNode.Name
	}
	
	out := []string{
		fmt.Sprintf("Current state: %s", stateName),
		fmt.Sprintf("Time: %.2f", exec.CurrentTime()),
		fmt.Sprintf("Execution state: %s", exec.State()),
	}
	
	if len(stateStack) > 1 {
		out = append(out, "", "State stack (active configuration):")
		for i, stateNode := range stateStack {
			if stateNode.Name != "" {
				out = append(out, fmt.Sprintf("  %d. %s", i, stateNode.Name))
			}
		}
	}
	
	if len(stateData) > 0 {
		out = append(out, "", "State data:")
		for k, v := range stateData {
			out = append(out, fmt.Sprintf("  %s = %s", k, formatValue(v)))
		}
	}
	
	return out, false, nil
}

// doAdvance advances simulation time.
func (s *Session) doAdvance(timeStr string) ([]string, bool, error) {
	if s.stateExec == nil {
		return []string{"error: no active state machine session (use %state <name> first)"}, false, nil
	}
	
	// Parse time - for now just process next event
	// TODO: parse timeStr and advance to specific time
	
	exec := s.stateExec.executor
	
	if exec.EventQueue().Len() == 0 {
		return []string{"Event queue empty - no events to process"}, false, nil
	}
	
	// Process next event
	err := exec.ProcessNextEvent()
	if err != nil {
		return []string{fmt.Sprintf("error: event processing failed: %v", err)}, false, nil
	}
	
	// Display new state
	currentState := exec.CurrentState()
	stateName := "<unknown>"
	if stateNode, ok := currentState.(*ast.StateNode); ok && stateNode.Name != "" {
		stateName = stateNode.Name
	}
	
	out := []string{
		"✓ Event processed",
		fmt.Sprintf("  Current state: %s", stateName),
		fmt.Sprintf("  Time: %.2f", exec.CurrentTime()),
		fmt.Sprintf("  Remaining events: %d", exec.EventQueue().Len()),
	}
	
	if exec.State() == runtime.StateCompleted {
		out = append(out, "", "✓ State machine completed (final state reached)")
	}
	
	return out, false, nil
}
