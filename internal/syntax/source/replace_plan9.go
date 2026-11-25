package source

// targetNotEmpty reports false: Plan 9 has no error number for a directory with entries,
// and ReplaceFile cannot remove such a target anyway.
func targetNotEmpty(error) bool {
	return false
}
