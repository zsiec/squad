package chat

const (
	KindSay       = "say"
	KindAsk       = "ask"
	KindThinking  = "thinking"
	KindStuck     = "stuck"
	KindMilestone = "milestone"
	KindFYI       = "fyi"
	KindHandoff   = "handoff"
	KindReviewReq = "review_req"
	KindProgress  = "progress"
	KindDone      = "done"
	KindSystem    = "system"
)

func AllKinds() []string {
	return []string{
		KindSay, KindAsk,
		KindThinking, KindStuck, KindMilestone, KindFYI,
		KindHandoff, KindReviewReq,
		KindProgress, KindDone, KindSystem,
	}
}

const PriorityNormal = "normal"

const ThreadGlobal = "global"
