package codemode

import (
	"context"
	"errors"
)

type cellWait struct {
	done     chan struct{}
	observed bool
	response Response
	err      error
}

// waitCell detaches the observer, not the host operation, on cancellation.
// Host waits drain cell output, so their responses must survive detachment:
// issuing a second wire wait instead would silently lose the first output.
// One observer per cell is permitted. The next observer resumes an abandoned
// wait (including its original yield duration) before starting another one.
func (c *Client) waitCell(ctx context.Context, cellID string, yieldTimeMS uint64) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, err
	}
	if cellID == "" {
		return Response{}, errors.New("code-mode cell ID is required")
	}
	c.mu.Lock()
	if c.err != nil {
		err := c.err
		c.mu.Unlock()
		return Response{}, err
	}
	wait := c.waits[cellID]
	if wait != nil && wait.observed {
		c.mu.Unlock()
		return Response{}, errors.New("code-mode cell already has a wait observer")
	}
	if wait == nil {
		if len(c.waits) >= 128 {
			c.mu.Unlock()
			return Response{}, errors.New("too many unconsumed code-mode waits")
		}
		if c.waits == nil {
			c.waits = make(map[string]*cellWait)
		}
		wait = &cellWait{done: make(chan struct{})}
		c.waits[cellID] = wait
		// Admission and fail share mu, so Close cannot race a new worker Add
		// after it begins joining workers.
		c.workers.Add(1)
		go func() {
			defer c.workers.Done()
			wait.response, wait.err = c.observe(c.ctx, cellID, map[string]any{"method": "session/wait", "sessionId": c.sessionID,
				"request": map[string]any{"cell_id": cellID, "yield_time_ms": yieldTimeMS}})
			close(wait.done)
		}()
	}
	wait.observed = true
	c.mu.Unlock()
	consumed := false
	defer func() {
		c.mu.Lock()
		wait.observed = false
		if consumed {
			delete(c.waits, cellID)
		}
		c.mu.Unlock()
	}()
	select {
	case <-ctx.Done():
		return Response{}, ctx.Err()
	case <-wait.done:
		consumed = true
		return wait.response, wait.err
	}
}
