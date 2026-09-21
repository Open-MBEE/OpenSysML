- **A connector whose first end is missing is reported at the `to`/`then`, with
  one diagnostic.** `connection c connect  to ;` read the `to` as the first
  end's name and then wanted a second `to`, producing the misleading `expected
  'to' between connector ends` at the semicolon, and `connect to a;` produced a
  second spurious `expected '{' or ';' after declaration`. The clause now reports
  `expected a connector end before 'to'` (or `'then'` for successions) at the
  keyword and still parses the end after it, so recovery adds no further error;
  `connect to to b;`, where the first end is genuinely named `to`, is unchanged.
