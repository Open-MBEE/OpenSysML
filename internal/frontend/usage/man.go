package usage

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// ManDir is where the shipped pages live, relative to the repository root.
const ManDir = "man/man1"

// Page renders the command's manual page as the shipped file holds it.
func Page(d Doc, fs *flag.FlagSet) []byte {
	var page bytes.Buffer
	d.WriteRoff(&page, fs, DefaultManMeta())
	return page.Bytes()
}

// CheckShippedPage reports whether the page committed under root still matches
// the description the command renders, naming the command to regenerate it.
func CheckShippedPage(root string, d Doc, fs *flag.FlagSet) error {
	path := filepath.Join(root, ManDir, fmt.Sprintf("%s.%d", d.Command, d.Section()))
	// #nosec G304 -- the path is composed from this package's own layout.
	shipped, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !bytes.Equal(shipped, Page(d, fs)) {
		return fmt.Errorf("%s is not what %s -man now writes; run make man", path, d.Command)
	}
	return nil
}
