package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/zsiec/squad/internal/chat"
)

// staleChatThreshold is the silence window above which the agent gets
// a stale-chat nudge — chat-cadence doctrine is "post often, post
// small", and 30m without thinking/milestone/stuck is the threshold
// dogfood data showed correlates with peers losing visibility.
const staleChatThreshold = 30 * time.Minute

// staleChatNudgeText returns the one-line "you've gone quiet" reminder
// when silenceFor crosses the threshold, or "" when below threshold or
// silenced. The CALLER decides invocation cadence — the function is a
// pure text helper, like the other nudges in this file.
func staleChatNudgeText(silenceFor time.Duration) string {
	if cadenceNudgesSilenced() {
		return ""
	}
	if silenceFor < staleChatThreshold {
		return ""
	}
	return "  tip: 30m+ without thinking/milestone/stuck — see `squad-chat-cadence` skill so peers can route attention · silence with SQUAD_NO_CADENCE_NUDGES=1"
}

func printStaleChatNudge(w io.Writer, silenceFor time.Duration) {
	if t := staleChatNudgeText(silenceFor); t != "" {
		fmt.Fprintln(w, t)
	}
}

// maybePrintStaleChatNudge looks up the agent's active claim and
// prints the stale-chat nudge to w when warranted. No-op when the
// agent holds no claim or when silenced. Called from `squad tick` so
// the nudge surfaces on every Bash boundary the loop hook fires on.
func maybePrintStaleChatNudge(ctx context.Context, db *sql.DB, repoID, agentID string, now time.Time, w io.Writer) {
	if cadenceNudgesSilenced() {
		return
	}
	var itemID string
	err := db.QueryRowContext(ctx,
		`SELECT item_id FROM claims WHERE repo_id=? AND agent_id=? LIMIT 1`,
		repoID, agentID,
	).Scan(&itemID)
	if errors.Is(err, sql.ErrNoRows) {
		return
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: stale-chat nudge claim lookup: %v\n", err)
		return
	}
	silenceFor, err := staleChatSilenceFor(ctx, db, repoID, agentID, itemID, now)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: stale-chat nudge silence query: %v\n", err)
		return
	}
	printStaleChatNudge(w, silenceFor)
}

// staleChatSilenceFor returns how long the agent has been quiet on
// cadence verbs, anchored at the latest of (claim time, last
// thinking/milestone/stuck post). Used to feed staleChatNudgeText.
func staleChatSilenceFor(ctx context.Context, db *sql.DB, repoID, agentID, itemID string, now time.Time) (time.Duration, error) {
	var claimedAt int64
	if err := db.QueryRowContext(ctx,
		`SELECT claimed_at FROM claims WHERE repo_id=? AND item_id=? AND agent_id=?`,
		repoID, itemID, agentID,
	).Scan(&claimedAt); err != nil {
		return 0, err
	}
	anchor := time.Unix(claimedAt, 0)

	var latestPost sql.NullInt64
	if err := db.QueryRowContext(ctx,
		`SELECT MAX(ts) FROM messages WHERE repo_id=? AND agent_id=? AND kind IN (?, ?, ?) AND ts >= ?`,
		repoID, agentID, chat.KindThinking, chat.KindMilestone, chat.KindStuck, claimedAt,
	).Scan(&latestPost); err != nil {
		return 0, err
	}
	if latestPost.Valid && latestPost.Int64 > anchor.Unix() {
		anchor = time.Unix(latestPost.Int64, 0)
	}
	return now.Sub(anchor), nil
}

const (
	// timeBoxThreshold90m is the soft cap on exploratory work — at 90m
	// without a recent milestone the agent gets a thinking-prompt to
	// share intent before the cap. squad-time-boxing skill anchors the
	// 2h ceiling.
	timeBoxThreshold90m = 90 * time.Minute
	// timeBoxThreshold120m is the hard cap. At 2h the agent is prompted
	// to hand off or split-and-park; long unsuccessful sessions are a
	// signal, not a setback.
	timeBoxThreshold120m = 120 * time.Minute
	// timeBoxRecentMilestone is the silence window the 90m nudge respects
	// — if a milestone landed in the last 30m the agent is already
	// posting, no need to nag.
	timeBoxRecentMilestone = 30 * time.Minute
)

// timeBoxNudgeText returns the per-threshold reminder, or "" when the
// claim hasn't crossed a threshold or when silenced. The 90m branch is
// silenced when a milestone was posted in the last 30m — the goal is to
// catch silent agents, not to spam noisy ones.
func timeBoxNudgeText(claimAge, sinceLastMilestone time.Duration) string {
	if cadenceNudgesSilenced() {
		return ""
	}
	if claimAge >= timeBoxThreshold120m {
		return "  tip: 2h time-box hit — `squad handoff` or follow `squad-time-boxing` skill to file what's known and claim a smaller follow-up · silence with SQUAD_NO_CADENCE_NUDGES=1"
	}
	if claimAge >= timeBoxThreshold90m && sinceLastMilestone >= timeBoxRecentMilestone {
		return "  tip: 90m on this claim and 30m+ without a milestone — `squad thinking <msg>` to share where you are before the 2h cap · silence with SQUAD_NO_CADENCE_NUDGES=1"
	}
	return ""
}

