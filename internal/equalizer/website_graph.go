package equalizer

import "math"

// Kuba uses these filters for Natural's graph. The preset catalogue holds the
// separate filters that are written to the cable.
var naturalDisplayBands = [bandCount]band{
	peak(25, -2.9, 0.5),
	peak(157, -5.2, 0.5),
	peak(1077, -1.5, 2.47),
	peak(1837, -4.3, 1.67),
	peak(2272, -7.3, 2.05),
	peak(4326, -8, 2.73),
	peak(4970, -7.5, 3),
	peak(7130, -7.3, 3),
	peak(424, 3.9, 0.54),
	peak(3334, 7, 0.78),
}

type CurvePoint struct {
	Frequency float64 `json:"frequency"`
	DB        float64 `json:"db"`
}

func websiteGraph(p preset) (curve, target []CurvePoint, targetName string) {
	// The website displays its default JM-1 target for every preset except Harman.
	targetDB := &auriJM1TargetDB
	targetName = "JM-1"
	if p.id == "harman" {
		targetDB = &auriHarmanTargetDB
		targetName = "Harman IE 2019"
	}
	bands := p.bands
	if p.id == "pref6" {
		bands = naturalDisplayBands
	}
	var coefficients [bandCount][5]float64
	for i, b := range bands {
		coefficients[i] = b.wireCoefficients()
	}

	curve = make([]CurvePoint, len(auriMeasurementFrequencies))
	target = make([]CurvePoint, len(auriMeasurementFrequencies))
	var offset float64
	var alignedCount int
	for i, frequency := range auriMeasurementFrequencies {
		db := auriMeasurementDB[i] + displayFilterGain(coefficients, frequency)
		curve[i] = CurvePoint{Frequency: frequency, DB: db}
		target[i] = CurvePoint{Frequency: frequency, DB: targetDB[i]}
		if frequency >= 300 && frequency <= 3000 {
			offset += db - targetDB[i]
			alignedCount++
		}
	}
	offset /= float64(alignedCount)
	for i := range curve {
		curve[i].DB -= offset
	}
	return curve, target, targetName
}

// The website plots unquantized filters without the preamp. The coefficient
// order is b0, b2, -a1, b1, -a2, as returned by wireCoefficients.
func displayFilterGain(coefficients [bandCount][5]float64, frequency float64) float64 {
	omega := 2 * math.Pi * frequency / sampleRate
	sin1, cos1 := math.Sincos(omega)
	sin2, cos2 := math.Sincos(2 * omega)
	var db float64
	for _, c := range coefficients {
		nr, ni := c[0]+c[3]*cos1+c[1]*cos2, c[3]*sin1+c[1]*sin2
		dr, di := 1-c[2]*cos1-c[4]*cos2, c[2]*sin1+c[4]*sin2
		db += 10 * math.Log10((nr*nr+ni*ni)/(dr*dr+di*di))
	}
	return db
}
