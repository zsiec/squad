package items

import (
	"fmt"
	"regexp"
	"strings"
)

var sectionHeader = regexp.MustCompile(`(?m)^## .+$`)

func WriteFeedback(body, comments string) string {
	body = stripFeedback(body)
	feedback := "## Reviewer feedback\n" + strings.TrimRight(comments, "\n") + "\n\n"
	if idx := strings.Index(body, "## Problem"); idx >= 0 {
		return body[:idx] + feedback + body[idx:]
	}
	return feedback + body
}

func MoveFeedbackToHistory(body, date string) string {
	feedback, rest, ok := extractFeedback(body)
	if !ok {
		return body
	}
	round := nextRoundNumber(rest)
	entry := fmt.Sprintf("### Round %d — %s\n%s\n", round, date, strings.TrimRight(feedback, "\n"))

	if hi := strings.Index(rest, "## Refinement history"); hi >= 0 {
		histEnd := nextSection(rest, hi+len("## Refinement history"))
		return rest[:histEnd] + entry + "\n" + rest[histEnd:]
	}
	header := "## Refinement history\n" + entry + "\n"
	if pi := strings.Index(rest, "## Problem"); pi >= 0 {
		return rest[:pi] + header + rest[pi:]
	}
	return header + rest
}

func stripFeedback(body string) string {
	_, rest, ok := extractFeedback(body)
	if !ok {
		return body
	}
	return rest
}

func extractFeedback(body string) (feedback, rest string, ok bool) {
	hdr := "## Reviewer feedback\n"
	idx := strings.Index(body, hdr)
	if idx < 0 {
		return "", body, false
	}
	contentStart := idx + len(hdr)
	end := nextSection(body, contentStart)
	feedback = body[contentStart:end]
	rest = body[:idx] + body[end:]
	rest = strings.TrimLeft(rest, "\n")
	return feedback, rest, true
}

func nextSection(body string, start int) int {
	loc := sectionHeader.FindStringIndex(body[start:])
	if loc == nil {
		return len(body)
	}
	return start + loc[0]
}

func nextRoundNumber(body string) int {
	re := regexp.MustCompile(`(?m)^### Round (\d+)`)
	matches := re.FindAllStringSubmatch(body, -1)
	max := 0
	for _, m := range matches {
		var n int
		fmt.Sscanf(m[1], "%d", &n)
		if n > max {
			max = n
		}
	}
	return max + 1
}
