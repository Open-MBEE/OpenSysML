# The compact API JSON form (`api-json-compact/1`)

A conversion of a large model to `api-json` is mostly ids and references. The
[element array](rdf-mapping.md) the SysML v2 API serves repeats each element's id in full
wherever another element refers to it, writes every derived property beside the owned one it
restates, and is indented. This page defines a smaller document that says the same thing, for
clients that read a whole model: the `compact` option of `Convert`.

**Status.** `api-json-compact/1` is an OpenSysML format, not a standard. The JSON serialization
of the OMG SysML v2 API is the element array, and the standard form stays the default: nothing
changes for a client that does not ask. This form is defined entirely in terms of that array: a
document expands to an element array by the rules below, so it adds no meaning and can carry
nothing the array cannot. A client that cannot read it asks for `api-json` as before. The form
is versioned in its `format` member, and a reader refuses a version it does not know.

## Why not an existing format

What makes the element array large is repetition of meaning, not of bytes: each id is written
in full at every reference to it (about ten per element), and derived properties restate owned
ones. A generic encoding does not remove that. Measured on the same 46,000-element conversion:

| Encoding | Size | Decode | After gzip |
|---|---|---|---|
| element array, JSON text (the standard form) | 71.8 MB | 126 ms | 2.76 MB |
| the same as CBOR (RFC 8949) | 57.3 MB | 121 ms | 2.75 MB |
| the same as MessagePack | 56.4 MB | 258 ms | 2.74 MB |
| `api-json-compact/1`, JSON text | 13.2 MB | 21 ms | 1.07 MB |
| the same as CBOR | 11.6 MB | 24 ms | 1.06 MB |

A binary encoding saves a fifth of the bytes and no decoding time, and compression
(gzip, RFC 1952, which the service already negotiates) gives the same result for all three
standard-form encodings, because it finds the same repetition a table removes at the source.
The compact form is what makes the document small and quick to read; once it is, a generic
encoding adds nothing worth a second format. RDF serializations (Turtle, which `Convert` also
writes) state each reference as a triple and are larger. CBOR's string-reference extension (tag
256) would deduplicate the ids in a binary document, but it is an Internet-Draft rather than a
standard, and the common libraries do not write it.

## The standards it is built on

The document is JSON (RFC 8259), so any JSON reader reads it. Its shape is defined by a JSON
Schema (draft 2020-12), [`api-json-compact.schema.json`](api-json-compact.schema.json), which the
service's tests validate real output against, and by the expansion rules below, which give its
meaning in terms of the SysML v2 API's element array. Its compression is gzip (RFC 1952), as
the service negotiates it for any response. Nothing in it is specific to a language or
runtime.

## Asking for it

`ConvertRequest.compact` writes `api-json` from notation as this document. It needs the
`convert_compact` capability and is refused (`invalid_argument`) for any other target or for a
source that is not notation. It composes with `documents` and with `id_form`.

| Field | Meaning |
|---|---|
| `compact` | Write the compact document in place of the element array. |
| `omit_derived` | Leave out every derived property of the metamodel except those in `keep_derived`. Needs `compact`. |
| `keep_derived` | Derived properties still written when `omit_derived` is set, named as in the element array (`qualifiedName`). A name that is not a derived property of the metamodel is `invalid_argument`. Needs `omit_derived`. |

A derived property is one the metamodel computes (`derived="true"` in `SysML.ecore`) from
others: an element's `owner` is the other end of its `owningRelationship`, its `member`s are
what its memberships name. A reader that follows the owned properties does not need them, and
a reader that does can list them in `keep_derived`.

## The document

One JSON object, not indented, ending in a newline:

```json
{
  "format": "api-json-compact/1",
  "ids": ["P", "P__m", "P__A", "ScalarValues__Integer"],
  "elementCount": 3,
  "elementIdsAreIds": true,
  "referenceKeys": ["ownedRelatedElement", "owningRelatedElement"],
  "elements": [
    {"@type": "Namespace", "ownedRelationship": [{"@id": 1}]},
    {"@type": "OwningMembership", "owningRelatedElement": 0, "ownedRelatedElement": [2]},
    {"@type": "PartDefinition", "declaredName": "A"}
  ]
}
```

