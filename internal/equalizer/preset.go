package equalizer

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"
	"time"
)

const (
	vendorID        = 0x3302
	productID       = 0x8902
	reportID        = 0xcb
	chunkSize       = 30
	dataReportCount = 9
	nativeReportLen = 2 + chunkSize // Report ID, chunk index, data.
	sampleRate      = 96000
	bandCount       = 10
)

type band struct {
	frequency, gainDB, q float64
	lowShelf             bool
}

type preset struct {
	id, name, group, description string
	preampDB                     float64
	bands                        [bandCount]band
}

func findPreset(id string) (preset, bool) {
	if strings.EqualFold(id, "natural") {
		id = "pref6"
	}
	for _, p := range presets {
		if strings.EqualFold(id, p.id) {
			return p, true
		}
	}
	return preset{}, false
}

// wireCoefficients returns the normalized biquad in the DSP's coefficient
// order, with the denominator coefficients negated for the cable's recurrence.
func (b band) wireCoefficients() [5]float64 {
	amplitude := math.Pow(10, b.gainDB/40)
	omega := 2 * math.Pi * b.frequency / sampleRate
	alpha := math.Sin(omega) / (2 * b.q)
	cosine := math.Cos(omega)
	if b.lowShelf {
		slope := 2 * math.Sqrt(amplitude) * alpha
		plus, minus := amplitude+1, amplitude-1
		denominator := plus + minus*cosine + slope
		b0 := amplitude * (plus - minus*cosine + slope) / denominator
		b1 := 2 * amplitude * (minus - plus*cosine) / denominator
		b2 := amplitude * (plus - minus*cosine - slope) / denominator
		a1 := -2 * (minus + plus*cosine) / denominator
		a2 := (plus + minus*cosine - slope) / denominator
		return [5]float64{b0, b2, -a1, b1, -a2}
	}
	denominator := 1 + alpha/amplitude
	b0 := (1 + alpha*amplitude) / denominator
	b1 := -2 * cosine / denominator
	b2 := (1 - alpha*amplitude) / denominator
	a2 := (1 - alpha/amplitude) / denominator
	return [5]float64{b0, b2, -b1, b1, -a2}
}

func (b band) programmedCoefficients() [5]int32 {
	var coefficients [5]int32
	for i, value := range b.wireCoefficients() {
		coefficients[i] = fixedPoint(value, 27)
	}
	return coefficients
}

func (p preset) programmedPreamp() int32 {
	return fixedPoint(math.Pow(10, p.preampDB/20), 23)
}

func (p preset) blob() []byte {
	data := make([]byte, 0, 220)
	data = append(data, 'E', 'Q', '1', '#', 0xd6, 0, 0, bandCount, 4, 5, 1, bandCount, 0, 0)
	data = binary.LittleEndian.AppendUint32(data, uint32(p.programmedPreamp()))
	for _, band := range p.bands {
		for _, coefficient := range band.programmedCoefficients() {
			data = binary.LittleEndian.AppendUint32(data, uint32(coefficient))
		}
	}
	return binary.LittleEndian.AppendUint16(data, crc16(data))
}

func fixedPoint(value float64, fractionalBits int) int32 {
	// Match JavaScript Math.round's ties toward positive infinity.
	return int32(math.Floor(math.Ldexp(value, fractionalBits) + 0.5))
}

func crc16(data []byte) uint16 {
	crc := uint16(0xffff)
	for _, b := range data {
		crc ^= uint16(b) << 8
		for range 8 {
			high := crc&0x8000 != 0
			crc <<= 1
			if high {
				crc ^= 0x1021
			}
		}
	}
	return crc
}

func (p preset) dataReports() [dataReportCount][nativeReportLen]byte {
	var padded [dataReportCount * chunkSize]byte
	copy(padded[:], p.blob())
	var reports [dataReportCount][nativeReportLen]byte
	for index := range reports {
		reports[index][0] = reportID
		reports[index][1] = byte(index)
		copy(reports[index][2:], padded[index*chunkSize:(index+1)*chunkSize])
	}
	return reports
}

func applyPreset(ctx context.Context, device io.Writer, p preset) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for index, report := range p.dataReports() {
		if err := writeReport(device, report[:]); err != nil {
			return fmt.Errorf("send EQ data report %d: %w", index, err)
		}
		if err := wait(ctx, 16*time.Millisecond); err != nil {
			return err
		}
	}
	if err := wait(ctx, 150*time.Millisecond); err != nil {
		return err
	}
	if err := writeReport(device, []byte{reportID, 0x01, 0x65, 0, 0x01, 0xff}); err != nil {
		return fmt.Errorf("send EQ apply report: %w", err)
	}
	return nil
}

func writeReport(device io.Writer, report []byte) error {
	n, err := device.Write(report)
	if err == nil && n != len(report) {
		return io.ErrShortWrite
	}
	return err
}

func wait(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-timer.C:
		return ctx.Err()
	case <-ctx.Done():
		return ctx.Err()
	}
}
