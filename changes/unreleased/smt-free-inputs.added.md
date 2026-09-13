- **The SMT model checker decides properties over free inputs, and is reachable as `-engine
  smt`.** A feature the model leaves unbound — an `in` parameter with no argument, an attribute
  with no default — is now a free variable of the initial state, ranging over its declared type
  (`Boolean`, `Integer`, `Natural` as `>= 0`, `Real` and a quantity over one, an enumeration's or
  variation point's constructors); a feature the model binds stays pinned as before, and a
  declared type the encoding cannot narrow (`String`, a collection, an object-valued feature) is
  *not covered* naming it, before any query. `-check-input <feature>` (`%check-input`) leaves a
  bound feature free in its domain; `-check-assume <constraint>` (`%check-assume`) asserts a
  constraint or requirement over the initial state, one no initial state satisfies being *not
  covered: assumptions admit no initial state*, never *proved*. A proof now reads *proved over
  schedules and inputs: inputs free in their domains* and lists each free input and assumption; a
  violation's witness names the values the solver chose (`inputs chosen from their domains`), its
  file opens with `input <feature> = <value>` lines ahead of the choice lines, and `-schedule
  replay:<file>` fixes them as the action starts before following the moves — a file without
  input lines replays as before, and one naming a feature the action lacks is refused naming it.
  `-json` results gain `inputs` (name, type, domain, `free`, value) and `assumptions`, and a
  witness its `inputs`. `-check-unroll <n>` (`%check-bounds unroll=`, `Budget.Unroll`) bounds the
  loop unrolling (default 4); `-check-depth` is the move bound under `smt` too (default 40). The
  `smt` engine joins the build's registry at authority *proved*, listed by `-engines`,
  `%engines` and `ListEngines` with its solver as its status; `-engine smt` and `%engine smt`
  reach it, `-engine all` with `-check-input` shows `check` refusing the free inputs beside its
  answer, and since no check is asked as `holds` under `auto`, no existing verdict, plan line or
  golden changes.
