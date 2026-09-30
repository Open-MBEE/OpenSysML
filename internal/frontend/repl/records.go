package repl

import "github.com/Open-MBEE/OpenSysML/internal/workspace/libs"

// SetRecordCache has the session hold the files it loads as the interface
// records cache holds for their content, and write the records of the files it
// analyzes to it; nil holds every file loaded, as a session with no cache does.
// The typed transcript is never recorded.
func (s *Session) SetRecordCache(cache *libs.Cache) {
	defer s.enter()()
	s.ws.SetRecordCache(cache)
}
