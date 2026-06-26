package openai

import "testing"

func TestExtractGeminiCompatibleImageResponseAcceptsOpenAIImageResponse(t *testing.T) {
	imageResponse, _, err := extractGeminiCompatibleImageResponse([]byte(`{"created":1,"data":[{"b64_json":"aGVsbG8="}]}`))
	if err != nil {
		t.Fatalf("extractGeminiCompatibleImageResponse() error = %v", err)
	}
	if len(imageResponse.Data) != 1 {
		t.Fatalf("image count = %d, want 1", len(imageResponse.Data))
	}
	if imageResponse.Data[0].B64Json != "aGVsbG8=" {
		t.Fatalf("b64_json = %q, want aGVsbG8=", imageResponse.Data[0].B64Json)
	}
}
