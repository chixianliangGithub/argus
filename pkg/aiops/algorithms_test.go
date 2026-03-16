package aiops

import (
	"math"
	"testing"
)

func TestCalculateMean(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5}
	mean := CalculateMean(data)
	if mean != 3.0 {
		t.Errorf("Expected mean 3.0, got %f", mean)
	}
}

func TestCalculateStdDev(t *testing.T) {
	data := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	stdDev := CalculateStdDev(data)
	// Mean = 5
	// Variance = (9+1+1+1+0+0+4+16)/7 = 32/7 = 4.5714
	// StdDev = sqrt(4.5714) = 2.138
	expected := math.Sqrt(32.0 / 7.0)
	if math.Abs(stdDev-expected) > 0.0001 {
		t.Errorf("Expected stdDev %f, got %f", expected, stdDev)
	}
}

func TestIsAnomaly3Sigma(t *testing.T) {
	history := []float64{10, 10, 10, 10, 10} // Mean=10, StdDev=0
	// With 0 stddev, anything != mean is anomaly if we consider strictness, but let's see implementation.
	// If stdDev is 0, any deviation is infinite z-score.
	// But our implementation uses value < mean - 3*std or value > mean + 3*std.
	// 10 +/- 0 -> range [10, 10].

	isAnomaly, _, _ := IsAnomaly3Sigma(10.1, history, 3)
	if !isAnomaly {
		t.Error("Expected 10.1 to be anomaly with 0 variance history")
	}

	history = []float64{0, 10, 20} // Mean=10, StdDev=10
	// Range: 10 +/- 30 => [-20, 40]
	isAnomaly, _, _ = IsAnomaly3Sigma(30, history, 3)
	if isAnomaly {
		t.Error("Expected 30 to NOT be anomaly")
	}
	isAnomaly, _, _ = IsAnomaly3Sigma(41, history, 3)
	if !isAnomaly {
		t.Error("Expected 41 to be anomaly")
	}
}

func TestCalculateMedian(t *testing.T) {
	data := []float64{1, 5, 3}
	median := CalculateMedian(data)
	if median != 3 {
		t.Errorf("Expected median 3, got %f", median)
	}

	data = []float64{1, 2, 3, 4}
	median = CalculateMedian(data)
	if median != 2.5 {
		t.Errorf("Expected median 2.5, got %f", median)
	}
}

func TestCalculateMAD(t *testing.T) {
	data := []float64{1, 1, 2, 2, 4, 6, 9}
	// Sorted: 1, 1, 2, 2, 4, 6, 9
	// Median = 2
	// Deviations: |1-2|=1, |1-2|=1, |2-2|=0, |2-2|=0, |4-2|=2, |6-2|=4, |9-2|=7
	// Sorted Deviations: 0, 0, 1, 1, 2, 4, 7
	// Median of Deviations (MAD) = 1
	mad := CalculateMAD(data)
	if mad != 1.0 {
		t.Errorf("Expected MAD 1.0, got %f", mad)
	}
}

func TestIsAnomalyMAD(t *testing.T) {
	history := []float64{1, 1, 2, 2, 4, 6, 9} // Median=2, MAD=1
	// Thresholds = 2 +/- 3*1 = [-1, 5]

	isAnomaly, _, _ := IsAnomalyMAD(4, history, 3)
	if isAnomaly {
		t.Error("Expected 4 to NOT be anomaly")
	}

	isAnomaly, _, _ = IsAnomalyMAD(6, history, 3)
	if !isAnomaly {
		t.Error("Expected 6 to be anomaly (upper 5)")
	}
}

func TestHoltWintersAdditiveForecast(t *testing.T) {
	season := 10
	data := make([]float64, 0, season*4)
	for i := 0; i < season*4; i++ {
		base := 100.0 + float64(i)*0.1
		seasonal := float64(i%season) * 0.5
		data = append(data, base+seasonal)
	}
	pred, sigma, ok := HoltWintersAdditiveForecast(data, season, 0.2, 0.01, 0.2)
	if !ok {
		t.Fatal("Expected ok=true")
	}
	if math.IsNaN(pred) || math.IsInf(pred, 0) {
		t.Fatalf("Invalid pred: %v", pred)
	}
	if sigma < 0 || math.IsNaN(sigma) || math.IsInf(sigma, 0) {
		t.Fatalf("Invalid sigma: %v", sigma)
	}
	if math.Abs(pred-data[len(data)-1]) > 50 {
		t.Fatalf("Pred seems off: pred=%v last=%v", pred, data[len(data)-1])
	}
}
