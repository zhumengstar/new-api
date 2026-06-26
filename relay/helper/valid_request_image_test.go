package helper

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/gin-gonic/gin"
)

func TestGetAndValidOpenAIImageEditMultipartStrictAspectRatio(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	_ = writer.WriteField("model", "gemini-3.1-flash-image-4k")
	_ = writer.WriteField("prompt", "make it portrait")
	_ = writer.WriteField("size", "2160x3840")
	_ = writer.WriteField("strict_aspect_ratio", "true")
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/images/edits", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = req

	imageRequest, err := GetAndValidOpenAIImageRequest(ctx, relayconstant.RelayModeImagesEdits)
	if err != nil {
		t.Fatalf("GetAndValidOpenAIImageRequest() error = %v", err)
	}
	if imageRequest.StrictAspectRatio == nil || !*imageRequest.StrictAspectRatio {
		t.Fatalf("StrictAspectRatio = %v, want true", imageRequest.StrictAspectRatio)
	}
}
