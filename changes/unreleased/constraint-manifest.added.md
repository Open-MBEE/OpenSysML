- A machine-readable constraint manifest, `docs/project/constraint-manifest.json`, is now the single
  record behind the validation-constraint census: every pilot-named constraint with its metaclass,
  the clause of the pinned KerML 1.0 / SysML 2.0 specification, the implementing location, the
  diagnostic code its probes observe, its negative case, its census status and a rationale for
  every status but ✅, plus the constraint-tier passes beyond the pilot's names, the declared errata
  and the OMG issues the census bears on. The census table in `validation-constraints.md` and the
  census summary quoted in `spec-compliance.md` are generated from it by `make docs-counts`, whose
  `-check` gates refuse a page or a manifest row edited without regenerating; `go test -C tools
  ./census/validation` holds each probe to a code its row records and each constraint-tier pass to
  a row or an `extensions` entry. It supersedes `validation-constraints-baseline.json`; no status or
  count moved.
