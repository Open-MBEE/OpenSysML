package migrate

import (
	"reflect"
	"testing"
)

func TestParseProseText(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []proseRun
	}{
		{"plain text as written", "plain  text\n\twith tabs", []proseRun{{text: "plain  text\n\twith tabs"}}},
		{"nested tags dropped", "<p>The <b>quick <i>brown</i></b> fox.</p>", []proseRun{{text: "The quick brown fox."}}},
		{"entities and nbsp", "<p>a&nbsp;&lt;b&gt;&nbsp; c &amp; d&#39;s</p>", []proseRun{{text: "a <b> c & d's"}}},
		{"breaks kept, spaces around them dropped", "<p>one </p><p> two<br/>three<br>four</p>", []proseRun{{text: "one\ntwo\nthree\nfour"}}},
		{"blank lines bounded", "<p>a</p><p></p><p></p><p></p><p>b</p>", []proseRun{{text: "a\n\nb"}}},
		{"spaces from dropped tags collapse", "<p>the <span>  </span> <a>x</a>  and</p>", []proseRun{{text: "the x and"}}},
		{"style and script dropped", "<html><style>p { color: red }</style><body><p>kept</p><script>alert('<p>')</script></body></html>", []proseRun{{text: "kept"}}},
		{"comments and images dropped", "<p>see <!-- a <b> --><img src=\"x.png\" alt=\"a > b\"> here</p>", []proseRun{{text: "see here"}}},
		{"crlf normalized", "<p>a\r\nb</p>", []proseRun{{text: "a\nb"}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseProse(c.body); !reflect.DeepEqual(got, c.want) {
				t.Errorf("parseProse(%q)\n got %#v\nwant %#v", c.body, got, c.want)
			}
		})
	}
}

func TestParseProseReferences(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []proseRun
	}{
		{
			"mms-cf of each type",
			`<p>See <mms-cf mms-cf-type="name" mms-element-id="_a" class="mceNonEditable">[cf:APS Location.name]</mms-cf>, ` +
				`<mms-cf mms-cf-type="val" mms-element-id="_b">[cf:.val]</mms-cf> and ` +
				`<mms-cf mms-cf-type="com" mms-element-id="_c">[cf:Thing.com]</mms-cf>.</p>`,
			[]proseRun{
				{text: "See "},
				{text: "[cf:APS Location.name]", id: "_a", cf: "name"},
				{text: ", "},
				{text: "[cf:.val]", id: "_b", cf: "val"},
				{text: " and "},
				{text: "[cf:Thing.com]", id: "_c", cf: "com"},
				{text: "."},
			},
		},
		{
			"mdel hyperlink",
			`<p>as part of the <a href="mdel://_1" erit:display="NAME" erit:update="AUTOMATIC_UPDATE">Post Segment-Exchange Alignment</a> and/or <a href='mdel://_2'>Maintenance</a> use cases</p>`,
			[]proseRun{
				{text: "as part of the "},
				{text: "Post Segment-Exchange Alignment", id: "_1", link: true},
				{text: " and/or "},
				{text: "Maintenance", id: "_2", link: true},
				{text: " use cases"},
			},
		},
		{
			"view link",
			`<p>as described in <mms-view-link data-mms-element-id="_v">[cf:Entrance Requirements.vlink]</mms-view-link> .</p>`,
			[]proseRun{
				{text: "as described in "},
				{text: "[cf:Entrance Requirements.vlink]", id: "_v", cf: "vlink"},
				{text: " ."},
			},
		},
		{
			"View Editor view page hyperlink",
			`<p>part of the <span class="ng-scope"> <a target="_self" href="https://ve.example.org/2022ve/index.html#/projects/PROJECT-1/master//present?viewId=_v1&amp;display=document"> <span>Post Segment-Exchange Alignment</span> </a> </span>  and <a href="https://ve.example.org/ve/#/projects/PROJECT-1/master/documents/_d/views/_v2">Maintenance</a> use cases</p>`,
			[]proseRun{
				{text: "part of the "},
				{text: "Post Segment-Exchange Alignment", id: "_v1", link: true},
				{text: " and "},
				{text: "Maintenance", id: "_v2", link: true},
				{text: " use cases"},
			},
		},
		{
			"web hyperlink is text",
			`<p>see <a href="https://example.org">the site</a> now</p>`,
			[]proseRun{{text: "see the site now"}},
		},
		{
			"nested markup inside a reference",
			`<p><a href="mdel://_1"><b>Bold  </b> name</a></p>`,
			[]proseRun{{text: "Bold name", id: "_1", link: true}},
		},
		{
			"reference without an id is text",
			`<p>a <mms-cf mms-cf-type="name">[cf:X.name]</mms-cf> b</p>`,
			[]proseRun{{text: "a [cf:X.name] b"}},
		},
		{
			"both kinds with breaks",
			`<p><mms-cf mms-cf-type="name" mms-element-id="_a">[cf:A.name]</mms-cf></p><p><a href="mdel://_b">B</a></p>`,
			[]proseRun{
				{text: "[cf:A.name]", id: "_a", cf: "name"},
				{text: "\n"},
				{text: "B", id: "_b", link: true},
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := parseProse(c.body); !reflect.DeepEqual(got, c.want) {
				t.Errorf("parseProse(%q)\n got %#v\nwant %#v", c.body, got, c.want)
			}
		})
	}
}

func TestCFFallback(t *testing.T) {
	cases := map[string]string{
		"[cf:APS Location.name]": "APS Location",
		"[cf:.val]":              "",
		"[cf:Table 7-4.name]":    "Table 7-4",
		"cached text":            "cached text",
	}
	for body, want := range cases {
		if got := cfFallback(body); got != want {
			t.Errorf("cfFallback(%q) = %q, want %q", body, got, want)
		}
	}
}

func TestCommentTextWithoutModel(t *testing.T) {
	body := `<p>See <mms-cf mms-cf-type="name" mms-element-id="_a">[cf:APS Location.name]</mms-cf> and ` +
		`<mms-cf mms-cf-type="val" mms-element-id="_b">[cf:.val]</mms-cf> in <a href="mdel://_c">Acquire</a>.</p>`
	if got, want := commentText(body), "See APS Location and in Acquire."; got != want {
		t.Errorf("commentText = %q, want %q", got, want)
	}
}
