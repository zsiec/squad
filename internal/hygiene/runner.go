package hygiene

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Runner struct {
	lockPath string
	window   time.Duration
}

func NewRunner(lockPath string, window time.Duration) *Runner {
	return &Runner{lockPath: lockPath, window: window}
}

// RunIfDue runs work iff the previous run was longer ago than the runner's
// debounce window. Suppressed runs return nil without invoking work.
func (r *Runner) RunIfDue(ctx context.Context, work func(context.Context) error) error {
	now := time.Now()
	if last, err := r.readLast(); err == nil {
		if now.Sub(last) < r.window {
			return nil
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := r.writeLast(now); err != nil {
		return err
	}
	return work(ctx)
}

func (r *Runner) readLast() (time.Time, error) {
	b, err := os.ReadFile(r.lockPath)
	if err != nil {
		return time.Time{}, err
	}
	n, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(n, 0), nil
}

func (r *Runner) writeLast(t time.Time) error {
	tmp := r.lockPath + ".tmp"
	if err := os.WriteFile(tmp, []byte(strconv.FormatInt(t.Unix(), 10)), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, r.lockPath)
}
