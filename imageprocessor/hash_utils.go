package imageprocessor

import (
	"fmt"
	"image"
	"math"
	"sort"

	"github.com/disintegration/imaging"
)

// ComputeAverageHash calculates a simple average hash for the image
// Always returns a hexadecimal string representation
func ComputeAverageHash(img image.Image) (string, error) {
	if img == nil {
		return "", fmt.Errorf("cannot compute hash for nil image")
	}

	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return "", fmt.Errorf("cannot compute hash for empty image")
	}

	// Resize to 8x8
	resized := imaging.Resize(img, 8, 8, imaging.Linear)

	// Convert to grayscale
	gray := imaging.Grayscale(resized)

	// Calculate mean pixel value
	var sum uint64
	var count int

	grayImg := toGrayscale(gray)
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			pixel := grayImg.GrayAt(x, y).Y
			sum += uint64(pixel)
			count++
		}
	}

	// Calculate average
	var threshold float64
	if count > 0 {
		threshold = float64(sum) / float64(count)
	}

	// Compute binary hash (as bits)
	var hashBytes []byte
	var currentByte byte = 0
	var bitCount uint = 0

	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			pixel := grayImg.GrayAt(x, y).Y

			// Set bit based on comparison with threshold
			currentByte = currentByte << 1
			if float64(pixel) >= threshold {
				currentByte |= 1
			}

			bitCount++

			// When we have 8 bits, add the byte to our slice
			if bitCount == 8 {
				hashBytes = append(hashBytes, currentByte)
				currentByte = 0
				bitCount = 0
			}
		}
	}

	// Handle any remaining bits
	if bitCount > 0 {
		// Pad with zeros on the right
		currentByte = currentByte << (8 - bitCount)
		hashBytes = append(hashBytes, currentByte)
	}

	// Convert bytes to hex string
	hexString := ""
	for _, b := range hashBytes {
		hexString += fmt.Sprintf("%02x", b)
	}

	return hexString, nil
}

// ComputePerceptualHash computes a DCT-based perceptual hash for the image
// Always returns a hexadecimal string representation
// Uses memory pools for better performance with high-throughput processing
func ComputePerceptualHash(img image.Image) (string, error) {
	if img == nil {
		return "", fmt.Errorf("cannot compute hash for nil image")
	}

	bounds := img.Bounds()
	if bounds.Dx() == 0 || bounds.Dy() == 0 {
		return "", fmt.Errorf("cannot compute hash for empty image")
	}

	// Resize to 32x32 for DCT
	resized := imaging.Resize(img, 32, 32, imaging.Linear)

	// Convert to grayscale
	gray := imaging.Grayscale(resized)
	grayImg := toGrayscale(gray)

	// Convert to float matrix for DCT (uses pool for 32x32)
	floatImg := grayImageToFloatMatrix(grayImg)
	defer floatImg.Release()

	// Apply DCT (result uses pool for 32x32)
	dct := applyDCT(floatImg)
	defer dct.Release()

	// Extract 8x8 low frequency components (uses pool for 8x8)
	lowFreq := dct.Region(0, 0, 8, 8)
	defer lowFreq.Release()

	// Get pooled slice for median calculation
	valuesPtr := GetFloat64Slice()
	values := *valuesPtr
	defer ReleaseFloat64Slice(valuesPtr)

	idx := 0
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			values[idx] = lowFreq.Get(x, y)
			idx++
		}
	}

	// Calculate median
	median := calculateMedian64(values)

	// Compute binary hash (as bits) using pooled byte slice
	hashBytesPtr := GetHashBytes()
	hashBytes := *hashBytesPtr
	defer ReleaseHashBytes(hashBytesPtr)

	var currentByte byte = 0
	var bitCount uint = 0

	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			val := lowFreq.Get(x, y)

			// Set bit based on comparison with median
			currentByte = currentByte << 1
			if val >= median {
				currentByte |= 1
			}

			bitCount++

			// When we have 8 bits, add the byte to our slice
			if bitCount == 8 {
				hashBytes = append(hashBytes, currentByte)
				currentByte = 0
				bitCount = 0
			}
		}
	}

	// Handle any remaining bits
	if bitCount > 0 {
		// Pad with zeros on the right
		currentByte = currentByte << (8 - bitCount)
		hashBytes = append(hashBytes, currentByte)
	}

	// Convert bytes to hex string
	hexString := ""
	for _, b := range hashBytes {
		hexString += fmt.Sprintf("%02x", b)
	}

	return hexString, nil
}

// applyDCT applies a Discrete Cosine Transform to a float matrix
func applyDCT(img *FloatMatrix) *FloatMatrix {
	rows, cols := img.Height, img.Width
	result := NewFloatMatrix(cols, rows)

	for u := 0; u < rows; u++ {
		for v := 0; v < cols; v++ {
			sum := 0.0
			for i := 0; i < rows; i++ {
				for j := 0; j < cols; j++ {
					// DCT-II formula
					cosU := math.Cos(math.Pi * float64(u) * (2*float64(i) + 1) / (2 * float64(rows)))
					cosV := math.Cos(math.Pi * float64(v) * (2*float64(j) + 1) / (2 * float64(cols)))
					sum += img.Get(j, i) * cosU * cosV
				}
			}

			// Apply scaling factors
			scaleU := 1.0
			if u == 0 {
				scaleU = 1.0 / math.Sqrt(2.0)
			}

			scaleV := 1.0
			if v == 0 {
				scaleV = 1.0 / math.Sqrt(2.0)
			}

			scaleFactor := (2.0 * scaleU * scaleV) / math.Sqrt(float64(rows*cols))
			result.Set(v, u, sum*scaleFactor)
		}
	}

	return result
}

// calculateMedian64 calculates the median value of a float64 array
func calculateMedian64(values []float64) float64 {
	// Make a copy to avoid modifying the original slice
	valuesCopy := make([]float64, len(values))
	copy(valuesCopy, values)

	// Sort the copy
	sort.Float64s(valuesCopy)

	// Find median
	length := len(valuesCopy)
	if length == 0 {
		return 0
	} else if length%2 == 0 {
		return (valuesCopy[length/2-1] + valuesCopy[length/2]) / 2
	} else {
		return valuesCopy[length/2]
	}
}
