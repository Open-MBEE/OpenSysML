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
