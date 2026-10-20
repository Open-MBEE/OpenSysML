package codes

// workspaceCodes are the codes of the checks made of the workspace as a whole:
// how the documents it loads fit together, which no one document's analysis sees.
var workspaceCodes = []Entry{
	{
		Code: "library-declared-twice", Tier: TierWorkspace, Severity: "warning",
		Summary: "a library package declared by more than one document of the workspace",
		Meaning: "Two documents the workspace loads declare the same `library package`: two copies of the standard library under the workspace folder or the library path, a copy of a standard library file beside the bundled one, or a user library vendored twice. " +
			"Every reference to the package resolves to one copy — the first the index registered, which the message names — and the other is unused, so edits to it change nothing; a filtered view or a metaclass lookup reading the duplicated declarations sees both. " +
			"Reported once per pair of trees the copies lie in, on the first file of the unused copy; for copies on the library path, the locations are relative to `OPENSYSML_LIBRARY_PATH`, which the message names. Remove the unused copy, or point `OPENSYSML_LIBRARY_PATH` at one tree.",
		Spec: "",
	},
}
