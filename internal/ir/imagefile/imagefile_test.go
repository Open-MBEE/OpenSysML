package imagefile

import (
	"encoding/base64"
	"errors"
	"net/url"
	"testing"
)

// icon is the header of a Windows icon, an image kind no picture is written as.
const icon = "\x00\x00\x01\x00\x01\x00\x10\x10\x00\x00\x01\x00\x20\x00\x68\x04\x00\x00\x16\x00\x00\x00"

func TestContentTypeReadsTheSignature(t *testing.T) {
	cases := []struct {
		name string
		data string
		want string
	}{
		{"png", "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR", "image/png"},
		{"jpeg", "\xff\xd8\xff\xe0\x00\x10JFIF", "image/jpeg"},
		{"gif87a", "GIF87a\x01\x00\x01\x00", "image/gif"},
		{"gif", "GIF89a\x01\x00\x01\x00", "image/gif"},
		{"bmp", "BM\x36\x00\x00\x00\x00\x00", "image/bmp"},
		{"webp without VP", "RIFF\x24\x00\x00\x00WEBP", ""},
		{"webp VP8", "RIFF\x24\x00\x00\x00WEBPVP8 ", "image/webp"},
		{"svg", `<svg xmlns="http://www.w3.org/2000/svg"/>`, "image/svg+xml"},
		{"svg with prolog", "  <?xml version=\"1.0\"?>\n<svg xmlns=\"http://www.w3.org/2000/svg\"><rect/></svg>", "image/svg+xml"},
		{"svg with a comment first", `<!-- drawn --><svg xmlns="http://www.w3.org/2000/svg"/>`, "image/svg+xml"},
		{"xml that is no svg", `<?xml version="1.0"?><doc/>`, ""},
		{"svg outside its namespace", `<svg/>`, ""},
		{"svg element inside html", `<html><body><svg xmlns="http://www.w3.org/2000/svg"/></body></html>`, ""},
		{"unclosed svg", `<svg xmlns="http://www.w3.org/2000/svg"><rect>`, ""},
		{"two roots", `<svg xmlns="http://www.w3.org/2000/svg"/><svg xmlns="http://www.w3.org/2000/svg"/>`, ""},
		{"text after svg", `<svg xmlns="http://www.w3.org/2000/svg"/>trailing`, ""},
		{"icon, an image not written", icon, ""},
		{"text", "Screen Shot 2013-12-08 at 9.46.18 PM.png", ""},
		{"empty", "", ""},
	}
	for _, c := range cases {
		if got := ContentType([]byte(c.data)); got != c.want {
			t.Errorf("%s: ContentType = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestCheckStaticSVG(t *testing.T) {
	const ns = `xmlns="http://www.w3.org/2000/svg"`
	cases := []struct {
		name, svg, want string
	}{
		{"script", `<svg ` + ns + `><script/></svg>`, "<script>"},
		{"foreignObject", `<svg ` + ns + `><foreignObject/></svg>`, "<foreignObject>"},
		{"onload", `<svg ` + ns + `><rect onload="run()"/></svg>`, "an onload attribute on <rect>"},
		{"external href", `<svg ` + ns + `><a href="https://example.org"/></svg>`, "an external href on <a>"},
		{"javascript href", `<svg ` + ns + `><a href="javascript:run()"/></svg>`, "an external href on <a>"},
		{"external xlink use", `<svg ` + ns + ` xmlns:xlink="http://www.w3.org/1999/xlink"><use xlink:href="other.svg#x"/></svg>`, "an external xlink:href on <use>"},
		{"external xlink image", `<svg ` + ns + ` xmlns:xlink="http://www.w3.org/1999/xlink"><image xlink:href="other.svg"/></svg>`, "an external xlink:href on <image>"},
		{"external feImage", `<svg ` + ns + `><feImage href="http://example.org/image.png"/></svg>`, "an external href on <feImage>"},
		{"iframe", `<svg ` + ns + `><iframe/></svg>`, "<iframe>"},
		{"embed", `<svg ` + ns + `><embed/></svg>`, "<embed>"},
		{"object", `<svg ` + ns + `><object/></svg>`, "<object>"},
		{"handler", `<svg ` + ns + `><handler/></svg>`, "<handler>"},
		{"listener", `<svg ` + ns + `><listener/></svg>`, "<listener>"},
		{"xml-stylesheet", `<?xml-stylesheet href="style.css"?><svg ` + ns + `/>`, "<?xml-stylesheet?>"},
		{"style attribute url", `<svg ` + ns + `><rect style="background:url(http://example.org/a)"/></svg>`, "url() in a style attribute on <rect>"},
		{"style attribute escape", `<svg ` + ns + `><rect style="fill:u\72l(#x)"/></svg>`, "a CSS escape in a style attribute on <rect>"},
		{"style import", `<svg ` + ns + `><style>@import url(x.css);</style></svg>`, "@import in <style>"},
		{"style url", `<svg ` + ns + `><style>.a{fill:url(http://example.org/a)}</style></svg>`, "url() in <style>"},
		{"style url split by CDATA", `<svg ` + ns + `><style>.a{fill:ur<![CDATA[l(http://example.org/a)]]>}</style></svg>`, "url() in <style>"},
		{"css escape", `<svg ` + ns + `><style>.a{fill:u\\72l(#x)}</style></svg>`, "a CSS escape in <style>"},
		{"image-set", `<svg ` + ns + `><style>.a{background:image-set(url(x) 1x)}</style></svg>`, "image-set() in <style>"},
		{"presentation url", `<svg ` + ns + `><rect fill="url(http://example.org/a#g)"/></svg>`, "url() in the fill attribute on <rect>"},
		{"filter url", `<svg ` + ns + `><rect filter="url(https://x/f)"/></svg>`, "url() in the filter attribute on <rect>"},
		{"presentation escape", `<svg ` + ns + `><rect fill="u\72l(http://x/a)"/></svg>`, "a CSS escape in the fill attribute on <rect>"},
		{"xml base", `<svg ` + ns + `><g xml:base="other.svg"/></svg>`, "an xml:base attribute on <g>"},
		{"animate href", `<svg ` + ns + `><set attributeName="href" to="javascript:run()"/></svg>`, "<set> animating href"},
		{"animate event", `<svg ` + ns + `><animate attributeName="onbegin" to="run()"/></svg>`, "<animate> animating onbegin"},
		{"animateTransform href", `<svg ` + ns + `><animateTransform attributeName="xlink:href"/></svg>`, "<animateTransform> animating xlink:href"},
		{"animateMotion event", `<svg ` + ns + `><animateMotion attributeName="onload"/></svg>`, "<animateMotion> animating onload"},
		{"animateColor href", `<svg ` + ns + `><animateColor attributeName="href"/></svg>`, "<animateColor> animating href"},
		{"entity declaration", `<!DOCTYPE svg [<!ENTITY x "y">]><svg ` + ns + `/>`, "an <!ENTITY> declaration"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckStaticSVG([]byte(tc.svg))
			var active *ActiveContentError
			if !errors.As(err, &active) || active.Construct != tc.want {
				t.Fatalf("CheckStaticSVG error = %v, want ActiveContentError(%q)", err, tc.want)
			}
			if got := err.Error(); got != "the SVG has active content ("+tc.want+")" {
				t.Errorf("error = %q", got)
			}
		})
	}

	accepted := []struct {
		name, svg string
	}{
		{"plain shapes and text", `<svg ` + ns + `><rect/><text>hello</text></svg>`},
		{"same document hrefs", `<svg ` + ns + ` xmlns:xlink="http://www.w3.org/1999/xlink"><a href="#x"/><use xlink:href="#x"/></svg>`},
		{"gradient", `<svg ` + ns + `><defs><linearGradient id="grad"/></defs><rect fill="url(#grad)"/></svg>`},
		{"local style reference", `<svg ` + ns + `><rect style="fill:url( '#g')"/></svg>`},
		{"non-CSS attributes and title text", `<svg ` + ns + `><rect aria-label="See url(http://example.org)" id="url(http://example.org)"/><title>url(http://example.org)</title></svg>`},
		{"data image", `<svg ` + ns + `><image href="data:image/png;base64,AAAA"/></svg>`},
		{"standard doctype", `<!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd"><svg ` + ns + `/>`},
	}
	for _, tc := range accepted {
		t.Run(tc.name, func(t *testing.T) {
			if err := CheckStaticSVG([]byte(tc.svg)); err != nil {
				t.Errorf("CheckStaticSVG = %v", err)
			}
		})
	}

	malformed := CheckStaticSVG([]byte(`<svg ` + ns + `><rect></svg>`))
	var active *ActiveContentError
	if malformed == nil || errors.As(malformed, &active) {
		t.Errorf("malformed SVG error = %v, want a non-active-content error", malformed)
	}
	if err := CheckSVG([]byte(`<svg ` + ns + `><script/></svg>`)); err != nil {
		t.Errorf("CheckSVG changed its contract: %v", err)
	}
}

func TestDecodeDataURL(t *testing.T) {
	want := []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`)
	for _, tc := range []struct {
		name, location, mediaType string
		data                      []byte
		wantErr                   bool
	}{
		{"base64", "data:IMAGE/SVG+XML;BASE64," + base64.StdEncoding.EncodeToString(want), "image/svg+xml", want, false},
		{"percent encoded", "data:image/svg+xml," + url.PathEscape(string(want)), "image/svg+xml", want, false},
		{"absent media type", "data:,%3Csvg%2F%3E", "", []byte("<svg/>"), false},
		{"malformed escape", "data:image/png,%ZZ", "image/png", nil, true},
		{"missing comma", "data:image/svg+xml;base64", "image/svg+xml", nil, true},
		{"not a data URL", "https://example.org", "", nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mediaType, data, err := DecodeDataURL(tc.location)
			if mediaType != tc.mediaType || string(data) != string(tc.data) || (err != nil) != tc.wantErr {
				t.Errorf("DecodeDataURL(%q) = (%q, %q, %v), want (%q, %q, error=%v)",
					tc.location, mediaType, data, err, tc.mediaType, tc.data, tc.wantErr)
			}
		})
	}
}

func TestCheckStaticSVGNestedDataSVG(t *testing.T) {
	const ns = `xmlns="http://www.w3.org/2000/svg"`
	dataURL := func(data []byte) string {
		return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString(data)
	}
	svg := func(content string) []byte {
		return []byte(`<svg ` + ns + `>` + content + `</svg>`)
	}
	active := svg("<script/>")
	clean := svg("<rect/>")
	cases := []struct {
		name, source, want string
	}{
		{
			"image with active nested SVG",
			string(svg(`<image href="` + dataURL(active) + `"/>`)),
			"a data: SVG href on <image> with <script>",
		},
		{
			"xlink use with active nested SVG",
			`<svg ` + ns + ` xmlns:xlink="http://www.w3.org/1999/xlink"><use xlink:href="` + dataURL(active) + `"/></svg>`,
			"a data: SVG href on <use> with <script>",
		},
		{
			"undecodable nested SVG",
			string(svg(`<image href="data:image/svg+xml;base64,%%%"/>`)),
			"an undecodable data: href on <image>",
		},
		{
			"malformed nested SVG",
			string(svg(`<image href="data:image/svg+xml;base64,` + base64.StdEncoding.EncodeToString([]byte("<svg")) + `"/>`)),
			"a malformed data: SVG href on <image>",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CheckStaticSVG([]byte(tc.source))
			var active *ActiveContentError
			if !errors.As(err, &active) || active.Construct != tc.want {
				t.Fatalf("CheckStaticSVG error = %v, want ActiveContentError(%q)", err, tc.want)
			}
		})
	}

	for _, tc := range []struct {
		name, location string
	}{
		{"base64 clean", dataURL(clean)},
		{"percent encoded clean", "data:image/svg+xml," + url.PathEscape(string(clean))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := CheckStaticSVG(svg(`<image href="` + tc.location + `"/>`)); err != nil {
				t.Errorf("CheckStaticSVG = %v, want clean nested SVG accepted", err)
			}
		})
	}

	nested := clean
	for i := 0; i < 5; i++ {
		nested = svg(`<image href="` + dataURL(nested) + `"/>`)
	}
	err := CheckStaticSVG(nested)
	var activeError *ActiveContentError
	if !errors.As(err, &activeError) || activeError.Construct != "data: SVG hrefs nested too deeply" {
		t.Errorf("CheckStaticSVG five-deep error = %v, want too-deep ActiveContentError", err)
	}
}

func TestCheckStaticSVGAcceptsURLTextOutsideCSS(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><rect aria-label="url(http://example.org)" id="url(http://example.org)"/><title>url(http://example.org)</title></svg>`
	if err := CheckStaticSVG([]byte(svg)); err != nil {
		t.Errorf("CheckStaticSVG = %v, want URL text in non-CSS content accepted", err)
	}
}

func TestCheckStaticSVGReportsFirstConstruct(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><script/><rect onload="run()"/></svg>`
	err := CheckStaticSVG([]byte(svg))
	var active *ActiveContentError
	if !errors.As(err, &active) || active.Construct != "<script>" {
		t.Fatalf("CheckStaticSVG error = %v, want the first active construct <script>", err)
	}
}

func TestCheckStaticSVGMalformedIsNotActiveContent(t *testing.T) {
	err := CheckStaticSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script></svg>`))
	var active *ActiveContentError
	if err == nil || errors.As(err, &active) {
		t.Fatalf("CheckStaticSVG error = %v, want a malformed-document error", err)
	}
}

