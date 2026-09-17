package lordweb

import (
	"crypto/rand"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"sync"
	"time"
)

//go:embed static
var static embed.FS

const (
	sessionCookie = "lord_session"
	// maxBody bounds a request's JSON; a keypress and a few inputs need far less.
	maxBody = 4096
)

// View is what the browser renders after every request: the warrior as the
// model holds them, the screen the day machine is on, and what just happened.
type View struct {
	Character bool      `json:"character"`
	Location  string    `json:"location"`
	Warrior   *Snapshot `json:"warrior,omitempty"`
	Screen    *Screen   `json:"screen,omitempty"`
	Lines     []string  `json:"lines"`
}

// playRequest is a keypress on the current screen with the inputs its choice asks for.
type playRequest struct {
	Key    string            `json:"key"`
	Inputs map[string]string `json:"inputs"`
}

// session is one browser's game, run under its own lock since a runtime is
// not safe for concurrent use.
type session struct {
	mu       sync.Mutex
	game     *Game
	lastSeen time.Time
}

// Server serves the game over HTTP: a page, and a JSON API keyed by a session cookie.
type Server struct {
	modelSource []byte
	idle        time.Duration
	now         func() time.Time
	seed        func() uint64

	mu       sync.Mutex
	sessions map[string]*session
}

// NewServer serves games of the given model source. A session idle longer than
// idle is dropped when the next request comes in.
func NewServer(modelSource []byte, idle time.Duration) *Server {
	return &Server{
		modelSource: modelSource,
		idle:        idle,
		now:         time.Now,
		seed:        randomSeed,
		sessions:    map[string]*session{},
	}
}

// Handler routes the page and the API.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	pages, err := fs.Sub(static, "static")
	if err != nil {
		panic(err) // the embedded tree is fixed at build time
	}
	mux.Handle("GET /", http.FileServer(http.FS(pages)))
	mux.HandleFunc("GET /api/view", s.handleView)
	mux.HandleFunc("POST /api/new", s.handleNew)
	mux.HandleFunc("POST /api/play", s.handlePlay)
	return mux
}

// Sessions is how many games the server holds.
func (s *Server) Sessions() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

// handleView shows the browser's game, or the character screen when it has none.
func (s *Server) handleView(w http.ResponseWriter, r *http.Request) {
	sess := s.session(r)
	if sess == nil {
		writeJSON(w, http.StatusOK, View{Character: true, Lines: []string{"No warrior of yours walks the realm. Create one."}})
		return
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	view, err := s.view(sess.game, []string{"You are back where you left off."})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// handleNew starts a warrior for the browser, replacing any it had.
func (s *Server) handleNew(w http.ResponseWriter, r *http.Request) {
	var character Character
	if err := readJSON(r, &character); err != nil {
		writeError(w, err)
		return
	}
	game, err := NewGame(s.modelSource, s.seed(), character)
	if err != nil {
		writeError(w, err)
		return
	}
	sess := s.session(r)
	if sess == nil {
		sess = s.newSession(w)
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.game = game
	snapshot, err := game.Snapshot()
	if err != nil {
		writeError(w, err)
		return
	}
	view, err := s.view(game, []string{fmt.Sprintf("Welcome to the realm, %s. It is a fine day to slay the dragon.", snapshot.Name)})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view)
}

// handlePlay presses a key on the browser's game.
func (s *Server) handlePlay(w http.ResponseWriter, r *http.Request) {
	var req playRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	sess := s.session(r)
	if sess == nil {
		writeError(w, fmt.Errorf("%w: no game for this browser", ErrNoSuchCommand))
		return
	}
	sess.mu.Lock()
	defer sess.mu.Unlock()
	outcome, err := sess.game.Play(req.Key, req.Inputs)
	if err != nil {
		writeError(w, err)
		return
	}
	lines := outcome.Narrate(sess.game)
	view, err := s.view(sess.game, lines)
	if err != nil {
		writeError(w, err)
		return
	}
	if len(lines) == 0 {
		if outcome.Moved() {
			view.Lines = []string{fmt.Sprintf("You make your way to %s.", view.Screen.Title)}
		} else {
			view.Lines = []string{"Nothing comes of it."}
		}
	}
	writeJSON(w, http.StatusOK, view)
}

// view projects a game for the browser.
func (s *Server) view(game *Game, lines []string) (*View, error) {
	warrior, err := game.Snapshot()
	if err != nil {
		return nil, err
	}
	screen, err := game.Menu()
	if err != nil {
		return nil, err
	}
	return &View{Location: game.Location(), Warrior: warrior, Screen: screen, Lines: lines}, nil
}

// session finds the browser's session by its cookie, dropping idle ones on the way.
func (s *Server) session(r *http.Request) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	for id, sess := range s.sessions {
		if now.Sub(sess.lastSeen) > s.idle {
			delete(s.sessions, id)
		}
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return nil
	}
	sess, ok := s.sessions[cookie.Value]
	if !ok {
		return nil
	}
	sess.lastSeen = now
	return sess
}

// newSession issues the browser a cookie and a session to hold its game.
func (s *Server) newSession(w http.ResponseWriter) *session {
	s.mu.Lock()
	defer s.mu.Unlock()
	id := newSessionID()
	sess := &session{lastSeen: s.now()}
	s.sessions[id] = sess
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	return sess
}

func newSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // the platform's random source failed; nothing sensible remains
	}
	return hex.EncodeToString(b[:])
}

func randomSeed() uint64 {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	var seed uint64
	for _, x := range b {
		seed = seed<<8 | uint64(x)
	}
	return seed
}

func readJSON(r *http.Request, into any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrBadArgument, err)
	}
	if err := json.Unmarshal(body, into); err != nil {
		return fmt.Errorf("%w: the request is not the JSON expected: %v", ErrBadArgument, err)
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError answers with the error as the game would say it: a player's slip
// is a 4xx, a fault in the model or the server a 500.
func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, ErrNoSuchCommand), errors.Is(err, ErrBadArgument):
		status = http.StatusBadRequest
	case errors.Is(err, ErrNotHere), errors.Is(err, ErrRefused):
		status = http.StatusConflict
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
