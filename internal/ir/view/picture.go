package view

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/Open-MBEE/OpenSysML/internal/ir/imagefile"
)

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
		var err error
		data, err = decodeDataPicture(location)
		if err != nil {
			return errors.New("the data: URL does not decode")
		}
		if imagefile.ContentType(data) == "" {
			return errors.New("the data: URL is not a supported image")
		}
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

func decodeDataPicture(location string) ([]byte, error) {
	metadata, payload, ok := strings.Cut(location[len("data:"):], ",")
	if !ok {
		return nil, errors.New("missing data URL payload")
	}
	if strings.HasSuffix(strings.ToLower(metadata), ";base64") {
		data, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, err
		}
		return data, nil
	}
	data, err := url.PathUnescape(payload)
	if err != nil {
		return nil, err
	}
	return []byte(data), nil
}

func pictureRefusal(p Picture) error {
	if RemotePictureLocation(p.Location) || strings.HasPrefix(strings.ToLower(strings.TrimSpace(p.Location)), "data:") {
		return CheckPicture(p.Location, nil)
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

func (r *Rendering) pictureRefusals() []error {
	refusals := make([]error, len(r.Pictures))
	for i, picture := range r.Pictures {
		refusals[i] = pictureRefusal(picture)
	}
	return refusals
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
