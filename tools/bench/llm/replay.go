package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
)

// Fixture is a recorded set of generations: for every cell, the answers the
// model gave in order. Requests are kept only as digests so a fixture stays
// small and carries no prompt text twice; the one exception is the tool specs
// the model was offered, kept once so a replay offers the same ones.
type Fixture struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	// Tools are the MCP tool specs the model saw, as the server described them
	// when the run was recorded; the request digests cover them.
	Tools []ToolSpec             `json:"tools,omitempty"`
	Cells map[string][]Recording `json:"cells"`
}

// Recording is one answer with the digest of the request that produced it.
type Recording struct {
	RequestDigest string   `json:"requestDigest"`
	Response      Response `json:"response"`
}

// CellKey identifies a cell inside a fixture.
func CellKey(c Cell) string { return fmt.Sprintf("%s/%s/%d", c.Task, c.Arm, c.Sample) }

// RequestDigest is the sha256 of the request's canonical JSON.
func RequestDigest(req Request) string {
	data, _ := json.Marshal(req)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

type cellKeyType struct{}

// WithCell marks the cell a request belongs to; the replay and recording
// providers need it to file answers.
func WithCell(ctx context.Context, c Cell) context.Context {
	return context.WithValue(ctx, cellKeyType{}, c)
}

func cellOf(ctx context.Context) (Cell, bool) {
	c, ok := ctx.Value(cellKeyType{}).(Cell)
	return c, ok
}

// ReplayProvider answers from a fixture and never touches the network.
type ReplayProvider struct {
	fixture *Fixture
	mu      sync.Mutex
	next    map[string]int
	// Strict makes a request whose digest differs from the recording an error;
	// otherwise the recorded answer is replayed regardless of prompt drift.
	Strict bool
}

// LoadFixture reads a fixture file.
func LoadFixture(path string) (*Fixture, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- the fixture is the caller's file
	if err != nil {
		return nil, err
	}
	var f Fixture
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if f.Cells == nil {
		f.Cells = map[string][]Recording{}
	}
	return &f, nil
}

// NewReplay builds a provider over a fixture.
func NewReplay(f *Fixture) *ReplayProvider {
	return &ReplayProvider{fixture: f, next: map[string]int{}}
}

func (p *ReplayProvider) Name() string  { return "replay" }
func (p *ReplayProvider) Model() string { return p.fixture.Model }

// Cells lists the recorded cells in key order.
func (p *ReplayProvider) Cells() []string {
	keys := make([]string, 0, len(p.fixture.Cells))
	for k := range p.fixture.Cells {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Complete replays the next recorded answer of the request's cell.
func (p *ReplayProvider) Complete(ctx context.Context, req Request) (Response, error) {
	cell, ok := cellOf(ctx)
	if !ok {
		return Response{}, fmt.Errorf("replay: request carries no cell")
	}
	key := CellKey(cell)
	p.mu.Lock()
	defer p.mu.Unlock()
	recs := p.fixture.Cells[key]
	i := p.next[key]
	if i >= len(recs) {
		return Response{}, fmt.Errorf("replay: %s has %d recorded answers, a %dth was asked for", key, len(recs), i+1)
	}
	if p.Strict && recs[i].RequestDigest != RequestDigest(req) {
		return Response{}, fmt.Errorf("replay: %s answer %d was recorded for a different request", key, i+1)
	}
	p.next[key] = i + 1
	return recs[i].Response, nil
}

// Recorder wraps a live provider and files every answer into a fixture.
type Recorder struct {
	Provider
	mu      sync.Mutex
	Fixture *Fixture
}

// NewRecorder starts recording a provider into a fresh fixture.
func NewRecorder(p Provider) *Recorder {
	return &Recorder{Provider: p, Fixture: &Fixture{Provider: p.Name(), Model: p.Model(), Cells: map[string][]Recording{}}}
}

// Complete forwards the request and records the answer.
func (r *Recorder) Complete(ctx context.Context, req Request) (Response, error) {
	resp, err := r.Provider.Complete(ctx, req)
	if err != nil {
		return resp, err
	}
	if cell, ok := cellOf(ctx); ok {
		key := CellKey(cell)
		r.mu.Lock()
		if r.Fixture.Tools == nil && len(req.Tools) > 0 {
			r.Fixture.Tools = req.Tools
		}
		r.Fixture.Cells[key] = append(r.Fixture.Cells[key], Recording{RequestDigest: RequestDigest(req), Response: resp})
		r.mu.Unlock()
	}
	return resp, nil
}

// Save writes the fixture.
func (r *Recorder) Save(path string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	data, err := json.MarshalIndent(r.Fixture, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}
