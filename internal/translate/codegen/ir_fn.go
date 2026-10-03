package codegen

// FnCase is one function a function value may be at run time: a calc, read
// against no object, or a body-local calc closing over the run that read it.
type FnCase struct {
	Name string // qualified name, which the value prints as
	ID   int    // position in Program.FnCases
	// Closure is set for a body-local calc; Env are the bindings it captures.
	Closure bool
	Env     []Param
	val     funcValue
	fn      *Func
	pass    int
}

// FnSet is the set of cases a function type ranges over, in case order;
// sets are interned, so equal sets are one *FnSet.
type FnSet struct {
	Cases []*FnCase
	ID    int
}

// FnType is the type of function values ranging over s.
func FnType(s *FnSet) Type { return Type{k: kindFunc, Fns: s} }

// FnLit is a read of case Case; for a closure, Run is the identity of the run
// of the body declaring it and Env the values of its captured bindings.
type FnLit struct {
	Case *FnCase
	Run  Expr
	Env  []Expr
	T    Type
}

// FnWiden is X, a function value, typed by the wider function type T.
type FnWiden struct {
	X Expr
	T Type
}

// FnEnv is captured binding I of case Case, read from the function value in
// variable Name, which holds that case.
type FnEnv struct {
	Name string
	Case *FnCase
	I    int
	T    Type
}

// FnDispatch evaluates F into variable Name, then the arm of Cases its case
// selects: an application of that function.
type FnDispatch struct {
	Name  string
	F     Expr
	Cases []Expr
	T     Type
}

func (x FnLit) Type() Type      { return x.T }
func (x FnWiden) Type() Type    { return x.T }
func (x FnEnv) Type() Type      { return x.T }
func (x FnDispatch) Type() Type { return x.T }
