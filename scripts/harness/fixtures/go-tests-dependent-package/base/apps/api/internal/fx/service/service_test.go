package service

import (
	"testing"

	"github.com/aheumaier/harness-template/apps/api/internal/fx/model"
)

func TestAnswer(t *testing.T) {
	if got := model.Answer(); got != 42 {
		t.Fatalf("model.Answer() = %d, want 42", got)
	}
}
