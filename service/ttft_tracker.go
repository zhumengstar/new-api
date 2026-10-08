package service

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// 默认每个 (channel, model) 保留最近的样本数
	ttftRingBufferSize = 80
	// 最小样本数：达到此数量才正式采用历史 P95，否则使用模型预设基准
	minSamplesForP95 = 5
)

// TTFTRingBuffer 维护单个维度（如 channel+model）的环形延迟样本池
type TTFTRingBuffer struct {
	mu      sync.RWMutex
	samples []uint32 // 单位：毫秒
	head    int
	count   int
	p95Ms   uint32
	dirty   bool
}

func newTTFTRingBuffer(size int) *TTFTRingBuffer {
	return &TTFTRingBuffer{
		samples: make([]uint32, size),
	}
}

// Add 添加一次成功的出字耗时
func (b *TTFTRingBuffer) Add(ms uint32) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.samples[b.head] = ms
	b.head = (b.head + 1) % len(b.samples)
	if b.count < len(b.samples) {
		b.count++
	}
	b.dirty = true
}

// GetP95 计算并获取 95 分位数耗时（毫秒）以及有效样本总数
func (b *TTFTRingBuffer) GetP95() (uint32, int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.count == 0 {
		return 0, 0
	}

	if !b.dirty {
		return b.p95Ms, b.count
	}

	// 复制有效数据并排序
	valid := make([]uint32, b.count)
	copy(valid, b.samples[:b.count])
	sort.Slice(valid, func(i, j int) bool {
		return valid[i] < valid[j]
	})

	// P95 计算：取第 95% 位
	idx := int(float64(b.count-1) * 0.95)
	if idx >= b.count {
		idx = b.count - 1
	}
	b.p95Ms = valid[idx]
	b.dirty = false

	return b.p95Ms, b.count
}

// TTFTTracker 全局自适应出字延迟评估器
type TTFTTracker struct {
	channelModelPool sync.Map // key: "channelID:model" -> *TTFTRingBuffer
	modelPool        sync.Map // key: "model" -> *TTFTRingBuffer
}

var globalTTFTTracker = &TTFTTracker{}

func cleanModelKey(model string) string {
	m := strings.TrimSpace(strings.ToLower(model))
	// 去掉开头的 custom/ 或 provider/ 等别名前缀（如果有）
	if idx := strings.LastIndex(m, "/"); idx != -1 {
		m = m[idx+1:]
	}
	return m
}

func channelModelKey(channelID int, model string) string {
	return fmt.Sprintf("%d:%s", channelID, cleanModelKey(model))
}

// IsReasoningModel 判断是否为重推理/长思考/深度推导模型
func IsReasoningModel(model string) bool {
	m := cleanModelKey(model)
	reasoningKeywords := []string{
		"o1", "o3", "r1", "reasoning", "opus", "thinking", "gemini-2.5-pro",
		"deepseek-reasoner", "qwq", "kimi-k1.5",
	}
	for _, kw := range reasoningKeywords {
		if strings.Contains(m, kw) {
			return true
		}
	}
	return false
}

// IsFastModel 判断是否为轻量级/快模型（如 Flash、Mini、Haiku 等）
func IsFastModel(model string) bool {
	m := cleanModelKey(model)
	fastKeywords := []string{
		"flash", "mini", "haiku", "turbo", "8b", "nano", "small", "lite",
	}
	for _, kw := range fastKeywords {
		if strings.Contains(m, kw) {
			return true
		}
	}
	return false
}

// RecordTTFT 记录某渠道某模型成功产生首字的耗时
func RecordTTFT(channelID int, model string, d time.Duration) {
	if channelID <= 0 || model == "" || d <= 0 || d > 300*time.Second {
		return
	}
	ms := uint32(d.Milliseconds())
	if ms == 0 {
		ms = 1
	}

	cmKey := channelModelKey(channelID, model)
	val, _ := globalTTFTTracker.channelModelPool.LoadOrStore(cmKey, newTTFTRingBuffer(ttftRingBufferSize))
	val.(*TTFTRingBuffer).Add(ms)

	mKey := cleanModelKey(model)
	mVal, _ := globalTTFTTracker.modelPool.LoadOrStore(mKey, newTTFTRingBuffer(ttftRingBufferSize*2))
	mVal.(*TTFTRingBuffer).Add(ms)
}

