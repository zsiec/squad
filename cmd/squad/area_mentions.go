package main

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"time"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/stats"
)

// areaMentionMinCloses is the threshold at which an agent is considered the
// "top closer" of an area for routing purposes. Three closes in 30 days is
// the lowest count that survives random-walk noise — one or two dones in an
// area is plausibly happenstance, three is a pattern.
const areaMentionMinCloses = 3

// areaMentionWindow is the lookback for the top-closer query. Long enough
// to amortize agents who skip a few sessions, short enough to drop stale
// expertise that no longer applies after a refactor.
const areaMentionWindow = 30 * 24 * time.Hour

// notifyAreaTopCloser posts a fyi mentioning the area's top closer to the
// given thread, but only if (a) an agent has ≥ areaMentionMinCloses dones in
// the area over the window, (b) that agent is not the current agent, and
// (c) SQUAD_NO_AREA_MENTIONS is unset. Best-effort: any DB or post error
// is swallowed — this runs as a side-effect of squad new / squad refine
// and must never block the user-visible operation.
func notifyAreaTopCloser(ctx context.Context, db *sql.DB, c *chat.Chat, repoID, currentAgent, area, thread, body string) {
	if os.Getenv("SQUAD_NO_AREA_MENTIONS") == "1" {
		return
	}
	area = strings.TrimSpace(area)
	if area == "" || area == "<fill-in>" {
		return
	}
	since := time.Now().Add(-areaMentionWindow).Unix()
	row, ok, err := stats.TopCloser(ctx, db, repoID, area, since, 0, areaMentionMinCloses)
	if err != nil || !ok {
		return
	}
	if row.AgentID == currentAgent {
		return
	}
	_ = c.Post(ctx, chat.PostRequest{
		AgentID:  currentAgent,
		Thread:   thread,
		Kind:     chat.KindFYI,
		Body:     body,
		Mentions: []string{row.AgentID},
		Priority: chat.PriorityNormal,
	})
}

// notifyAreaChange fires when an item's area is rewritten (e.g. recapture
// after a refine round changed the frontmatter). Suppresses the fyi if the
// new area resolves to the same top closer as the old area — no routing
// signal is lost in that case.
func notifyAreaChange(ctx context.Context, db *sql.DB, c *chat.Chat, repoID, currentAgent, oldArea, newArea, thread, body string) {
	oldArea = strings.TrimSpace(oldArea)
	newArea = strings.TrimSpace(newArea)
	if oldArea == newArea {
		return
	}
	if os.Getenv("SQUAD_NO_AREA_MENTIONS") == "1" {
		return
	}
	if newArea == "" || newArea == "<fill-in>" {
		return
	}
	since := time.Now().Add(-areaMentionWindow).Unix()
	newTop, ok, err := stats.TopCloser(ctx, db, repoID, newArea, since, 0, areaMentionMinCloses)
	if err != nil || !ok || newTop.AgentID == currentAgent {
		return
	}
	if oldArea != "" && oldArea != "<fill-in>" {
		if oldTop, oldOK, _ := stats.TopCloser(ctx, db, repoID, oldArea, since, 0, areaMentionMinCloses); oldOK && oldTop.AgentID == newTop.AgentID {
			return
		}
	}
	_ = c.Post(ctx, chat.PostRequest{
		AgentID:  currentAgent,
		Thread:   thread,
		Kind:     chat.KindFYI,
		Body:     body,
		Mentions: []string{newTop.AgentID},
		Priority: chat.PriorityNormal,
	})
}
