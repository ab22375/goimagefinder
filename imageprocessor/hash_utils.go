package imageprocessor

import (
	"fmt"
	"image"
	"math"
	"sort"

	"gocv.io/x/gocv"
)

// ComputeAverageHash calculates a simple average hash for the image
// Always returns a hexadecimal string representation
func ComputeAverageHash(img gocv.Mat) (string, error) {
	if img.Empty() {
		return "", fmt.Errorf("cannot compute hash for empty image")
	}

	// Resize to 8x8
	resized := gocv.NewMat()
	defer resized.Close()

	gocv.Resize(img, &resized, image.Point{X: 8, Y: 8}, 0, 0, gocv.InterpolationLinear)

	// Convert to grayscale if not already
	gray := gocv.NewMat()
	defer gray.Close()

	if img.Channels() != 1 {
		gocv.CvtColor(resized, &gray, gocv.ColorBGRToGray)
	} else {
		resized.CopyTo(&gray)
	}

	// Calculate mean pixel value manually
	var sum uint64
	var count int

	for y := 0; y < gray.Rows(); y++ {
		for x := 0; x < gray.Cols(); x++ {
			pixel := gray.GetUCharAt(y, x)
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

	for y := 0; y < gray.Rows(); y++ {
		for x := 0; x < gray.Cols(); x++ {
			pixel := gray.GetUCharAt(y, x)

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
func ComputePerceptualHash(img gocv.Mat) (string, error) {
	if img.Empty() {
		return "", fmt.Errorf("cannot compute hash for empty image")
	}

	// Resize to 32x32 for DCT
	resized := gocv.NewMat()
	defer resized.Close()

	gocv.Resize(img, &resized, image.Point{X: 32, Y: 32}, 0, 0, gocv.InterpolationLinear)

	// Convert to grayscale if not already
	gray := gocv.NewMat()
	defer gray.Close()

	if img.Channels() != 1 {
		gocv.CvtColor(resized, &gray, gocv.ColorBGRToGray)
	} else {
		resized.CopyTo(&gray)
	}

	// Convert to float for DCT
	floatImg := gocv.NewMat()
	defer floatImg.Close()
	gray.ConvertTo(&floatImg, gocv.MatTypeCV32F)

	// Apply DCT
	dct := gocv.NewMat()
	defer dct.Close()

	gocv.DCT(floatImg, &dct, 0)
	if dct.Empty() {
		// Fall back to custom DCT implementation
		dct = applyDCT(floatImg)
	}

	// Extract 8x8 low frequency components
	lowFreq := dct.Region(image.Rect(0, 0, 8, 8))
	defer lowFreq.Close()

	// Calculate median value
	values := make([]float32, 64)
	idx := 0
	for y := 0; y < lowFreq.Rows(); y++ {
		for x := 0; x < lowFreq.Cols(); x++ {
			values[idx] = lowFreq.GetFloatAt(y, x)
			idx++
		}
	}

	// Calculate median
	median := calculateMedian(values)

	// Compute binary hash (as bits)
	var hashBytes []byte
	var currentByte byte = 0
	var bitCount uint = 0

	for y := 0; y < lowFreq.Rows(); y++ {
		for x := 0; x < lowFreq.Cols(); x++ {
			val := lowFreq.GetFloatAt(y, x)

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

// applyDCT applies a Discrete Cosine Transform to an image
// Simplified implementation when OpenCV's DCT is not available
func applyDCT(img gocv.Mat) gocv.Mat {
	rows, cols := img.Rows(), img.Cols()
	result := gocv.NewMatWithSize(rows, cols, gocv.MatTypeCV32F)

	for u := 0; u < rows; u++ {
		for v := 0; v < cols; v++ {
			sum := float32(0.0)
			for i := 0; i < rows; i++ {
				for j := 0; j < cols; j++ {
					// DCT-II formula
					cosU := float32(math.Cos(float64(math.Pi*float64(u)*(2*float64(i)+1)) / (2 * float64(rows))))
					cosV := float32(math.Cos(float64(math.Pi*float64(v)*(2*float64(j)+1)) / (2 * float64(cols))))
					sum += img.GetFloatAt(i, j) * cosU * cosV
				}
			}

			// Apply scaling factors
			scaleU := float32(1.0)
			if u == 0 {
				scaleU = 1.0 / float32(math.Sqrt(2.0))
			}

			scaleV := float32(1.0)
			if v == 0 {
				scaleV = 1.0 / float32(math.Sqrt(2.0))
			}

			scaleFactor := (2.0 * scaleU * scaleV) / float32(math.Sqrt(float64(rows*cols)))
			result.SetFloatAt(u, v, sum*scaleFactor)
		}
	}

	return result
}

// calculateMedian calculates the median value of a float32 array
func calculateMedian(values []float32) float32 {
	// Make a copy to avoid modifying the original slice
	valuesCopy := make([]float32, len(values))
	copy(valuesCopy, values)

	// Sort the copy
	sort.Slice(valuesCopy, func(i, j int) bool {
		return valuesCopy[i] < valuesCopy[j]
	})

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
