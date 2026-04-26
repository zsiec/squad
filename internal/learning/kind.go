package learning

import "fmt"

type Kind string

const (
	KindGotcha  Kind = "gotcha"
	KindPattern Kind = "pattern"
	KindDeadEnd Kind = "dead-end"
)

type State string

const (
	StateProposed State = "proposed"
	StateApproved State = "approved"
	StateRejected State = "rejected"
)

func ParseKind(s string) (Kind, error) {
	switch Kind(s) {
	case KindGotcha, KindPattern, KindDeadEnd:
		return Kind(s), nil
	}
	return "", fmt.Errorf("unknown kind %q (want gotcha | pattern | dead-end)", s)
}

func ParseState(s string) (State, error) {
	switch State(s) {
	case StateProposed, StateApproved, StateRejected:
		return State(s), nil
	}
	return "", fmt.Errorf("unknown state %q", s)
}

func (k Kind) Dir() string { return string(k) + "s" }