// GetChannelModelP95 获取某渠道某模型的 P95 出字耗时及样本数
func GetChannelModelP95(channelID int, model string) (time.Duration, int) {
	cmKey := channelModelKey(channelID, model)
	if val, ok := globalTTFTTracker.channelModelPool.Load(cmKey); ok {
		p95Ms, count := val.(*TTFTRingBuffer).GetP95()
		return time.Duration(p95Ms) * time.Millisecond, count
	}
	return 0, 0
}

// GetModelGlobalP95 获取某模型全局跨渠道的 P95 出字耗时及样本数
func GetModelGlobalP95(model string) (time.Duration, int) {
	mKey := cleanModelKey(model)
	if val, ok := globalTTFTTracker.modelPool.Load(mKey); ok {
		p95Ms, count := val.(*TTFTRingBuffer).GetP95()
		return time.Duration(p95Ms) * time.Millisecond, count
	}
	return 0, 0
}

// GetDynamicFirstByteTimeout 动态评估该渠道该模型的最晚出字超时时间
func GetDynamicFirstByteTimeout(channelID int, model string) time.Duration {
	// 1. 允许环境变量强制覆盖（优先级最高，便于运维应急）
	if val := os.Getenv("STREAMING_FIRST_BYTE_TIMEOUT_OVERRIDE"); val != "" {
		if sec, err := strconv.Atoi(val); err == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}

	// 2. 根据模型类型确定护栏边界与默认冷启动基准
	var (
		minFloor       time.Duration
		maxCeil        time.Duration
		defaultTimeout time.Duration
	)

	if IsReasoningModel(model) {
		// 深度推理/Opus 等：思考耗时较长，给予较宽容的边界，但绝不超过 55s（留出客户端 60s 断开前的切流时间）
		minFloor = 15 * time.Second
		maxCeil = 52 * time.Second
		defaultTimeout = 35 * time.Second
	} else if IsFastModel(model) {
		// Flash/Mini/Haiku 等快模型：正常出字极快，超时护栏需灵敏迅速截断并换渠
		minFloor = 5 * time.Second
		maxCeil = 22 * time.Second
		defaultTimeout = 12 * time.Second
	} else {
		// 普通通用模型（GPT-4o、Sonnet 等）
		minFloor = 8 * time.Second
		maxCeil = 38 * time.Second
		defaultTimeout = 22 * time.Second
	}

	// 3. 优先查本渠道该模型的 P95 统计
	p95, count := GetChannelModelP95(channelID, model)
	if count >= minSamplesForP95 && p95 > 0 {
		// 动态公式：P95 * 1.5 + 2s 缓冲
		calculated := time.Duration(float64(p95)*1.5) + 2*time.Second
		return clampDuration(calculated, minFloor, maxCeil)
	}

	// 4. 若本渠道该模型样本不足（冷启动），回退查该模型全局跨渠道的 P95
	globalP95, gCount := GetModelGlobalP95(model)
	if gCount >= minSamplesForP95 && globalP95 > 0 {
		calculated := time.Duration(float64(globalP95)*1.6) + 3*time.Second
		return clampDuration(calculated, minFloor, maxCeil)
	}

	// 5. 若无足够样本，兼容检查通用配置 STREAMING_FIRST_BYTE_TIMEOUT 或默认值
	if val := os.Getenv("STREAMING_FIRST_BYTE_TIMEOUT"); val != "" {
		if sec, err := strconv.Atoi(val); err == nil && sec > 0 {
			return clampDuration(time.Duration(sec)*time.Second, minFloor, maxCeil)
		}
	}

	return defaultTimeout
}

func clampDuration(val, minD, maxD time.Duration) time.Duration {
	if val < minD {
		return minD
	}
	if val > maxD {
		return maxD
	}
	return val
}
