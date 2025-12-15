package memory

import (
	"ai-memory/pkg/types"
	"math"
	"time"
)

// DecayCalculator 衰减计算器
type DecayCalculator struct {
	halfLifeDays int
	minScore     float64
}

// NewDecayCalculator 创建衰减计算器
func NewDecayCalculator(halfLifeDays int, minScore float64) *DecayCalculator {
	return &DecayCalculator{
		halfLifeDays: halfLifeDays,
		minScore:     minScore,
	}
}

// CalculateDecayScore 计算衰减分数
// 公式: DecayScore = 0.6 × TimeDecay + 0.4 × FrequencyBonus
// TimeDecay = e^(-(当前时间 - LastAccessAt) / 半衰期)
// FrequencyBonus = min(1.0, AccessCount / 10)
func (d *DecayCalculator) CalculateDecayScore(lastAccessAt time.Time, accessCount int) float64 {
	// 时间衰减
	daysSinceAccess := time.Since(lastAccessAt).Hours() / 24
	halfLifeDecay := math.Exp(-daysSinceAccess / float64(d.halfLifeDays))
	timeDecay := halfLifeDecay

	// 频次加成
	frequencyBonus := math.Min(1.0, float64(accessCount)/10.0)

	// 综合分数
	decayScore := 0.6*timeDecay + 0.4*frequencyBonus

	return decayScore
}

// ShouldEvict 判断是否应被遗忘
func (d *DecayCalculator) ShouldEvict(decayScore float64) bool {
	return decayScore < d.minScore
}

// UpdateMetadataDecay 更新LTMMetadata的衰减分数
func (d *DecayCalculator) UpdateMetadataDecay(metadata *types.LTMMetadata) {
	metadata.DecayScore = d.CalculateDecayScore(metadata.LastAccessAt, metadata.AccessCount)
}

// RefreshAccess 记录一次访问（召回时调用）
func RefreshAccess(metadata *types.LTMMetadata) {
	metadata.LastAccessAt = time.Now()
	metadata.AccessCount++
}
