- **`make proto-breaking` reads only `api/proto` from the baseline.** The baseline archive is
  taken from the `api/proto` subtree of `BUF_BREAKING_REF` rather than from the whole commit with
  a pathspec, which walked the whole tree and, from a blobless checkout, lazily fetched every blob
  the commit does not share with the checkout — a fetch CircleCI's checkout cannot always make, so
  the check failed with `could not fetch … from promisor remote` on a merge that touched no
  protobuf file. The cvc5 download in the same pipeline retries a failed transfer instead of
  failing the job on one bad response from the release host.
