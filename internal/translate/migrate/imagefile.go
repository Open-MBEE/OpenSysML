package migrate

import (
	"bytes"
	"net/http"

	"github.com/Open-MBEE/OpenSysML/internal/ir/imagefile"
)

func describedImageContentType(data []byte) string {
	ct := http.DetectContentType(data)
	if t := bytes.TrimSpace(data); bytes.HasPrefix(t, []byte("<")) {
		if err := imagefile.CheckSVG(t); err != nil {
			return ct + "; no SVG document: " + err.Error()
		}
	}
	return ct
}
