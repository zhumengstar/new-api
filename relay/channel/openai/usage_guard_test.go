package openai

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func newInfoWithEstimate(estimate int) *relaycommon.RelayInfo {
	info := &relaycommon.RelayInfo{}
	info.SetEstimatePromptTokens(estimate)
	return info
}

// 上游 prompt_tokens 远低于本地估算（典型上游截断 / stub 响应）：应用本地估算覆盖。
func TestGuardPromptUndercount_OverrideWhenSeverelyUndercounted(t *testing.T) {
	info := newInfoWithEstimate(200000)
	usage := &dto.Usage{PromptTokens: 6, CompletionTokens: 8, InputTokens: 6}

	guardPromptUndercount(info, usage)

	if usage.PromptTokens != 200000 {
		t.Fatalf("PromptTokens expected 200000, got %d", usage.PromptTokens)
	}
	if usage.InputTokens != 200000 {
		t.Fatalf("InputTokens expected 200000, got %d", usage.InputTokens)
	}
	if usage.CompletionTokens != 8 {
		t.Fatalf("CompletionTokens must not be touched, got %d", usage.CompletionTokens)
	}
	if usage.PromptUndercountUpstream != 6 {
		t.Fatalf("PromptUndercountUpstream must record original upstream value 6, got %d", usage.PromptUndercountUpstream)
	}
}

// 上游 prompt_tokens 与估算接近：不应触发覆盖。
func TestGuardPromptUndercount_NoOverrideWhenClose(t *testing.T) {
	info := newInfoWithEstimate(150)
	usage := &dto.Usage{PromptTokens: 141, CompletionTokens: 16}

	guardPromptUndercount(info, usage)

	if usage.PromptTokens != 141 {
		t.Fatalf("PromptTokens must stay 141, got %d", usage.PromptTokens)
	}
}

// 估算未设置（=0）：不触发，避免误覆盖。
func TestGuardPromptUndercount_NoEstimateNoOverride(t *testing.T) {
	info := newInfoWithEstimate(0)
	usage := &dto.Usage{PromptTokens: 6}

	guardPromptUndercount(info, usage)

	if usage.PromptTokens != 6 {
		t.Fatalf("PromptTokens must stay 6 when no estimate, got %d", usage.PromptTokens)
	}
}

// 上游 PromptTokens=0（未给 usage 或解析失败）：不触发覆盖，保留原有 fallback 逻辑。
func TestGuardPromptUndercount_NoOverrideWhenUpstreamZero(t *testing.T) {
	info := newInfoWithEstimate(20000)
	usage := &dto.Usage{PromptTokens: 0}

	guardPromptUndercount(info, usage)

	if usage.PromptTokens != 0 {
		t.Fatalf("PromptTokens must stay 0 (let other paths handle), got %d", usage.PromptTokens)
	}
}

// 上游报 35%、超过 30% 阈值：不触发覆盖。
func TestGuardPromptUndercount_NoOverrideAtBoundary(t *testing.T) {
	info := newInfoWithEstimate(10000)
	usage := &dto.Usage{PromptTokens: 3500} // 3500/10000 = 0.35

	guardPromptUndercount(info, usage)

	if usage.PromptTokens != 3500 {
		t.Fatalf("PromptTokens must stay 3500 at 35%% ratio, got %d", usage.PromptTokens)
	}
}

// 估算 < 1000 token（小请求）：一律不触发，避免与历史 prompt_tokens=1 误差场景冲突。
func TestGuardPromptUndercount_NoOverrideSmallEstimate(t *testing.T) {
	info := newInfoWithEstimate(60)
	usage := &dto.Usage{PromptTokens: 1}

	guardPromptUndercount(info, usage)

	if usage.PromptTokens != 1 {
		t.Fatalf("PromptTokens must stay 1 when estimate<1000, got %d", usage.PromptTokens)
	}
}

// 估算 >= 1000 但绝对差 <= 500：不触发覆盖，避免敏感度过高。
func TestGuardPromptUndercount_NoOverrideTinyGap(t *testing.T) {
	info := newInfoWithEstimate(1100)
	usage := &dto.Usage{PromptTokens: 700} // gap=400, ratio 0.63

	guardPromptUndercount(info, usage)

	if usage.PromptTokens != 700 {
		t.Fatalf("PromptTokens must stay 700 when gap<=500, got %d", usage.PromptTokens)
	}
}
