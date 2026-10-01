package modeldoc_test

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

func openDoc(t *testing.T, name, src string) *model.Workspace {
	t.Helper()
	ws := model.NewWorkspace()
	ws.Open(name, []byte(src), 1)
	return ws
}
