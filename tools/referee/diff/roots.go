package diff

import "slices"

// Root is one corpus directory the differential compares, as the other
// referees reuse it: Name labels the root in reports, Dir is repository-relative
// and Skip lists subdirectories (relative to Dir) left to other roots.
type Root = corpusRoot

// Roots lists the corpus roots the differential compares, in report order.
func Roots() []Root { return slices.Clone(defaultRoots) }

// CollectFiles lists the model files under a root, sorted, relative to it with
// forward slashes; a root absent from the checkout yields none.
func CollectFiles(repo string, root Root) ([]string, error) { return collectFiles(repo, root) }

// LibrariesInput names the provenance input that records the libraries a
// reference is handed.
const LibrariesInput = librariesInput

// LibrariesHandedOver is the OpenSysML library directory a reference is handed
// beside the standard library, as an absolute path: the one inside the
// standard-library root OpenSysML itself loads (see librariesHandedOver).
func LibrariesHandedOver(repo string) (string, error) { return librariesHandedOver(repo) }

// RelativeTo spells path relative to repo with forward slashes, or unchanged
// when it lies outside.
func RelativeTo(repo, path string) string { return relativeTo(repo, path) }
