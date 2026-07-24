package litellm

import "testing"

func TestGeneratedKeyIsSensitiveComputedState(t *testing.T) {
	field := resourceKey().Schema["generated_key"]
	if field == nil || !field.Computed || !field.Sensitive || field.WriteOnly {
		t.Fatal("generated_key must be computed, sensitive, and state-backed")
	}
}