// maybePrintTimeBoxNudge prints the time-box nudge to w when the calling
// agent's held claim has crossed a threshold and the per-threshold marker
// hasn't fired yet. Markers live on the claims row (nudged_90m_at,
// nudged_120m_at) so they vanish naturally when the claim closes — no
// separate cancellation path needed.
func maybePrintTimeBoxNudge(ctx context.Context, db *sql.DB, repoID, agentID string, now time.Time, w io.Writer) {
	if text := consumeTimeBoxNudge(ctx, db, repoID, agentID, now); text != "" {
		fmt.Fprintln(w, text)
	}
}

// consumeTimeBoxNudge returns the time-box nudge body and stamps the
// matching dedupe marker (nudged_90m_at or nudged_120m_at) when the
// calling agent's held claim has crossed a threshold. Returns "" when
// silenced, when the agent holds no claim, or when no threshold has
// fired. Used by both the tick-driven `maybePrintTimeBoxNudge` and the
// async `squad listen` path; stamping the marker is what keeps the two
// paths from double-firing.
func consumeTimeBoxNudge(ctx context.Context, db *sql.DB, repoID, agentID string, now time.Time) string {
	if cadenceNudgesSilenced() {
		return ""
	}
	var (
		itemID     string
		claimedAt  int64
		nudged90m  sql.NullInt64
		nudged120m sql.NullInt64
	)
	err := db.QueryRowContext(ctx, `
		SELECT item_id, claimed_at, nudged_90m_at, nudged_120m_at
		FROM claims WHERE repo_id=? AND agent_id=? LIMIT 1
	`, repoID, agentID).Scan(&itemID, &claimedAt, &nudged90m, &nudged120m)
	if errors.Is(err, sql.ErrNoRows) {
		return ""
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "warn: time-box nudge claim lookup: %v\n", err)
		return ""
	}
	claimAge := now.Sub(time.Unix(claimedAt, 0))

	// 120m takes priority: at the hard cap we don't also re-emit the 90m
	// nudge. Markers are stamped only after a successful print so a 90m
	// claim silenced by a recent milestone gets a second chance once the
	// milestone window expires — "fire when silent at 90m," not "fire
	// exactly once at the 90m mark."
	if claimAge >= timeBoxThreshold120m && !nudged120m.Valid {
		text := timeBoxNudgeText(claimAge, lastMilestoneSilence(ctx, db, repoID, agentID, claimedAt, now))
		if text != "" {
			_, _ = db.ExecContext(ctx,
				`UPDATE claims SET nudged_120m_at=? WHERE repo_id=? AND agent_id=? AND item_id=?`,
				now.Unix(), repoID, agentID, itemID)
		}
		return text
	}
	// Gate on claimAge < 120m: once the hard cap has been served, the 90m
	// slot is moot. Without this guard, a claim that crosses 90m → 120m
	// without ticking through (silenced by a recent milestone, or no Bash
	// boundary in the window) would re-emit 120m copy on every later tick
	// because timeBoxNudgeText returns 120m text whenever claimAge >= 120m,
	// and would stamp nudged_90m_at under that print, permanently
	// swallowing the 90m thinking-prompt.
	if claimAge >= timeBoxThreshold90m && claimAge < timeBoxThreshold120m && !nudged90m.Valid {
		silence := lastMilestoneSilence(ctx, db, repoID, agentID, claimedAt, now)
		text := timeBoxNudgeText(claimAge, silence)
		if text != "" {
			_, _ = db.ExecContext(ctx,
				`UPDATE claims SET nudged_90m_at=? WHERE repo_id=? AND agent_id=? AND item_id=?`,
				now.Unix(), repoID, agentID, itemID)
		}
		return text
	}
	return ""
}

// lastMilestoneSilence returns how long ago the agent's most recent
// milestone post landed, or a very large duration when none exists since
// claim time (so the threshold check fires).
func lastMilestoneSilence(ctx context.Context, db *sql.DB, repoID, agentID string, claimedAt int64, now time.Time) time.Duration {
	var latest sql.NullInt64
	if err := db.QueryRowContext(ctx,
		`SELECT MAX(ts) FROM messages WHERE repo_id=? AND agent_id=? AND kind=? AND ts >= ?`,
		repoID, agentID, chat.KindMilestone, claimedAt,
	).Scan(&latest); err != nil || !latest.Valid {
		return 24 * time.Hour
	}
	return now.Sub(time.Unix(latest.Int64, 0))
}