| Member | Type | Meaning |
|---|---|---|
| `format` | string | `api-json-compact/1`. |
| `ids` | array of string | The id table: every element id the document uses, each once. A *handle* is an index into it. |
| `elementCount` | integer | `ids[0 … elementCount)` are the written elements, in the order the element array writes them. Entries from `elementCount` on are elements a written element refers to that this conversion does not write: a library element, or an element of a document that `documents` left out. |
| `elementIdsAreIds` | boolean, optional | Present (and `true`) only when every element's `elementId` is its `@id`. Then no element writes `elementId`. |
| `referenceKeys` | array of string | Keys whose references are written as bare handles (below). |
| `elements` | array of object | The elements. Element *i* has the id `ids[i]` and carries no `@id`. |

## Reading it

A reader turns the document into the element array by these rules and needs nothing else.

1. Element *i* is an object whose `@id` is `ids[i]`, whose `@type` is its `@type`, and, if
   `elementIdsAreIds` is `true`, whose `elementId` is `ids[i]`.
2. Under a key in `referenceKeys`, a value is a handle (an array of handles for a collection);
   each handle *h* becomes `{"@id": ids[h]}`.
3. Under any other key, an object `{"@id": h}` with an integer *h* becomes `{"@id": ids[h]}`.
   This is how a reference is spelled under a key that also carries numbers.
4. Every other value is unchanged: strings, booleans, numbers, `{"@ref": name}` (a name the
   graph could not link), and arrays of them.

The document above expands to (`ScalarValues__Integer`, the fourth id, is referenced by nothing
shown here; it stands for a library element the conversion does not write):

```json
[
  {"@type": "Namespace", "@id": "P", "elementId": "P", "ownedRelationship": [{"@id": "P__m"}]},
  {"@type": "OwningMembership", "@id": "P__m", "elementId": "P__m",
   "owningRelatedElement": {"@id": "P"}, "ownedRelatedElement": [{"@id": "P__A"}]},
  {"@type": "PartDefinition", "@id": "P__A", "elementId": "P__A", "declaredName": "A"}
]
```

(The example is trimmed: real documents carry every property the element array does, minus
what `omit_derived` leaves out.)

## What the writer guarantees

- **It is the element array, expanded.** Without `omit_derived`, the expansion equals the array
  `Convert` writes for the same request, element for element and key for key. With it, the
  expansion equals that array minus the derived properties not in `keep_derived`. This is
  tested on a model of several documents, with and without `documents`.
- **`referenceKeys` is exact.** A key is listed only if it carries at least one reference in
  the document and no number. A reader never has to guess whether an integer is a handle.
- **The id table is complete.** Every handle is a valid index; ids are unique except that an
  id two elements share (which the element array also writes twice) appears once per element.
- **Order is the element array's.** Elements, and the members of a collection, keep the order
  the element array has.

## Size

A project of 55 documents over the libraries it imports (about 46,000 elements, 580 KB of
source), `Convert` with `documents` naming the project's own documents:

| Form | Bytes | gzip (level 6) |
|---|---|---|
| element array, as written today | 71.8 MB | 2.8 MB |
| `compact` | 22.9 MB | not measured |
| `compact` + `omit_derived` (keeping three properties) | 17.8 MB | not measured |
| the same, with `elementId` left out | 13.2 MB | 1.1 MB |

Most of what remains is the id table (4.2 MB), the `@type` of each element (1.2 MB) and the
source text a document records beside an element. The conversion's time is not in the writing:
the service spends about 1.0 s of a 2.2 s conversion of this model building the model's graph and
about 0.6 s more building the element objects, so the saving is in what a client receives and parses (JSON parse 160 ms to 35 ms here), not
in the service.

## Compression

The service answers a Connect request that sends `Accept-Encoding: gzip` with a gzipped body,
and a client that accepts it receives the compact document at under a tenth of its size again.
Over a local connection the compression is neither a gain nor a cost that shows in the
conversion's time (2.0 s with and without it for the element array, measured), so a client
that reads from a service on the same machine can decline it; across a network it is worth
having, and the compact document compresses to under half of what the element array does.
The format itself has no compression of its own: a client that stores the document should
compress it with the tool it already has.

## Limits

- The form is written from notation only, as `api-json` is for `id_form` and `documents`.
- It does not change what the element array means; it cannot carry anything the array cannot.
- `@type` is written in full for every element. A type table would save about 7% more and is
  not worth a second indirection.
