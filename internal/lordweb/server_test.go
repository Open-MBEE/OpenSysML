package lordweb

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// browser is one client with its own cookie jar against the test server.
type browser struct {
	t      *testing.T
	client *http.Client
	base   string
}

func newBrowser(t *testing.T, base string) *browser {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	return &browser{t: t, client: &http.Client{Jar: jar}, base: base}
}

func (b *browser) get(path string) (*http.Response, []byte) {
	b.t.Helper()
	resp, err := b.client.Get(b.base + path)
	if err != nil {
		b.t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return resp, body
}

func (b *browser) post(path string, body any) (*http.Response, []byte) {
	b.t.Helper()
	var buf bytes.Buffer
	if s, ok := body.(string); ok {
		buf.WriteString(s)
	} else if err := json.NewEncoder(&buf).Encode(body); err != nil {
		b.t.Fatal(err)
	}
	resp, err := b.client.Post(b.base+path, "application/json", &buf)
	if err != nil {
		b.t.Fatal(err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	return resp, out
}

func (b *browser) view(path string, body any) View {
	b.t.Helper()
	var resp *http.Response
	var out []byte
	if body == nil {
		resp, out = b.get(path)
	} else {
		resp, out = b.post(path, body)
	}
	if resp.StatusCode != http.StatusOK {
		b.t.Fatalf("%s: %d %s", path, resp.StatusCode, out)
	}
	var v View
	if err := json.Unmarshal(out, &v); err != nil {
		b.t.Fatalf("%s: %v in %s", path, err, out)
	}
	return v
}

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	t.Helper()
	s := NewServer(modelSource(t), time.Hour)
	s.seed = func() uint64 { return 42 }
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	return s, ts
}

func TestServerServesThePage(t *testing.T) {
	_, ts := newTestServer(t)
	b := newBrowser(t, ts.URL)
	for path, want := range map[string]string{"/": "text/html", "/lord.js": "text/javascript", "/lord.css": "text/css"} {
		resp, body := b.get(path)
		if resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), want) || len(body) == 0 {
			t.Errorf("%s: %d %s", path, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	}
	if resp, _ := b.get("/nothing"); resp.StatusCode != http.StatusNotFound {
		t.Errorf("/nothing: %d", resp.StatusCode)
	}
}

func TestServerPlaysAGamePerBrowser(t *testing.T) {
	s, ts := newTestServer(t)
	b := newBrowser(t, ts.URL)

	v := b.view("/api/view", nil)
	if !v.Character || v.Warrior != nil {
		t.Fatalf("a new browser should be asked for a character: %+v", v)
	}
	if resp, body := b.post("/api/play", playRequest{Key: "F"}); resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("playing with no game: %d %s", resp.StatusCode, body)
	}

	v = b.view("/api/new", Character{Name: "Jason", Class: "thievingSkills"})
	if v.Character || v.Warrior == nil || v.Warrior.Name != "Jason" || v.Warrior.Class != "thievingSkills" || v.Location != "townSquare" {
		t.Fatalf("new game = %+v", v)
	}
	if v.Screen == nil || v.Screen.Title != "The Town Square" || len(v.Lines) != 1 || !strings.HasPrefix(v.Lines[0], "Welcome to the realm, Jason.") {
		t.Fatalf("new game screen = %+v lines %q", v.Screen, v.Lines)
	}
	if s.Sessions() != 1 {
		t.Fatalf("sessions = %d", s.Sessions())
	}

	v = b.view("/api/play", playRequest{Key: "F"})
	if v.Location != "forest" || v.Lines[0] != "You make your way to The Forest." {
		t.Fatalf("after F: %s %q", v.Location, v.Lines)
	}
	v = b.view("/api/play", playRequest{Key: "L"})
	if v.Warrior.ForestFightsLeft != 14 || !strings.HasPrefix(v.Lines[0], "You have encountered ") {
		t.Fatalf("after L: %+v %q", *v.Warrior, v.Lines)
	}

	v = b.view("/api/view", nil)
	if v.Character || v.Warrior.ForestFightsLeft != 14 || v.Lines[0] != "You are back where you left off." {
		t.Fatalf("reload lost the game: %+v %q", v, v.Lines)
	}
}

func TestServerAnswersSlipsAsTheGameWould(t *testing.T) {
	_, ts := newTestServer(t)
	b := newBrowser(t, ts.URL)
	b.view("/api/new", Character{})
	for _, tc := range []struct {
		body any
		want int
	}{
		{playRequest{Key: "Z"}, http.StatusBadRequest},
		{playRequest{Key: "W"}, http.StatusBadRequest},
		{playRequest{Key: "W", Inputs: map[string]string{"weapon": "town.weapons.excalibur"}}, http.StatusBadRequest},
		{"not json", http.StatusBadRequest},
	} {
		resp, body := b.post("/api/play", tc.body)
		var e struct{ Error string }
		if resp.StatusCode != tc.want || json.Unmarshal(body, &e) != nil || e.Error == "" {
			t.Errorf("%v: %d %s, want %d with an error", tc.body, resp.StatusCode, body, tc.want)
		}
	}
	b.view("/api/play", playRequest{Key: "F"})
	if resp, body := b.post("/api/play", playRequest{Key: "D"}); resp.StatusCode != http.StatusConflict {
		t.Fatalf("seeking the dragon at level 1: %d %s", resp.StatusCode, body)
	}
	if resp, body := b.post("/api/new", Character{Class: "wizard"}); resp.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "wizard") {
		t.Fatalf("a class the model lacks: %d %s", resp.StatusCode, body)
	}
}

func TestServerKeepsBrowsersApart(t *testing.T) {
	s, ts := newTestServer(t)
	a, b := newBrowser(t, ts.URL), newBrowser(t, ts.URL)
	a.view("/api/new", Character{Name: "A"})
	b.view("/api/new", Character{Name: "B", Female: true})
	a.view("/api/play", playRequest{Key: "K"})
	a.view("/api/play", playRequest{Key: "D", Inputs: map[string]string{"amount": "400"}})

	va, vb := a.view("/api/view", nil), b.view("/api/view", nil)
	if va.Warrior.Name != "A" || va.Location != "bank" || va.Warrior.BankGold != 400 {
		t.Fatalf("a = %+v in %s", *va.Warrior, va.Location)
	}
	if vb.Warrior.Name != "B" || vb.Warrior.Sex != "female" || vb.Location != "townSquare" || vb.Warrior.BankGold != 0 {
		t.Fatalf("b saw a's play: %+v in %s", *vb.Warrior, vb.Location)
	}
	if s.Sessions() != 2 {
		t.Fatalf("sessions = %d", s.Sessions())
	}

	vb = b.view("/api/new", Character{Name: "B2"})
	if vb.Warrior.Name != "B2" || s.Sessions() != 2 {
		t.Fatalf("a new character should replace the browser's game: %s, %d sessions", vb.Warrior.Name, s.Sessions())
	}
}

func TestServerDropsIdleGames(t *testing.T) {
	s, ts := newTestServer(t)
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	a, b := newBrowser(t, ts.URL), newBrowser(t, ts.URL)
	a.view("/api/new", Character{Name: "A"})
	now = now.Add(30 * time.Minute)
	b.view("/api/new", Character{Name: "B"})
	now = now.Add(45 * time.Minute)
	if v := b.view("/api/view", nil); v.Character {
		t.Fatal("b's game was dropped within the idle time")
	}
	if v := a.view("/api/view", nil); !v.Character {
		t.Fatal("a's idle game was kept")
	}
	if s.Sessions() != 1 {
		t.Fatalf("sessions = %d", s.Sessions())
	}
}
