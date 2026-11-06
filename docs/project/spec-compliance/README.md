# Spec-compliance sources

The specification-compliance map is published as one page,
[`project/spec-compliance/`](https://runtime.opensysml.org/project/spec-compliance/), and
kept here as one file per area so that a one-line status change is a small commit that
cannot conflict with another branch's row. `scripts/spec_compliance.py` combines the sources
into `docs/project/spec-compliance.md` in path order with a blank line between them; the
combined page is **not committed on `develop`** — the site builds it, and a release renders
and commits it on the release branch (see [releasing.md](../releasing.md)).

## Where to edit

| Area | Source |
|---|---|
| Title, purpose, related documents | `00-preamble.md` |
| Current implementation status (tiers, test inventory) | `01-current-implementation-status.md` |
| Detailed semantic compliance map, one `###` area each | `02-detailed-semantic-compliance-map/NN-<area>.md` (`00-how-to-read-this-map.md` opens the section) |
| What we don't (yet) support | `03-not-yet-supported.md` |
| Implementation files, testing infrastructure | `04-implementation-files.md`, `05-testing-infrastructure.md` |
| Persistence and RDF, source-preserving editing, gRPC, LSP, MCP | `06-` … `10-` |
| Document query, planning, rendering | `11-` … `14-` |
| Analysis engines, constraint solving | `15-analysis-engines.md`, `16-constraint-solving.md` |

`python3 scripts/spec_compliance.py locate "State Machine"` names the source holding a heading;
`python3 scripts/spec_compliance.py sources` lists them in page order.

A source is a file named `<NN>-<slug>.md`, at the top of this directory or one level down in
a `<NN>-<slug>` section directory; the two digits fix the order. Each source starts with a
heading and ends with one newline, and holds exactly the text the page shows at its place, so
anchors and `#fragment` links work as they always did in the combined page. A new `##` section
is a new top-level source; a new `###` area of the semantic map is a new file in its directory.
This `README.md` is the one file here that is not a source.

## Previewing the combined page

```bash
make spec-compliance-render        # writes docs/project/spec-compliance.md (gitignored on develop)
make docs-serve                    # or build the site: the page is generated at project/spec-compliance/
make spec-compliance-check         # the sources are well-formed; a present page is current
```

Do not commit `docs/project/spec-compliance.md` in a pull request against `develop`:
`make spec-compliance-check SPEC_COMPLIANCE_PAGE=forbid` is the check CI runs there, and it
fails when the page is tracked. The release procedure renders and commits it with
`make spec-compliance-render`, and the release pipeline fails when the page is missing or
stale on a tag (`SPEC_COMPLIANCE_PAGE=require`).
