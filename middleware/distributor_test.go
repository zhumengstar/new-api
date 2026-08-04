package middleware

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
)

func TestCanAccessPlaygroundGroup(t *testing.T) {
	if !canAccessPlaygroundGroup(common.RoleCommonUser, "default,private-only", "private-only") {
		t.Fatal("an explicitly assigned private group should be usable in playground")
	}
	if canAccessPlaygroundGroup(common.RoleCommonUser, "default", "private-only") {
		t.Fatal("a common user must not access an unassigned private group")
	}
	if !canAccessPlaygroundGroup(common.RoleRootUser, "default", "private-only") {
		t.Fatal("a root user should be able to access every group in playground")
	}
}

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
