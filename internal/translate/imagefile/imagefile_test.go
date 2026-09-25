package imagefile

import "testing"

func TestContentTypeReadsTheSignature(t *testing.T) {
	cases := []struct {
		name string
		data string
		want string
	}{
		{"png", "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR", "image/png"},
		{"jpeg", "\xff\xd8\xff\xe0\x00\x10JFIF", "image/jpeg"},
		{"gif", "GIF89a\x01\x00\x01\x00", "image/gif"},
		{"bmp", "BM\x36\x00\x00\x00\x00\x00", "image/bmp"},
		{"webp", "RIFF\x24\x00\x00\x00WEBPVP8 ", "image/webp"},
		{"svg", `<svg xmlns="http://www.w3.org/2000/svg"/>`, "image/svg+xml"},
		{"svg with prolog", "  <?xml version=\"1.0\"?>\n<svg/>", "image/svg+xml"},
		{"xml that is no svg", `<?xml version="1.0"?><doc/>`, ""},
		{"text", "Screen Shot 2013-12-08 at 9.46.18 PM.png", ""},
		{"empty", "", ""},
	}
	for _, c := range cases {
		if got := ContentType([]byte(c.data)); got != c.want {
			t.Errorf("%s: ContentType = %q, want %q", c.name, got, c.want)
		}
	}
	if d := Described([]byte("just words")); d != "text/plain; charset=utf-8" {
		t.Errorf("Described = %q", d)
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
