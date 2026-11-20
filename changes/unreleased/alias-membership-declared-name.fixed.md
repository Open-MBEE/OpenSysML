- A query no longer reads an alias's names as the `declaredName` and `declaredShortName` of its
  `Membership`: `alias <s> m for E;` writes `memberName` and `memberShortName` only
  (KerML 8.3.2.4.3), so the two declared properties are absent, as the API-JSON export states them.
