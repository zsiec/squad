package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/zsiec/squad/internal/chat"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/learning"
)

// surpriseMatchKeywords drives the KindFYI body filter. Keep the set small
// — every keyword adds noise candidates the agent has to reject downstream.
var surpriseMatchKeywords = []string{
	"surprise",
	"surprised",
	"didn't expect",
	"turns out",
	"wait",
}

const defaultMaxProposals = 5

type surpriseCandidate struct {
	Body string
	Area string
}

type proposalDraft struct {
	Slug   string
	Title  string
	Area   string
	Path   string
	DryRun bool
}

func matchesSurprise(body string) bool {
	low := strings.ToLower(body)
	for _, kw := range surpriseMatchKeywords {
		if strings.Contains(low, kw) {
			return true
		}
	}
	return false
}

// minSubstringDedupLen is the floor below which substring containment is
// too coarse to be a real near-duplicate signal. A body like "wait" would
// otherwise swallow every later candidate that contains the substring
// anywhere. Below this length, only exact-match dedup applies.
const minSubstringDedupLen = 12

// dedupSurprises drops near-duplicates by lowercase substring containment.
// Bodies shorter than minSubstringDedupLen only dedup against an exact
// match — short stop-word-ish entries don't subsume meatier ones.
func dedupSurprises(in []surpriseCandidate) []surpriseCandidate {
	var out []surpriseCandidate
	for _, c := range in {
		body := strings.TrimSpace(c.Body)
		if body == "" {
			continue
		}
		low := strings.ToLower(body)
		dup := false
		for _, kept := range out {
			keptLow := strings.ToLower(kept.Body)
			if keptLow == low {
				dup = true
				break
			}
			shorter, longer := keptLow, low
			if len(longer) < len(shorter) {
				shorter, longer = longer, shorter
			}
			if len(shorter) >= minSubstringDedupLen && strings.Contains(longer, shorter) {
				dup = true
				break
			}
		}
		if !dup {
			out = append(out, surpriseCandidate{Body: body, Area: c.Area})
		}
	}
	return out
}

// gatherSurprises returns candidate surprises for handoff-time learning
// proposals. Sources:
//   - explicit slice (paired with the first held-claim's area as fallback);
//   - else mined from chat history of held claims: every KindStuck plus
//     every KindFYI whose body contains a surprise keyword.
//
// Result is deduped and capped at max. The bool is true when the cap clipped
// the result so callers can render a one-line warning.
func gatherSurprises(
	ctx context.Context,
	db *sql.DB,
	c *chat.Chat,
	repoID, agentID, itemsDir string,
	explicit []string,
	max int,
) ([]surpriseCandidate, bool, error) {
	if max <= 0 {
		max = defaultMaxProposals
	}

	heldItems, err := heldClaimItems(ctx, db, repoID, agentID)
	if err != nil {
		return nil, false, err
	}

	areaFor := func(itemID string) string {
		path := findItemPath(itemsDir, itemID)
		if path == "" {
			return "general"
		}
		it, perr := items.Parse(path)
		if perr != nil || it.Area == "" {
			return "general"
		}
		return it.Area
	}

	fallbackArea := "general"
	if len(heldItems) > 0 {
		fallbackArea = areaFor(heldItems[0])
	}

	var raw []surpriseCandidate
	if len(explicit) > 0 {
		for _, s := range explicit {
			if t := strings.TrimSpace(s); t != "" {
				raw = append(raw, surpriseCandidate{Body: t, Area: fallbackArea})
			}
		}
	} else {
		for _, itemID := range heldItems {
			area := areaFor(itemID)
			entries, herr := c.History(ctx, itemID)
			if herr != nil {
				continue
			}
			for _, e := range entries {
				body := strings.TrimSpace(e.Body)
				if body == "" {
					continue
				}
				switch e.Kind {
				case chat.KindStuck:
					raw = append(raw, surpriseCandidate{Body: body, Area: area})
				case chat.KindFYI:
					if matchesSurprise(body) {
						raw = append(raw, surpriseCandidate{Body: body, Area: area})
					}
				}
			}
		}
	}

	deduped := dedupSurprises(raw)
	clipped := false
	if len(deduped) > max {
		deduped = deduped[:max]
		clipped = true
	}
	return deduped, clipped, nil
}

func heldClaimItems(ctx context.Context, db *sql.DB, repoID, agentID string) ([]string, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT item_id FROM claims WHERE repo_id = ? AND agent_id = ? ORDER BY claimed_at`,
		repoID, agentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, nil
}

// proposeFromSurprises walks the surprise list and writes one gotcha-kind
// learning stub per surprise (skipping writes when dryRun). On slug
// collisions it tries baseSlug-2, baseSlug-3, ... up to the same cap as
// learning quick. Surprises whose body produces a too-short slug or hits
// the collision ceiling are silently skipped — callers see them missing
// from the returned list.
func proposeFromSurprises(
	ctx context.Context,
	repoRoot, agentID, sessionID string,
	surprises []surpriseCandidate,
	dryRun bool,
) ([]proposalDraft, error) {
	// Snapshot existing slugs once so dry-run previews and live writes
	// agree on collision resolution. The live path also walks Walk() per
	// LearningPropose call, so the snapshot is for preview parity only.
	used := map[string]bool{}
	if dryRun {
		all, werr := learning.Walk(repoRoot)
		if werr == nil {
			for _, l := range all {
				used[l.Slug] = true
			}
		}
	}

	var out []proposalDraft
	for _, s := range surprises {
		base := deriveQuickSlug(s.Body)
		if base == "" {
			continue
		}
		title := s.Body
		if len(title) > 80 {
			title = title[:80]
		}

		if dryRun {
			slug := base
			ok := false
			for suffix := 1; suffix <= quickCollisionMaxSuffix; suffix++ {
				if suffix > 1 {
					slug = fmt.Sprintf("%s-%d", base, suffix)
				}
				if !used[slug] {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
			used[slug] = true
			out = append(out, proposalDraft{
				Slug:   slug,
				Title:  title,
				Area:   s.Area,
				Path:   learning.PathFor(repoRoot, learning.KindGotcha, learning.StateProposed, slug),
				DryRun: true,
			})
			continue
		}

		slug := base
		var (
			res  *LearningProposeResult
			perr error
		)
		for suffix := 1; suffix <= quickCollisionMaxSuffix; suffix++ {
			if suffix > 1 {
				slug = fmt.Sprintf("%s-%d", base, suffix)
			}
			res, perr = LearningPropose(ctx, LearningProposeArgs{
				RepoRoot:  repoRoot,
				Kind:      "gotcha",
				Slug:      slug,
				Title:     title,
				Area:      s.Area,
				SessionID: sessionID,
				CreatedBy: agentID,
				Via:       "handoff-propose",
				Looks:     s.Body,
			})
			if perr == nil {
				break
			}
			var coll *SlugCollisionError
			if !errors.As(perr, &coll) {
				return out, perr
			}
		}
		if perr != nil || res == nil {
			continue
		}
		out = append(out, proposalDraft{
			Slug:  res.Learning.Slug,
			Title: title,
			Area:  s.Area,
			Path:  res.Path,
		})
	}
	return out, nil
}
