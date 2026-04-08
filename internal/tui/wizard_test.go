package tui

import (
	"testing"
)

func TestWizardResultEmpty(t *testing.T) {
	result, err := RunWizard(nil)
	if err != nil {
		t.Fatalf("RunWizard(nil) error = %v", err)
	}
	if result.Aborted {
		t.Fatal("empty wizard should not be aborted")
	}
	if len(result.Values) != 0 {
		t.Fatalf("Values = %v, want empty", result.Values)
	}
}

func TestStepStructure(t *testing.T) {
	// Verify step construction compiles and fields are set correctly
	steps := []Step{
		{
			Kind:    StepSelect,
			Key:     "action",
			Title:   "What do you want to do?",
			Options: []PickerItem{{Name: "init", Description: "Create a registry"}},
		},
		{
			Kind:        StepInput,
			Key:         "url",
			Title:       "Git URL",
			Placeholder: "git@github.com:org/repo.git",
		},
		{
			Kind:    StepConfirm,
			Key:     "mount_now",
			Title:   "Mount a profile now?",
			Default: "y",
		},
	}

	if len(steps) != 3 {
		t.Fatal("expected 3 steps")
	}
	if steps[0].Kind != StepSelect {
		t.Fatal("first step should be select")
	}
	if steps[1].Kind != StepInput {
		t.Fatal("second step should be input")
	}
	if steps[2].Kind != StepConfirm {
		t.Fatal("third step should be confirm")
	}
}