func TestNameTakesTheBaseAndTheTypeSuffix(t *testing.T) {
	cases := []struct {
		name, fallback, ct, want string
	}{
		{"Screen Shot 2013-12-08 at 9.46.18 PM.png", "_id", "image/png", "Screen Shot 2013-12-08 at 9.46.18 PM.png"},
		{"C:\\Users\\me\\Pictures\\bench.PNG", "_id", "image/png", "bench.PNG"},
		{"/tmp/photo.jpeg", "_id", "image/jpeg", "photo.jpeg"},
		{"photo.png", "_id", "image/jpeg", "photo.jpg"},
		{"diagram", "_id", "image/svg+xml", "diagram.svg"},
		{"", "_17_0_2_3_41e01aa_1386568017549_527977_60508", "image/png", "_17_0_2_3_41e01aa_1386568017549_527977_60508.png"},
		{"", "", "image/gif", "image.gif"},
		{"..", "/", "image/bmp", "image.bmp"},
		{".hidden", "_id", "image/png", ".hidden.png"},
		{"notes.txt", "_id", "", "notes.txt"},
	}
	for _, c := range cases {
		if got := Name(c.name, c.fallback, c.ct); got != c.want {
			t.Errorf("Name(%q, %q, %q) = %q, want %q", c.name, c.fallback, c.ct, got, c.want)
		}
	}
}
