package flexo

import (
	"fmt"
	"net/url"
	"strings"
)

// BranchRef is one project branch of a running stack, as a command-line URL
// names it. SysMLV2URL is the endpoint the URL spelled out, or empty when the
// shorthand left it to the configured default.
type BranchRef struct {
	SysMLV2URL string
	Project    string
	Branch     string
}

// ParseBranchURL tells a project branch's URL from a file path, and resolves
// the forms a command line accepts into the branch it names:
//
//	http(s)://host[:port][/base]/projects/{project}/branches/{branch}
//	flexo://{project}/{branch}
//
// The http(s) form names the endpoint itself — everything before /projects/
// is the SysML v2 API base URL — while the flexo:// shorthand uses the
// configured endpoint (FLEXO_SYSMLV2_URL, default http://localhost:8083).
// A trailing slash is tolerated; anything else after the branch id is an
// error, as is a URL of either scheme that names no branch. A string that is
// not a URL at all reports ok=false and is left to be read as a file path.
func ParseBranchURL(raw string) (ref BranchRef, ok bool, err error) {
	if rest, found := strings.CutPrefix(raw, "flexo://"); found {
		parts := strings.Split(strings.Trim(rest, "/"), "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return BranchRef{}, false, fmt.Errorf("%s names no project branch: write flexo://{project}/{branch}, or %s", raw, branchURLForm)
		}
		return BranchRef{Project: parts[0], Branch: parts[1]}, true, nil
	}
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		return BranchRef{}, false, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return BranchRef{}, false, fmt.Errorf("the repository URL %s does not parse: %w", raw, err)
	}
	if u.Host == "" || u.RawQuery != "" || u.Fragment != "" {
		return BranchRef{}, false, errBranchURL(raw)
	}
	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segments) < 4 || segments[len(segments)-2] != "branches" || segments[len(segments)-4] != "projects" ||
		segments[len(segments)-3] == "" || segments[len(segments)-1] == "" {
		return BranchRef{}, false, errBranchURL(raw)
	}
	base := strings.Join(segments[:len(segments)-4], "/")
	endpoint := u.Scheme + "://" + u.Host
	if base != "" {
		endpoint += "/" + base
	}
	return BranchRef{SysMLV2URL: endpoint, Project: segments[len(segments)-3], Branch: segments[len(segments)-1]}, true, nil
}

// branchURLForm is the shape the http(s) form spells out, quoted into every
// malformed-URL error.
const branchURLForm = "http(s)://host[:port][/base]/projects/{project}/branches/{branch}"

func errBranchURL(raw string) error {
	return fmt.Errorf("%s names no project branch: a repository branch URL is %s, or flexo://{project}/{branch}", raw, branchURLForm)
}
