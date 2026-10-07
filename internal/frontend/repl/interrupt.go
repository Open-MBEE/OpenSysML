package repl

import (
	"context"
	"sync"
	"time"
)

// commandContext is the context the command under way runs its plans under: one
// per command, cancelled when the command ends or when Interrupt stops it.
type commandContext struct {
	mu     sync.Mutex
	ctx    context.Context
	cancel context.CancelFunc
}

// begin opens the context of a command; the function returned closes it.
func (c *commandContext) begin() func() {
	ctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.ctx, c.cancel = ctx, cancel
	c.mu.Unlock()
	return func() {
		cancel()
		c.mu.Lock()
		if c.ctx == ctx {
			c.ctx, c.cancel = nil, nil
		}
		c.mu.Unlock()
	}
}

// context is the command's context; outside a command, where nothing is
// there to interrupt, the background one.
func (c *commandContext) context() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

// scoped is a context following whichever command is under way when it is
// consulted, for a plan put from state the session keeps across commands.
func (c *commandContext) scoped() context.Context { return commandScoped{c} }

type commandScoped struct{ c *commandContext }

func (commandScoped) Deadline() (time.Time, bool) { return time.Time{}, false }
func (s commandScoped) Done() <-chan struct{}     { return s.c.context().Done() }
func (s commandScoped) Err() error                { return s.c.context().Err() }
func (commandScoped) Value(any) any               { return nil }

// interrupt cancels the command under way, if any.
func (c *commandContext) interrupt() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
}

// Interrupt stops the command under way: the run it drives fails at its next
// step with runtime.ErrInterrupted, and a plan it put to the engines is
// cancelled. It is safe to call from another goroutine while a command runs,
// which is what it is for; the next command starts unaffected. A command that
// is not running anything — a declaration being analysed, a file being read —
// ends on its own.
func (s *Session) Interrupt() {
	s.interrupt.Store(true)
	s.command.interrupt()
}
