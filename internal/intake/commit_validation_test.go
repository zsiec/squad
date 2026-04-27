package intake

import (
	"errors"
	"testing"
)

func validItem() ItemDraft {
	return ItemDraft{
		Title:      "Fix the flaky timer test",
		Intent:     "Stabilise the time-based assertion that races on slow CI.",
		Acceptance: []string{"flaky test passes 100/100 runs locally"},
		Area:       "internal/timer",
	}
}

func validSpec() *SpecDraft {
	return &SpecDraft{
		Title:       "Observability v2",
		Motivation:  "Logs are unsearchable; we need structured fields.",
		Acceptance:  []string{"every request emits a trace id", "logs ship to OTLP"},
		NonGoals:    []string{"no metrics overhaul"},
		Integration: []string{"internal/server", "internal/store"},
	}
}

func validEpic(title string) EpicDraft {
	return EpicDraft{
		Title:        title,
		Parallelism:  "parallel",
		Dependencies: []string{},
	}
}

func TestValidate(t *testing.T) {
	checklist, err := LoadChecklist("")
	if err != nil {
		t.Fatalf("load checklist: %v", err)
	}

	cases := []struct {
		name      string
		bundle    Bundle
		mode      string
		wantShape string
		wantErr   error // nil, *IntakeShapeInvalid, or *IntakeIncomplete (compared by type+field)
	}{
		{
			name:      "valid_item_only",
			bundle:    Bundle{Items: []ItemDraft{validItem()}},
			mode:      ModeNew,
			wantShape: ShapeItemOnly,
		},
		{
			name: "valid_spec_epic_items",
			bundle: Bundle{
				Spec:  validSpec(),
				Epics: []EpicDraft{validEpic("Stand up tracing")},
				Items: []ItemDraft{withEpic(validItem(), "Stand up tracing")},
			},
			mode:      ModeNew,
			wantShape: ShapeSpecEpicItems,
		},
		{
			name:    "empty_bundle_no_items",
			bundle:  Bundle{},
			mode:    ModeNew,
			wantErr: &IntakeShapeInvalid{},
		},
		{
			name: "spec_without_epics",
			bundle: Bundle{
				Spec:  validSpec(),
				Items: []ItemDraft{validItem()},
			},
			mode:    ModeNew,
			wantErr: &IntakeShapeInvalid{},
		},
		{
			name: "epics_without_spec",
			bundle: Bundle{
				Epics: []EpicDraft{validEpic("ghost-epic")},
				Items: []ItemDraft{withEpic(validItem(), "ghost-epic")},
			},
			mode:    ModeNew,
			wantErr: &IntakeShapeInvalid{},
		},
		{
			name: "epic_with_no_items_mapped",
			bundle: Bundle{
				Spec: validSpec(),
				Epics: []EpicDraft{
					validEpic("Stand up tracing"),
					validEpic("Logs to OTLP"),
				},
				Items: []ItemDraft{withEpic(validItem(), "Stand up tracing")},
			},
			mode:    ModeNew,
			wantErr: &IntakeShapeInvalid{},
		},
		{
			name: "item_references_unknown_epic",
			bundle: Bundle{
				Spec:  validSpec(),
				Epics: []EpicDraft{validEpic("Stand up tracing")},
				Items: []ItemDraft{withEpic(validItem(), "no-such-epic")},
			},
			mode:    ModeNew,
			wantErr: &IntakeShapeInvalid{},
		},
		{
			name: "item_only_empty_title",
			bundle: Bundle{Items: []ItemDraft{
				{Intent: "x", Acceptance: []string{"y"}, Area: "z"},
			}},
			mode:    ModeNew,
			wantErr: &IntakeIncomplete{Field: "title"},
		},
		{
			name: "item_only_empty_acceptance",
			bundle: Bundle{Items: []ItemDraft{
				{Title: "t", Intent: "x", Area: "z"},
			}},
			mode:    ModeNew,
			wantErr: &IntakeIncomplete{Field: "acceptance"},
		},
		{
			name: "item_only_empty_area",
			bundle: Bundle{Items: []ItemDraft{
				{Title: "t", Intent: "x", Acceptance: []string{"y"}},
			}},
			mode:    ModeNew,
			wantErr: &IntakeIncomplete{Field: "area"},
		},
		{
			name: "spec_epic_items_empty_motivation",
			bundle: func() Bundle {
				s := validSpec()
				s.Motivation = ""
				return Bundle{Spec: s, Epics: []EpicDraft{validEpic("e1")}, Items: []ItemDraft{withEpic(validItem(), "e1")}}
			}(),
			mode:    ModeNew,
			wantErr: &IntakeIncomplete{Field: "spec.motivation"},
		},
		{
			name: "spec_epic_items_empty_epic_parallelism",
			bundle: Bundle{
				Spec: validSpec(),
				Epics: []EpicDraft{
					{Title: "e1", Dependencies: []string{}},
				},
				Items: []ItemDraft{withEpic(validItem(), "e1")},
			},
			mode:    ModeNew,
			wantErr: &IntakeIncomplete{Field: "epic.parallelism"},
		},
		{
			name: "spec_epic_items_empty_item_intent",
			bundle: Bundle{
				Spec:  validSpec(),
				Epics: []EpicDraft{validEpic("e1")},
				Items: []ItemDraft{
					{Title: "t", Acceptance: []string{"y"}, Area: "z", Epic: "e1"},
				},
			},
			mode:    ModeNew,
			wantErr: &IntakeIncomplete{Field: "item.intent"},
		},
		{
			name:      "refine_single_valid_item",
			bundle:    Bundle{Items: []ItemDraft{validItem()}},
			mode:      ModeRefine,
			wantShape: ShapeItemOnly,
		},
		{
			name:    "refine_multi_item_rejected",
			bundle:  Bundle{Items: []ItemDraft{validItem(), validItem()}},
			mode:    ModeRefine,
			wantErr: &IntakeShapeInvalid{},
		},
		{
			name: "refine_with_spec_rejected",
			bundle: Bundle{
				Spec:  validSpec(),
				Epics: []EpicDraft{validEpic("e1")},
				Items: []ItemDraft{withEpic(validItem(), "e1")},
			},
			mode:    ModeRefine,
			wantErr: &IntakeShapeInvalid{},
		},
		{
			name: "spec_title_unslug_safe",
			bundle: func() Bundle {
				s := validSpec()
				s.Title = "!!!"
				return Bundle{Spec: s, Epics: []EpicDraft{validEpic("e1")}, Items: []ItemDraft{withEpic(validItem(), "e1")}}
			}(),
			mode:    ModeNew,
			wantErr: &IntakeIncomplete{Field: "spec.title"},
		},
		{
			name: "epic_title_unslug_safe",
			bundle: Bundle{
				Spec:  validSpec(),
				Epics: []EpicDraft{{Title: "   ", Parallelism: "parallel", Dependencies: []string{}}},
				Items: []ItemDraft{withEpic(validItem(), "   ")},
			},
			mode:    ModeNew,
			wantErr: &IntakeIncomplete{Field: "epic.title"},
		},
		{
			name: "acceptance_with_empty_bullet_rejected",
			bundle: Bundle{Items: []ItemDraft{
				{Title: "t", Intent: "x", Acceptance: []string{"valid", "  "}, Area: "z"},
			}},
			mode:    ModeNew,
			wantErr: &IntakeIncomplete{Field: "acceptance"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shape, err := Validate(tc.bundle, tc.mode, checklist)

			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("got err %v; want nil", err)
				}
				if shape != tc.wantShape {
					t.Fatalf("shape=%q want %q", shape, tc.wantShape)
				}
				return
			}

			if err == nil {
				t.Fatalf("got nil err; want %T", tc.wantErr)
			}
			switch want := tc.wantErr.(type) {
			case *IntakeShapeInvalid:
				var got *IntakeShapeInvalid
				if !errors.As(err, &got) {
					t.Fatalf("err type=%T (%v); want *IntakeShapeInvalid", err, err)
				}
			case *IntakeIncomplete:
				var got *IntakeIncomplete
				if !errors.As(err, &got) {
					t.Fatalf("err type=%T (%v); want *IntakeIncomplete", err, err)
				}
				if got.Field != want.Field {
					t.Fatalf("incomplete field=%q; want %q", got.Field, want.Field)
				}
			}
		})
	}
}

func withEpic(it ItemDraft, epic string) ItemDraft {
	it.Epic = epic
	return it
}

func TestValidate_ErrorMessageFormat(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{
			err:  &IntakeShapeInvalid{Reason: "bundle has no items"},
			want: "intake shape invalid: bundle has no items",
		},
		{
			err:  &IntakeIncomplete{Field: "spec.motivation"},
			want: "intake incomplete: missing or empty spec.motivation",
		},
	}
	for _, tc := range cases {
		if got := tc.err.Error(); got != tc.want {
			t.Errorf("Error() = %q\nwant %q", got, tc.want)
		}
	}
}
