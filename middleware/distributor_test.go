package middleware

import "testing"

func TestExtractModelNameFromGeminiPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "v1beta generateContent",
			path: "/v1beta/models/gemini-3.1-flash-image:generateContent",
			want: "gemini-3.1-flash-image",
		},
		{
			name: "v1 generateContent",
			path: "/v1/models/gemini-3.1-flash-image:generateContent",
			want: "gemini-3.1-flash-image",
		},
		{
			name: "publisher model",
			path: "/v1beta/models/publishers/google/models/gemini-3.1-flash-image:generateContent",
			want: "publishers/google/models/gemini-3.1-flash-image",
		},
		{
			name: "stream generateContent",
			path: "/v1beta/models/gemini-3.1-flash-image:streamGenerateContent",
			want: "gemini-3.1-flash-image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractModelNameFromGeminiPath(tt.path); got != tt.want {
				t.Fatalf("extractModelNameFromGeminiPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
