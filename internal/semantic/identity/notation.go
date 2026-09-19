package identity

import "fmt"

// ElementIdInline renders an ElementId annotation for the body of the element
// it identifies.
func ElementIdInline(id string) string {
	return fmt.Sprintf("@%s { id = \"%s\"; }", ElementIdFQN, id)
}

// ElementIdAbout renders a standalone ElementId annotation naming its element
// by the qualified path that resolves to it where the annotation is written.
func ElementIdAbout(path, id string) string {
	return fmt.Sprintf("metadata : %s about %s { id = \"%s\"; }", ElementIdFQN, path, id)
}

// ProjectRefInline renders a ProjectRef annotation for the body of the
// namespace it binds.
func ProjectRefInline(projectID string) string {
	return fmt.Sprintf("@%s { projectId = \"%s\"; }", ProjectRefFQN, projectID)
}

// ProjectRefAbout renders a standalone ProjectRef annotation naming the
// namespace it binds by qualified path.
func ProjectRefAbout(path, projectID string) string {
	return fmt.Sprintf("metadata : %s about %s { projectId = \"%s\"; }", ProjectRefFQN, path, projectID)
}
