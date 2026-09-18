package tuner

import (
	"math"
)

// RoundMode 取整模式
type RoundMode string

const (
	// RoundPrecision 精准取整：向上取整到最近整数，追求最贴近目标
	RoundPrecision RoundMode = "precision"
	// RoundNearest 就近阶梯取整：取最接近的阶梯值，可上可下
	RoundNearest RoundMode = "nearest"
	// RoundUp 向上阶梯取整（兼容旧版）：向上找到第一个 >= raw 的阶梯值
	RoundUp RoundMode = "up"
)

// niceStepValues 默认阶梯序列（含中间档位，降低取整偏差）
var niceStepValues = []int{
	1, 2, 5,
	10, 20, 50,
	100, 200, 300, 500,
	1000, 2000, 3000, 4000, 5000,
	10000, 20000, 30000, 50000,
	100000, 200000, 300000, 500000,
	1000000,
}

// CalcExpectHours 计算期望小时数
// expectH = den / (kph * num)
// kph: 每小时真实击杀数
// num: 爆率分子（通常为1）
// den: 爆率分母
func CalcExpectHours(kph float64, probNum, probDen int) float64 {
	if kph <= 0 || probNum <= 0 || probDen <= 0 {
		return 999999
	}
	p := float64(probNum) / float64(probDen)
	if p <= 0 {
		return 999999
	}
	return 1.0 / (kph * p)
}

// RecommendRate 单条推荐结果
type RecommendRate struct {
	MonsterName  string  `json:"monsterName"`
	MonsterIndex int     `json:"monsterIndex"`
	EntryIndex   int     `json:"entryIndex"`
	MapName      string  `json:"mapName"`
	OldNum       int     `json:"oldNum"`
	OldDen       int     `json:"oldDen"`
	OldExpectH   float64 `json:"oldExpectH"`
	NewNum       int     `json:"newNum"`
	NewDen       int     `json:"newDen"`
	NewExpectH   float64 `json:"newExpectH"`
	Deviation    float64 `json:"deviation"` // 偏差百分比
}

// CalcRecommendDenominator 根据目标时间反算推荐分母
// 公式：den_target_raw = den_old * (expectH_target / expectH_old)
// 然后按指定模式取整
func CalcRecommendDenominator(oldDen int, oldExpectH, targetH float64, mode RoundMode) int {
	if oldExpectH <= 0 || targetH <= 0 || oldDen <= 0 {
		return oldDen
	}
	rawDen := float64(oldDen) * (targetH / oldExpectH)
	return RoundDenominator(rawDen, mode)
}

// RoundDenominator 按指定模式取整分母
func RoundDenominator(raw float64, mode RoundMode) int {
	if raw <= 0 {
		return 1
	}
	switch mode {
	case RoundPrecision:
		return roundPrecision(raw)
	case RoundNearest:
		return roundNearestStep(raw)
	case RoundUp, "":
		return roundUpStep(raw)
	default:
		return roundUpStep(raw)
	}
}

// roundPrecision 精准取整：向上取整到最近整数
func roundPrecision(raw float64) int {
	v := int(math.Ceil(raw))
	if v < 1 {
		return 1
	}
	return v
}

// roundNearestStep 就近阶梯取整：取最接近的阶梯值（可上可下）
func roundNearestStep(raw float64) int {
	if raw <= 0 {
		return 1
	}
	// 先扩展阶梯到足够大
	steps := expandSteps(raw)
	best := steps[0]
	bestDist := math.Abs(float64(best) - raw)
	for _, s := range steps {
		d := math.Abs(float64(s) - raw)
		if d < bestDist {
			bestDist = d
			best = s
		}
	}
	return best
}

// roundUpStep 向上阶梯取整（兼容旧版）：向上找到第一个 >= raw 的阶梯值
func roundUpStep(raw float64) int {
	if raw <= 0 {
		return 1
	}
	steps := expandSteps(raw)
	for _, n := range steps {
		if float64(n) >= raw {
			return n
		}
	}
	// 超大值，向上取整到万
	return int(math.Ceil(raw/10000)) * 10000
}

// expandSteps 扩展阶梯序列以覆盖 raw 值
func expandSteps(raw float64) []int {
	// 先用预定义阶梯
	var steps []int
	steps = append(steps, niceStepValues...)
	// 如果 raw 超过最大预定义值，动态扩展
	if raw > float64(niceStepValues[len(niceStepValues)-1]) {
		base := 1000000
		for float64(base) < raw*2 {
			for _, mult := range []int{1, 2, 3, 5} {
				v := base * mult
				if v > 0 {
					steps = append(steps, v)
				}
			}
			base *= 10
		}
	}
	return steps
}

// CalcDeviation 计算偏差百分比
// deviation = (newExpectH - targetH) / targetH * 100%
func CalcDeviation(newExpectH, targetH float64) float64 {
	if targetH <= 0 {
		return 0
	}
	return (newExpectH - targetH) / targetH * 100.0
}