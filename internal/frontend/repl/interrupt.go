package repl

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// commandContext is the context the command under way runs its plans under: one
// per command, cancelled when the command ends or when Interrupt stops it. It
// raises the flag the runtime polls too, for that command or for one requested
// and waiting to begin, so an interrupt never lands between the two.
type commandContext struct {
	mu      sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
	waiting int          // commands requested that have not begun
	flag    *atomic.Bool // what the runtime polls, lowered when a command ends
}

// request notes a command about to wait for the session, so an interrupt that
// arrives before it begins stops it at its first step rather than vanishing.
func (c *commandContext) request() {
	c.mu.Lock()
	c.waiting++
	c.mu.Unlock()
}

// begin opens the context of a command requested; the function returned closes
// it and lowers the flag, which the next command then starts without.
func (c *commandContext) begin() func() {
	ctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.waiting--
	c.ctx, c.cancel = ctx, cancel
	c.mu.Unlock()
	return func() {
		cancel()
		c.mu.Lock()
		if c.ctx == ctx {
			c.ctx, c.cancel = nil, nil
			c.flag.Store(false)
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

// interrupt cancels the command under way, or marks the one waiting to begin
// interrupted; outside a command, with nothing to stop, it does nothing.
func (c *commandContext) interrupt() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
	if c.cancel != nil || c.waiting > 0 {
		c.flag.Store(true)
	}
}

// Interrupt stops the command under way: the run it drives fails at its next
// step with runtime.ErrInterrupted, and a plan it put to the engines is
// cancelled. It is safe to call from another goroutine while a command runs,
// which is what it is for; the next command starts unaffected. A command
// requested but still waiting for the session is stopped at its first step.
// A command that is not running anything — a declaration being analysed, a
// file being read — ends on its own, and with no command under way or waiting
// there is nothing to stop.
func (s *Session) Interrupt() { s.command.interrupt() }
