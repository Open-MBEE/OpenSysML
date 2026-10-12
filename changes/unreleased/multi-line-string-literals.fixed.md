- A string literal or unrestricted name that spans lines now lexes as the one token the
  KerML `STRING_VALUE` and `UNRESTRICTED_NAME` terminals define, instead of being cut at
  the line break and reported as unterminated with a cascade of parse errors after it. A
  literal the file never closes is reported once, at its opening quote, and taken to end
  with its line so the declarations after it are still read; the REPL and Jupyter kernel
  keep reading a line that leaves such a literal open.
- Document planning and query execution read string literals as the KerML `STRING_VALUE`
  terminal spells them rather than as Go string syntax, so a document title, paragraph text,
  column name or query argument may span lines or escape a single quote.
