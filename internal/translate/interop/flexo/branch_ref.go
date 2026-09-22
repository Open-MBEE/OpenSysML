package flexo

import (
	"fmt"
	"net/url"
	"strings"
)

// BranchRef is one project branch as a command-line URL names it; SysMLV2URL
// is the endpoint the URL spelled out, empty for the configured default.
type BranchRef struct {
	SysMLV2URL string
	Project    string
	Branch     string
}

// ParseBranchURL resolves the branch URL forms a command line accepts, and
// reports ok=false for a string that is a file path rather than a URL:
//
//	http(s)://host[:port][/base]/projects/{project}/branches/{branch}
//	flexo://{project}/{branch}
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

// SameEndpoint reports whether two URLs name the same endpoint: scheme, host
// and effective port equal, path equal once trailing slashes are trimmed.
func SameEndpoint(a, b string) bool {
	ua, err := url.Parse(a)
	if err != nil {
		return false
	}
	ub, err := url.Parse(b)
	if err != nil {
		return false
	}
	if ua.User != nil || ub.User != nil || ua.RawQuery != "" || ub.RawQuery != "" ||
		ua.Fragment != "" || ub.Fragment != "" {
		return false
	}
	port := func(u *url.URL) string {
		if p := u.Port(); p != "" {
			return p
		}
		switch strings.ToLower(u.Scheme) {
		case "http":
			return "80"
		case "https":
			return "443"
		}
		return ""
	}
	return strings.EqualFold(ua.Scheme, ub.Scheme) &&
		strings.EqualFold(ua.Hostname(), ub.Hostname()) &&
		port(ua) == port(ub) &&
		strings.TrimRight(ua.EscapedPath(), "/") == strings.TrimRight(ub.EscapedPath(), "/")
}

// branchURLForm is the shape the http(s) form spells out, quoted into every
// malformed-URL error.
const branchURLForm = "http(s)://host[:port][/base]/projects/{project}/branches/{branch}"

func errBranchURL(raw string) error {
	return fmt.Errorf("%s names no project branch: a repository branch URL is %s, or flexo://{project}/{branch}", raw, branchURLForm)
}
