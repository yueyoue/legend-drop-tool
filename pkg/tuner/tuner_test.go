package tuner

import (
	"testing"
)

func TestCalcRecommendDenominator(t *testing.T) {
	tests := []struct {
		name     string
		oldDen   int
		oldExpH  float64
		targetH  float64
		mode     RoundMode
		wantDen  int
	}{
		{"方向修正-变长", 100, 1.0, 5.0, RoundPrecision, 500},
		{"方向修正-变短", 500, 5.0, 1.0, RoundPrecision, 100},
		{"精准取整-3782", 3000, 11.9, 15.0, RoundPrecision, 3782},
		{"就近阶梯-3782", 3000, 11.9, 15.0, RoundNearest, 4000},
		{"向上阶梯-3782", 3000, 11.9, 15.0, RoundUp, 5000},
		{"不变", 100, 10.0, 10.0, RoundPrecision, 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalcRecommendDenominator(tt.oldDen, tt.oldExpH, tt.targetH, tt.mode)
			if got != tt.wantDen {
				t.Errorf("CalcRecommendDenominator(%d, %.1f, %.1f, %s) = %d, want %d",
					tt.oldDen, tt.oldExpH, tt.targetH, tt.mode, got, tt.wantDen)
			}
		})
	}
}

func TestCalcExpectHours(t *testing.T) {
	// kph=100, prob=1/100 => expectH = 1 / (100 * 0.01) = 1.0
	got := CalcExpectHours(100, 1, 100)
	if got < 0.99 || got > 1.01 {
		t.Errorf("CalcExpectHours(100, 1, 100) = %.2f, want ~1.0", got)
	}

	// kph=0 => 999999
	got = CalcExpectHours(0, 1, 100)
	if got != 999999 {
		t.Errorf("CalcExpectHours(0, 1, 100) = %.0f, want 999999", got)
	}
}

func TestCalcDeviation(t *testing.T) {
	// 目标10h，实际10h => 0%
	if d := CalcDeviation(10.0, 10.0); d != 0 {
		t.Errorf("CalcDeviation(10, 10) = %.1f, want 0", d)
	}
	// 目标10h，实际15h => +50%
	if d := CalcDeviation(15.0, 10.0); d < 49.9 || d > 50.1 {
		t.Errorf("CalcDeviation(15, 10) = %.1f, want 50", d)
	}
	// 目标10h，实际5h => -50%
	if d := CalcDeviation(5.0, 10.0); d < -50.1 || d > -49.9 {
		t.Errorf("CalcDeviation(5, 10) = %.1f, want -50", d)
	}
}

func TestRoundDenominator_Precision(t *testing.T) {
	tests := []struct {
		raw  float64
		want int
	}{
		{3782.0, 3782},
		{100.0, 100},
		{100.1, 101},
		{0.5, 1},
	}
	for _, tt := range tests {
		got := RoundDenominator(tt.raw, RoundPrecision)
		if got != tt.want {
			t.Errorf("RoundDenominator(%.1f, precision) = %d, want %d", tt.raw, got, tt.want)
		}
	}
}

func TestRoundDenominator_Nearest(t *testing.T) {
	tests := []struct {
		raw  float64
		want int
	}{
		{3782.0, 4000},
		{3200.0, 3000},
		{120.0, 100},
		{85.0, 100},
	}
	for _, tt := range tests {
		got := RoundDenominator(tt.raw, RoundNearest)
		if got != tt.want {
			t.Errorf("RoundDenominator(%.1f, nearest) = %d, want %d", tt.raw, got, tt.want)
		}
	}
}

func TestRoundDenominator_Up(t *testing.T) {
	tests := []struct {
		raw  float64
		want int
	}{
		{3782.0, 5000},
		{120.0, 200},
		{85.0, 100},
		{1050.0, 2000},
	}
	for _, tt := range tests {
		got := RoundDenominator(tt.raw, RoundUp)
		if got != tt.want {
			t.Errorf("RoundDenominator(%.1f, up) = %d, want %d", tt.raw, got, tt.want)
		}
	}
}