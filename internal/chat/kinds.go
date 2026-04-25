package chat

const (
	KindSay       = "say"
	KindAsk       = "ask"
	KindAnswer    = "answer"
	KindThinking  = "thinking"
	KindStuck     = "stuck"
	KindMilestone = "milestone"
	KindFYI       = "fyi"
	KindKnock     = "knock"
	KindHandoff   = "handoff"
	KindReviewReq = "review_req"
	KindProgress  = "progress"
	KindDone      = "done"
	KindSystem    = "system"
)

func AllKinds() []string {
	return []string{
		KindSay, KindAsk, KindAnswer,
		KindThinking, KindStuck, KindMilestone, KindFYI,
		KindKnock, KindHandoff, KindReviewReq,
		KindProgress, KindDone, KindSystem,
	}
}

const (
	PriorityNormal = "normal"
	PriorityHigh   = "high"
)

const ThreadGlobal = "global"
