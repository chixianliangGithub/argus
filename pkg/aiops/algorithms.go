package aiops

import (
	"math"
	"sort"
)

// CalculateMean calculates the arithmetic mean of a slice of float64.
func CalculateMean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range data {
		sum += v
	}
	return sum / float64(len(data))
}

// CalculateStdDev calculates the standard deviation of a slice of float64.
func CalculateStdDev(data []float64) float64 {
	if len(data) < 2 {
		return 0
	}
	mean := CalculateMean(data)
	variance := 0.0
	for _, v := range data {
		variance += math.Pow(v-mean, 2)
	}
	return math.Sqrt(variance / float64(len(data)-1))
}

// IsAnomaly3Sigma checks if a value is an anomaly using the 3-Sigma rule.
// Returns true if the value is outside mean ± n*stdDev.
// n is usually 3.
func IsAnomaly3Sigma(value float64, history []float64, n float64) (bool, float64, float64) {
	if len(history) < 2 {
		return false, 0, 0
	}
	mean := CalculateMean(history)
	stdDev := CalculateStdDev(history)
	lower := mean - n*stdDev
	upper := mean + n*stdDev

	return value < lower || value > upper, lower, upper
}

// CalculateMedian calculates the median of a slice of float64.
// Note: This sorts the input slice.
func CalculateMedian(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	// Make a copy to avoid modifying the original slice
	sorted := make([]float64, len(data))
	copy(sorted, data)
	sort.Float64s(sorted)

	mid := len(sorted) / 2
	if len(sorted)%2 == 0 {
		return (sorted[mid-1] + sorted[mid]) / 2.0
	}
	return sorted[mid]
}

// CalculateMAD calculates the Median Absolute Deviation.
func CalculateMAD(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}
	median := CalculateMedian(data)
	deviations := make([]float64, len(data))
	for i, v := range data {
		deviations[i] = math.Abs(v - median)
	}
	return CalculateMedian(deviations)
}

// IsAnomalyMAD checks if a value is an anomaly using MAD.
// Returns true if the value is outside median ± k*MAD.
// k is usually 3.
func IsAnomalyMAD(value float64, history []float64, k float64) (bool, float64, float64) {
	if len(history) == 0 {
		return false, 0, 0
	}
	median := CalculateMedian(history)
	mad := CalculateMAD(history)

	// Often MAD is scaled to be consistent with standard deviation for normal distribution
	// sigma_hat = 1.4826 * MAD
	// threshold = median +/- n * sigma_hat
	// But raw MAD usage: median +/- k * MAD

	lower := median - k*mad
	upper := median + k*mad

	return value < lower || value > upper, lower, upper
}

func HoltWintersAdditiveForecast(data []float64, seasonLength int, alpha, beta, gamma float64) (float64, float64, bool) {
	if seasonLength <= 1 {
		return 0, 0, false
	}
	if len(data) < seasonLength*2 {
		return 0, 0, false
	}
	if alpha <= 0 || alpha >= 1 || beta < 0 || beta >= 1 || gamma <= 0 || gamma >= 1 {
		return 0, 0, false
	}

	firstSeason := data[:seasonLength]
	secondSeason := data[seasonLength : 2*seasonLength]
	avg1 := CalculateMean(firstSeason)
	avg2 := CalculateMean(secondSeason)
	level := avg1
	trend := (avg2 - avg1) / float64(seasonLength)

	seasonals := make([]float64, seasonLength)
	for i := 0; i < seasonLength; i++ {
		seasonals[i] = firstSeason[i] - avg1
	}

	residuals := make([]float64, 0, len(data))
	for t := 0; t < len(data); t++ {
		si := t % seasonLength
		forecast := level + trend + seasonals[si]
		residual := data[t] - forecast
		if t >= seasonLength {
			residuals = append(residuals, residual)
		}

		prevLevel := level
		level = alpha*(data[t]-seasonals[si]) + (1-alpha)*(level+trend)
		trend = beta*(level-prevLevel) + (1-beta)*trend
		seasonals[si] = gamma*(data[t]-level) + (1-gamma)*seasonals[si]
	}

	pred := level + trend + seasonals[len(data)%seasonLength]
	if len(residuals) < 2 {
		return pred, 0, true
	}
	sigma := CalculateStdDev(residuals)
	return pred, sigma, true
}
