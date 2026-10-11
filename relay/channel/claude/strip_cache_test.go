package claude

import (
	"net/http/httptest"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestStripClaudeMessageStartCacheData(t *testing.T) {
	raw := `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-3-7-sonnet-20250219","usage":{"input_tokens":100,"cache_creation_input_tokens":50,"cache_read_input_tokens":1500,"cache_creation":{"ephemeral_5m_input_tokens":50,"ephemeral_1h_input_tokens":0}}}}`
	cleaned := stripClaudeMessageStartCacheData(raw)

	assert.Equal(t, int64(1650), gjson.Get(cleaned, "message.usage.input_tokens").Int())
	assert.Equal(t, int64(0), gjson.Get(cleaned, "message.usage.cache_read_input_tokens").Int())
	assert.Equal(t, int64(0), gjson.Get(cleaned, "message.usage.cache_creation_input_tokens").Int())
	assert.False(t, gjson.Get(cleaned, "message.usage.cache_creation").Exists())
}

func TestStripClaudeMessageDeltaCacheData(t *testing.T) {
	raw := `{"type":"message_delta","delta":{"stop_reason":"end_turn","stop_sequence":null},"usage":{"output_tokens":25,"cache_creation_input_tokens":50,"cache_read_input_tokens":1500,"input_tokens":100,"cache_creation":{"ephemeral_5m_input_tokens":50}}}`
	cleaned := stripClaudeMessageDeltaCacheData(raw)

	assert.Equal(t, int64(1650), gjson.Get(cleaned, "usage.input_tokens").Int())
	assert.Equal(t, int64(0), gjson.Get(cleaned, "usage.cache_read_input_tokens").Int())
	assert.Equal(t, int64(0), gjson.Get(cleaned, "usage.cache_creation_input_tokens").Int())
	assert.Equal(t, int64(25), gjson.Get(cleaned, "usage.output_tokens").Int())
	assert.False(t, gjson.Get(cleaned, "usage.cache_creation").Exists())
}

func TestStripClaudeResponseCacheBytes(t *testing.T) {
	raw := []byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"hello"}],"model":"claude-3-7-sonnet-20250219","usage":{"input_tokens":200,"output_tokens":10,"cache_creation_input_tokens":100,"cache_read_input_tokens":800,"cache_creation":{"ephemeral_5m_input_tokens":100}}}`)
	cleaned := stripClaudeResponseCacheBytes(raw)

	assert.Equal(t, int64(1100), gjson.GetBytes(cleaned, "usage.input_tokens").Int())
	assert.Equal(t, int64(0), gjson.GetBytes(cleaned, "usage.cache_read_input_tokens").Int())
	assert.Equal(t, int64(0), gjson.GetBytes(cleaned, "usage.cache_creation_input_tokens").Int())
	assert.Equal(t, int64(10), gjson.GetBytes(cleaned, "usage.output_tokens").Int())
	assert.False(t, gjson.GetBytes(cleaned, "usage.cache_creation").Exists())
}

func TestHandleStreamResponseData_StripsCacheForCCAntiGravityGroup(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		UsingGroup:  "cc反重力按量",
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	claudeInfo := &ClaudeResponseInfo{
		Usage: &dto.Usage{},
	}

	startRaw := `{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-3-7-sonnet-20250219","usage":{"input_tokens":100,"cache_creation_input_tokens":50,"cache_read_input_tokens":1500}}}`
	err := HandleStreamResponseData(c, info, claudeInfo, startRaw)
	require.Nil(t, err)

	body := w.Body.String()
	assert.Contains(t, body, `"input_tokens":1650`)
	assert.Contains(t, body, `"cache_read_input_tokens":0`)
	assert.Contains(t, body, `"cache_creation_input_tokens":0`)

	// 验证 claudeInfo.Usage 同步更新且内部无缓存残留
	assert.Equal(t, 1650, claudeInfo.Usage.PromptTokens)
	assert.Equal(t, 0, claudeInfo.Usage.PromptTokensDetails.CachedTokens)
	assert.Equal(t, 0, claudeInfo.Usage.PromptTokensDetails.CachedCreationTokens)
}

func TestHandleClaudeResponseData_StripsCacheForCCAntiGravityGroupNonStream(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		UsingGroup:  "cc反重力按量",
		ChannelMeta: &relaycommon.ChannelMeta{},
	}
	claudeInfo := &ClaudeResponseInfo{
		Usage: &dto.Usage{},
	}

	raw := []byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"hi"}],"model":"claude-3-7-sonnet-20250219","usage":{"input_tokens":200,"output_tokens":10,"cache_creation_input_tokens":100,"cache_read_input_tokens":800}}`)
	httpResp := httptest.NewRecorder().Result()
	httpResp.Header.Set("Content-Type", "application/json")

	err := HandleClaudeResponseData(c, info, claudeInfo, httpResp, raw)
	require.Nil(t, err)

	body := w.Body.String()
	assert.Equal(t, int64(1100), gjson.Get(body, "usage.input_tokens").Int())
	assert.Equal(t, int64(0), gjson.Get(body, "usage.cache_read_input_tokens").Int())
	assert.Equal(t, int64(0), gjson.Get(body, "usage.cache_creation_input_tokens").Int())
	assert.Equal(t, int64(10), gjson.Get(body, "usage.output_tokens").Int())

	// 验证 claudeInfo 也被规整
	assert.Equal(t, 1100, claudeInfo.Usage.PromptTokens)
	assert.Equal(t, 0, claudeInfo.Usage.PromptTokensDetails.CachedTokens)
}

func TestHandleClaudeResponseData_StripsCacheForChannelSetting(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	info := &relaycommon.RelayInfo{
		RelayFormat: types.RelayFormatClaude,
		UsingGroup:  "default",
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelOtherSettings: dto.ChannelOtherSettings{
				StripClaudeResponseCache: true,
			},
		},
	}
	claudeInfo := &ClaudeResponseInfo{
		Usage: &dto.Usage{},
	}

	raw := []byte(`{"id":"msg_1","type":"message","role":"assistant","content":[{"type":"text","text":"hi"}],"model":"claude-3-7-sonnet-20250219","usage":{"input_tokens":50,"output_tokens":5,"cache_read_input_tokens":500}}`)
	httpResp := httptest.NewRecorder().Result()
	httpResp.Header.Set("Content-Type", "application/json")

	err := HandleClaudeResponseData(c, info, claudeInfo, httpResp, raw)
	require.Nil(t, err)

	body := w.Body.String()
	assert.Equal(t, int64(550), gjson.Get(body, "usage.input_tokens").Int())
	assert.Equal(t, int64(0), gjson.Get(body, "usage.cache_read_input_tokens").Int())
}
