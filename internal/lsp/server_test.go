package lsp

import (
	"testing"

	"github.com/Open-MBEE/OpenSysML/internal/workspace/model"
)

func TestNewServerWrapsWorkspace(t *testing.T) {
	ws := model.NewWorkspace()
	s := NewServer(ws)
	if s == nil {
		t.Fatal("NewServer returned nil")
	}
	if s.ws != ws {
		t.Fatal("server does not hold the given workspace")
	}
}
