package hygiene

import (
	"strings"
	"testing"
)

// TestREPLCoreLinksNoTranslatorOrTransport keeps a bare REPL session small: the
// translators, protobuf and the network stack reach the REPL only through the
// replext feature packages a binary links. rdf and filename are standard-library
// leaves the layers below translate already share.
func TestREPLCoreLinksNoTranslatorOrTransport(t *testing.T) {
	leaves := map[string]bool{
		module + "/internal/translate/rdf":      true,
		module + "/internal/translate/filename": true,
	}
	for _, dep := range dependencies(t, "./internal/frontend/repl") {
		switch {
		case strings.HasPrefix(dep, module+"/internal/translate/") && !leaves[dep],
			strings.HasPrefix(dep, module+"/api/proto"),
			dep == module+"/internal/frontend/protoconv",
			dep == module+"/internal/frontend/grpc",
			dep == module+"/internal/doc/docpdf",
			dep == "net/http", dep == "crypto/tls",
			strings.HasPrefix(dep, "google.golang.org/"),
			strings.HasPrefix(dep, "connectrpc.com/"):
			t.Errorf("internal/frontend/repl depends on %s", dep)
		}
	}
}
