- Analysing a set of documents (`sysml -validate`, the gRPC `ParseSources` call, a workspace's
  batch) now settles each namespace's import collisions (KerML 7.2.5.4, the hidden colliding
  imports) once for the batch rather than once per document, and skips the route check for an
  entry re-exported through namespaces none of which hides anything. A chain of documents each
  re-exporting the previous one (`public import P::*`) was analysed in time quadratic in the
  chain's depth: 200 such documents took about 270 s and now take about 15–20 s. Each
  document's interface record still depends on everything the shared decision read.
