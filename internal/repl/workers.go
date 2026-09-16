package repl

// Workers returns how many files of one load the session parses and analyzes at once.
func (s *Session) Workers() int {
	defer s.reading()()
	return s.ws.Workers()
}

// SetWorkers sets how many files of one load are parsed and analyzed at once;
// a value below one is a typed error. What a load reports is the same at any count.
func (s *Session) SetWorkers(n int) error {
	defer s.enter()()
	return s.ws.SetWorkers(n)
}
