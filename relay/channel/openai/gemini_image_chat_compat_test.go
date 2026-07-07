package openai

import "testing"

func TestExtractGeminiCompatibleImageResponseAcceptsOpenAIImageResponse(t *testing.T) {
	images := collectOpenAIChatImages([]byte(`{"choices":[{"message":{"images":[{"type":"image_url","image_url":{"url":"data:image/png;base64,aGVsbG8="}}]}}]}`))
	if len(images) != 1 {
		t.Fatalf("image count = %d, want 1", len(images))
	}
	if images[0].B64Json != "aGVsbG8=" {
		t.Fatalf("b64_json = %q, want aGVsbG8=", images[0].B64Json)
	}
}
