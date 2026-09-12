- **An ordering operator over a value the Kernel Function Library declares no ordering for is
  reported as a type mismatch, not as "operands must be constants".** `Color::red < Color::blue`
  over `enum def Color { red; green; blue; }` was refused with `comparison operands must be
  constants, got enumeration literal and enumeration literal`, though both operands are constants;
  the refusal stands — `DataFunctions::'<'` and `ScalarFunctions::'<'` are abstract and only the
  numeric libraries and `StringFunctions` declare an ordering — but it now says so: `type mismatch:
  operator '<' is not defined for the enumeration literal Color::red and the enumeration literal
  Color::blue; DataFunctions::'<' is abstract and no library function declares '<' for the
  enumeration Color, which is no ScalarValue`. Every ordering operator (`<`, `>`, `<=`, `>=`) over a
  Boolean, a part or metadata instance, a function value, a sequence, a set, null or a measurement
  reference reports the same way, naming the operator, both operand types and the library function
  that would have to declare it, on the operator, its `'<'(x, y)` library form, `->minimize`/
  `->maximize`, and in the compiled calc tier, which no longer orders a Boolean as a number. An
  enumeration specializing `Integer`, Strings and quantities order as before.
