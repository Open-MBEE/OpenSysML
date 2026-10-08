package hygiene

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The native kernel embeds the kernelspec logos and the pip package ships
// them as package data; both must carry the same image.
func TestKernelspecLogosAreTheSameInBothInstallers(t *testing.T) {
	for _, name := range []string{"logo-32x32.png", "logo-64x64.png"} {
		native, err := os.ReadFile(filepath.Join("..", "..", "cmd", "sysml-jupyter-kernel", name))
		if err != nil {
			t.Fatal(err)
		}
		pip, err := os.ReadFile(filepath.Join("..", "..", "client", "jupyter-kernel", "jupyter_opensysml_kernel", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.HasPrefix(native, []byte("\x89PNG\r\n\x1a\n")) {
			t.Errorf("%s is not a PNG", name)
		}
		if !bytes.Equal(native, pip) {
			t.Errorf("%s differs between cmd/sysml-jupyter-kernel and the pip package", name)
		}
	}
}
