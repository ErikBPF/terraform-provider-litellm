package litellm

import "testing"

func TestModelModeValidation(t *testing.T) {
	validate := resourceLiteLLMModel().Schema["mode"].ValidateFunc
	for _, mode := range []string{"completion", "embedding", "image_generation", "chat", "moderation", "audio_transcription", "audio_speech", "rerank", "responses"} {
		t.Run(mode, func(t *testing.T) {
			if _, errors := validate(mode, "mode"); len(errors) != 0 {
				t.Fatalf("supported mode rejected: %v", errors)
			}
		})
	}
	if _, errors := validate("unknown", "mode"); len(errors) == 0 {
		t.Fatal("unknown mode accepted")
	}
}
