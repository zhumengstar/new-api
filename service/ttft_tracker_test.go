package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestTTFTRingBuffer_P95(t *testing.T) {
	buf := newTTFTRingBuffer(20)

	// 样本空
	p95, count := buf.GetP95()
	assert.Equal(t, uint32(0), p95)
	assert.Equal(t, 0, count)

	// 加入 10 个样本：100ms 到 1000ms
	for i := 1; i <= 10; i++ {
		buf.Add(uint32(i * 100))
	}

	p95, count = buf.GetP95()
	assert.Equal(t, 10, count)
	// 第 95% 位应为 900ms 或 1000ms
	assert.True(t, p95 >= 900 && p95 <= 1000, "expected p95 around 900-1000, got %d", p95)
}

func TestTTFTModelTypeDetection(t *testing.T) {
	assert.True(t, IsReasoningModel("claude-3-opus-20240229"))
	assert.True(t, IsReasoningModel("o1-preview"))
	assert.True(t, IsReasoningModel("deepseek-reasoner"))
	assert.True(t, IsReasoningModel("gemini-2.5-pro"))
	assert.False(t, IsReasoningModel("gemini-1.5-flash"))

	assert.True(t, IsFastModel("gemini-1.5-flash"))
	assert.True(t, IsFastModel("gpt-4o-mini"))
	assert.True(t, IsFastModel("claude-3-5-haiku-20241022"))
	assert.False(t, IsFastModel("claude-3-opus"))
}

func TestDynamicFirstByteTimeout(t *testing.T) {
	channelID := 999
	fastModel := "gemini-1.5-flash"

	// 1. 冷启动无样本时，快模型应返回 18s
	timeout := GetDynamicFirstByteTimeout(channelID, fastModel)
	assert.Equal(t, 18*time.Second, timeout)

	// 2. 模拟该渠道该模型 10 次成功请求，出字时间在 1.5s 左右 (1500ms)
	for i := 0; i < 10; i++ {
		RecordTTFT(channelID, fastModel, 1500*time.Millisecond)
	}

	// 3. 计算 P95 = 1500ms，timeout = clamp(1500ms * 1.6 + 3s = 5.4s, minFloor: 10s, maxCeil: 28s) -> 10s
	timeout = GetDynamicFirstByteTimeout(channelID, fastModel)
	assert.Equal(t, 10*time.Second, timeout)

	// 4. 测试慢模型推理
	reasoningModel := "claude-opus-4-6"
	channelID2 := 888
	// 冷启动时思考模型 50s
	timeoutReasoning := GetDynamicFirstByteTimeout(channelID2, reasoningModel)
	assert.Equal(t, 50*time.Second, timeoutReasoning)

	// 模拟该思考模型 P95 在 20s (20000ms)
	for i := 0; i < 10; i++ {
		RecordTTFT(channelID2, reasoningModel, 20000*time.Millisecond)
	}
	// calculated = 20s * 1.5 + 3s = 33s
	timeoutReasoning = GetDynamicFirstByteTimeout(channelID2, reasoningModel)
	assert.Equal(t, 33*time.Second, timeoutReasoning)

	// 5. 测试环境变量强制覆盖 90s
	t.Setenv("STREAMING_FIRST_BYTE_TIMEOUT_OVERRIDE", "90")
	assert.Equal(t, 90*time.Second, GetDynamicFirstByteTimeout(channelID2, reasoningModel))
	assert.Equal(t, 90*time.Second, GetDynamicFirstByteTimeout(channelID, fastModel))
}
