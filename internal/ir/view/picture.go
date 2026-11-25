package view

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/imagefile"
)

// ErrOutsidePicture is why a picture whose file is not under the directory of the model stating it is not read.
var ErrOutsidePicture = errors.New("the file is outside the directory of the model that states it")

// ErrRemotePicture is why a picture located by a URL scheme other than data: is neither drawn nor fetched.
var ErrRemotePicture = errors.New("remote pictures are not drawn")

// RemotePictureLocation reports whether location names a URL scheme other than data:: it holds "://",
// or starts with an RFC 3986 scheme of two or more characters and ':' (one letter is a Windows drive).
func RemotePictureLocation(location string) bool {
	location = strings.TrimSpace(location)
	if strings.HasPrefix(strings.ToLower(location), "data:") {
		return false
	}
	if strings.Contains(location, "://") {
		return true
	}
	colon := strings.IndexByte(location, ':')
	if colon < 2 || !asciiLetter(location[0]) {
		return false
	}
	for i := 1; i < colon; i++ {
		b := location[i]
		if !asciiLetter(b) && !(b >= '0' && b <= '9') && b != '+' && b != '.' && b != '-' {
			return false
		}
	}
	return true
}

func asciiLetter(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z'
}

// CheckPicture is nil when the picture at location may be drawn, else why not. data are the bytes of
// a local file, nil when unread; a data: URL is decoded and checked in their place.
func CheckPicture(location string, data []byte) error {
	location = strings.TrimSpace(location)
	if RemotePictureLocation(location) {
		return ErrRemotePicture
	}
	if strings.HasPrefix(strings.ToLower(location), "data:") {
		mediaType, decoded, err := imagefile.DecodeDataURL(location)
		if err != nil {
			return errors.New("the data: URL does not decode")
		}
		contentType := imagefile.ContentType(decoded)
		if contentType == "" {
			return errors.New("the data: URL is not a supported image")
		}
		if mediaType != contentType {
			if mediaType == "" {
				mediaType = "no media type"
			}
			return fmt.Errorf("the data: URL declares %s but holds %s", mediaType, contentType)
		}
		data = decoded
	}
	if data == nil {
		return nil
	}
	if imagefile.ContentType(data) == "image/svg+xml" || strings.EqualFold(filepath.Ext(location), ".svg") {
		if err := imagefile.CheckStaticSVG(data); err != nil {
			var active *imagefile.ActiveContentError
			if errors.As(err, &active) {
				return active
			}
			return fmt.Errorf("the SVG is not well-formed (%v)", err)
		}
	}
	return nil
}

func pictureRefusal(p Picture) error {
	if RemotePictureLocation(p.Location) || strings.HasPrefix(strings.ToLower(strings.TrimSpace(p.Location)), "data:") {
		return CheckPicture(p.Location, nil)
	}
	if p.outside() {
		return ErrOutsidePicture
	}
	info, err := os.Stat(p.Path())
	if err != nil || !info.Mode().IsRegular() {
		return nil
	}
	data, err := os.ReadFile(p.Path()) // #nosec G304 -- a picture names the file the rendering reads.
	if err != nil {
		return nil
	}
	return CheckPicture(p.Location, data)
}

// outside is whether the file the picture's path opens lies outside the directory of the model
// stating it, links followed; a picture stated in no file has no such directory.
func (p Picture) outside() bool {
	if p.Dir == "" {
		return false
	}
	dir, path := p.Dir, p.Path()
	if cwd, err := os.Getwd(); err == nil {
		if !filepath.IsAbs(dir) {
			dir = filepath.Join(cwd, dir)
		}
		if !filepath.IsAbs(path) {
			path = filepath.Join(cwd, path)
		}
	}
	// A ".." after a link leaves the link's target on one system and the link's own directory
	// on another, so the path is held to the model's directory read both ways.
	return !pathUnder(path, dir) || !pathUnder(filepath.Clean(path), dir)
}

// pathUnder is whether path, links followed, is under dir; a path that does not
// resolve is not read, and its spelling alone says where it would be.
func pathUnder(path, dir string) bool {
	if file, err := filepath.EvalSymlinks(path); err == nil {
		path = file
		if dir, err = filepath.EvalSymlinks(dir); err != nil {
			return false
		}
	}
	rel, err := filepath.Rel(dir, path)
	return err == nil && filepath.IsLocal(rel)
}

func (r *Rendering) pictureRefusals() []error {
	refusals := make([]error, len(r.Pictures))
	for i, picture := range r.Pictures {
		refusals[i] = pictureRefusal(picture)
	}
	return refusals
}

// RefusedPictures states, one notice a reason, the pictures no form draws: the unsafe,
// the remote, and those outside the directory of the model stating them.
func (r *Rendering) RefusedPictures() []string {
	return refusedPictureNotices(r.Pictures, r.pictureRefusals())
}

func refusedPictureNotices(pictures []Picture, refusals []error) []string {
	grouped := make(map[string][]Picture)
	var reasons []string
	for i, picture := range pictures {
		if i >= len(refusals) || refusals[i] == nil {
			continue
		}
		reason := refusals[i].Error()
		if _, ok := grouped[reason]; !ok {
			reasons = append(reasons, reason)
		}
		grouped[reason] = append(grouped[reason], picture)
	}
	notices := make([]string, 0, len(reasons))
	for _, reason := range reasons {
		notices = append(notices, pictureNotice(grouped[reason], reason))
	}
	return notices
}
