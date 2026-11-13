package program

// Operator is an operator of the expression ladder, whatever its arity.
type Operator uint8

const (
	OpInvalid        Operator = iota
	OpConditional             // if C ? A else B
	OpNullCoalesce            // ??
	OpImplies                 // implies
	OpOr                      // |
	OpConditionalOr           // or
	OpXor                     // xor
	OpAnd                     // &
	OpConditionalAnd          // and
	OpEq                      // ==
	OpNeq                     // !=
	OpEqEqEq                  // ===
	OpNeqEqEq                 // !==
	OpHasType                 // hastype
	OpIsType                  // istype
	OpAt                      // @
	OpMetaAt                  // @@
	OpAs                      // as
	OpMeta                    // meta
	OpLt                      // <
	OpGt                      // >
	OpLe                      // <=
	OpGe                      // >=
	OpRange                   // ..
	OpAdd                     // +
	OpSub                     // -
	OpMul                     // *
	OpDiv                     // /
	OpMod                     // %
	OpPow                     // ** or ^
	OpNeg                     // unary -
	OpPos                     // unary +
	OpBitNot                  // unary ~
	OpNot                     // unary not
	OpAll                     // extent: all
	OpIndex                   // [ ... ]
)
