"""
One state entered or transition taken by the explored runs, and how many did: `path`
names it under the instance running the machine (`<object>.<machine>: <state>` or
`<object>.<machine>: <from> -> <to>`), `declaration` under its declaration
(`<machine>::<state>`), and `runs` counts the runs (schedules, for a check).
"""
struct CoverageEntry
    path::String
    declaration::String
    runs::Int
end
CoverageEntry(raw::AbstractDict) = CoverageEntry(String(get(raw, "path", "")),
    String(get(raw, "declaration", "")), Int(get(raw, "runs", 0)))

"""
The states entered and transitions taken across every run of an exploration or every
schedule of a check, not only the witness of each outcome.
"""
struct Coverage
    states::Vector{CoverageEntry}
    transitions::Vector{CoverageEntry}
end
Coverage() = Coverage(CoverageEntry[], CoverageEntry[])
Coverage(raw::AbstractDict) = Coverage(
    CoverageEntry[CoverageEntry(e) for e in get(raw, "states", Any[])],
    CoverageEntry[CoverageEntry(e) for e in get(raw, "transitions", Any[])])
_coverage(raw::AbstractDict) = haskey(raw, "coverage") ? Coverage(raw["coverage"]) : Coverage()
