package grpc

import (
	"connectrpc.com/connect"
	corequery "github.com/Open-MBEE/OpenSysML/internal/semantic/query"
	"github.com/Open-MBEE/OpenSysML/internal/semantic/symbols"
)

type coreQueryModel struct {
	eval   *queryEval
	cached *CachedModel
}

func (m *coreQueryModel) Candidates(scope []string) ([]*symbols.Symbol, error) {
	return m.eval.candidates(m.cached, scope)
}

func (m *coreQueryModel) Value(sym *symbols.Symbol, property string) ([]string, bool) {
	return m.eval.reader.Values(sym, property)
}

func (m *coreQueryModel) Identity(sym *symbols.Symbol) string {
	return m.eval.sc.Index.GetFQN(sym)
}

func (m *coreQueryModel) Type(sym *symbols.Symbol) string {
	return corequery.MetamodelTypeNameOf(sym)
}

// queryStatus reports a refused query as INVALID_ARGUMENT, wrapping the fault
// so a caller can still read its kind.
func queryStatus(err error) error {
	if err == nil {
		return nil
	}
	return connect.NewError(connect.CodeInvalidArgument, err)
}
