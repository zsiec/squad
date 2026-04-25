package claims

import (
	"context"
	"testing"
	"time"
)

func TestTouchClaim_AdvancesLastTouch(t *testing.T) {
	d, db := newTestStore(t)
	ctx := context.Background()
	_ = d.Claim(ctx, "BUG-080", "agent-a", "", nil, false)

	var t0 int64
	_ = db.QueryRow(`SELECT last_touch FROM claims WHERE item_id='BUG-080'`).Scan(&t0)

	d.now = func() time.Time { return time.Date(2026, 4, 24, 12, 5, 0, 0, time.UTC) }
	if err := d.TouchClaim(ctx, "agent-a"); err != nil {
		t.Fatalf("touch: %v", err)
	}
	var t1 int64
	_ = db.QueryRow(`SELECT last_touch FROM claims WHERE item_id='BUG-080'`).Scan(&t1)
	if t1 <= t0 {
		t.Fatalf("last_touch did not advance: t0=%d t1=%d", t0, t1)
	}
}