// cadenceNudgeText returns the one-line claim/done reminder pointing the
// agent at the right chat verb for the moment, or "" when silenced or when
// no copy applies. The string never carries a trailing newline — the print
// wrappers add it; the MCP handlers carry the bare line into a JSON array.
//
// AGENTS.md tells agents to post on claim, on commit, on done, etc. The
// nudge is the in-flow reminder so the rule reaches the agent without
// requiring them to re-read the manual mid-loop.
func cadenceNudgeText(kind, itemType string) string {
	if cadenceNudgesSilenced() {
		return ""
	}
	switch kind {
	case "claim":
		return "  tip: `squad thinking <msg>` to share intent · silence with SQUAD_NO_CADENCE_NUDGES=1"
	case "done":
		switch itemType {
		case "bug":
			return "  tip: gotcha worth filing? `squad learning propose gotcha <slug>` · silence with SQUAD_NO_CADENCE_NUDGES=1"
		case "feat", "feature", "task":
			return "  tip: surprised by anything? `squad learning propose <kind> <slug>` · silence with SQUAD_NO_CADENCE_NUDGES=1"
		}
	}
	return ""
}

// secondOpinionNudgeText returns the high-stakes-claim peer-check nudge,
// or "" when silenced or when neither priority nor risk warrant it.
// Redirecting at claim-time is much cheaper than at review.
func secondOpinionNudgeText(priority, risk string) string {
	if cadenceNudgesSilenced() {
		return ""
	}
	if priority != "P0" && priority != "P1" && risk != "high" {
		return ""
	}
	return "  tip: high-stakes claim — consider `squad ask @<peer> \"sanity-check my approach?\"` before starting · silence with SQUAD_NO_CADENCE_NUDGES=1"
}

// milestoneTargetNudgeText returns the AC-target nudge naming the AC total
// at claim-time so the agent has a concrete number to compare against while
// working — chat-cadence says "milestone each AC" but the dogfood data
// showed agents posting at most one milestone per item. Empty for 0 or 1
// AC items where a per-AC target adds no signal.
func milestoneTargetNudgeText(acTotal int) string {
	if cadenceNudgesSilenced() {
		return ""
	}
	if acTotal < 2 {
		return ""
	}
	return fmt.Sprintf("  tip: %d AC items — expect ~%d 'squad milestone' posts as you green each one · silence with SQUAD_NO_CADENCE_NUDGES=1", acTotal, acTotal)
}

func printCadenceNudge(w io.Writer, kind string) {
	printCadenceNudgeFor(w, kind, "")
}

func printCadenceNudgeFor(w io.Writer, kind, itemType string) {
	if t := cadenceNudgeText(kind, itemType); t != "" {
		fmt.Fprintln(w, t)
	}
}

func cadenceNudgesSilenced() bool {
	v := os.Getenv("SQUAD_NO_CADENCE_NUDGES")
	return v == "1" || v == "true" || v == "TRUE"
}

func printSecondOpinionNudge(w io.Writer, priority, risk string) {
	if t := secondOpinionNudgeText(priority, risk); t != "" {
		fmt.Fprintln(w, t)
	}
}

func printMilestoneTargetNudge(w io.Writer, acTotal int) {
	if t := milestoneTargetNudgeText(acTotal); t != "" {
		fmt.Fprintln(w, t)
	}
}

// decomposeNudgeText returns the post-claim nudge suggesting `squad
// decompose <ID>` for items whose AC count and distinct file-reference
// count both clear thresholds. Empty when silenced or when either signal
// is below threshold — a 4-bullet item with all bullets in one file is
// not the target case.
func decomposeNudgeText(itemID string, acTotal, fileRefs int) string {
	if cadenceNudgesSilenced() {
		return ""
	}
	if acTotal < 4 || fileRefs < 3 {
		return ""
	}
	return fmt.Sprintf("  tip: %d AC items spanning %d files — consider `squad decompose %s` before starting · silence with SQUAD_NO_CADENCE_NUDGES=1", acTotal, fileRefs, itemID)
}

func printDecomposeNudge(w io.Writer, itemID string, acTotal, fileRefs int) {
	if t := decomposeNudgeText(itemID, acTotal, fileRefs); t != "" {
		fmt.Fprintln(w, t)
	}
}

// worktreeNudgeText returns the post-claim cd hint when the claim provisioned
// an isolated worktree. Empty when silenced or path is empty so the caller
// can branch on a single value.
func worktreeNudgeText(path string) string {
	if cadenceNudgesSilenced() {
		return ""
	}
	if path == "" {
		return ""
	}
	return "  tip: cd into the isolated worktree: cd " + path
}

func printWorktreeNudge(w io.Writer, path string) {
	if t := worktreeNudgeText(path); t != "" {
		fmt.Fprintln(w, t)
	}
}

// quickFollowupNudgeText returns the one-line reminder that the auto-derived
// stub from `squad learning quick` still has placeholder sections worth
// filling in when the agent has a moment. Empty when silenced. The print
// wrapper adds the newline; MCP carries the bare line into Tips.
func quickFollowupNudgeText() string {
	if cadenceNudgesSilenced() {
		return ""
	}
	return "  tip: edit the stub when you can — sections are placeholders · silence with SQUAD_NO_CADENCE_NUDGES=1"
}

func printQuickFollowupNudge(w io.Writer) {
	if t := quickFollowupNudgeText(); t != "" {
		fmt.Fprintln(w, t)
	}
}
