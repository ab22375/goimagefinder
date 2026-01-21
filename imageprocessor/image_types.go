package imageprocessor

import (
	"image"
	"image/color"
)

// FloatMatrix represents a 2D matrix of float64 values for DCT operations
type FloatMatrix struct {
	Data   [][]float64
	Width  int
	Height int
}

// NewFloatMatrix creates a new FloatMatrix with the given dimensions
func NewFloatMatrix(width, height int) *FloatMatrix {
	data := make([][]float64, height)
	for i := range data {
		data[i] = make([]float64, width)
	}
	return &FloatMatrix{
		Data:   data,
		Width:  width,
		Height: height,
	}
}

// Get returns the value at position (x, y)
func (m *FloatMatrix) Get(x, y int) float64 {
	return m.Data[y][x]
}

// Set sets the value at position (x, y)
func (m *FloatMatrix) Set(x, y int, val float64) {
	m.Data[y][x] = val
}

// Region extracts a subregion of the matrix
func (m *FloatMatrix) Region(x, y, width, height int) *FloatMatrix {
	result := NewFloatMatrix(width, height)
	for j := 0; j < height; j++ {
		for i := 0; i < width; i++ {
			result.Data[j][i] = m.Data[y+j][x+i]
		}
	}
	return result
}

// grayImageToFloatMatrix converts a grayscale image to a FloatMatrix
func grayImageToFloatMatrix(img *image.Gray) *FloatMatrix {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	matrix := NewFloatMatrix(width, height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			matrix.Data[y][x] = float64(img.GrayAt(x+bounds.Min.X, y+bounds.Min.Y).Y)
		}
	}
	return matrix
}

// normalizeGrayImage normalizes the contrast of a grayscale image to 0-255 range
// This replaces gocv.Normalize
func normalizeGrayImage(img *image.Gray) *image.Gray {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Find min and max values
	var minVal, maxVal uint8 = 255, 0
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			val := img.GrayAt(x, y).Y
			if val < minVal {
				minVal = val
			}
			if val > maxVal {
				maxVal = val
			}
		}
	}

	// Create normalized image
	result := image.NewGray(image.Rect(0, 0, width, height))

	// Handle case where all pixels have the same value
	if minVal == maxVal {
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				result.SetGray(x, y, color.Gray{Y: 128})
			}
		}
		return result
	}

	// Normalize to 0-255 range
	rangeVal := float64(maxVal - minVal)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcVal := img.GrayAt(x+bounds.Min.X, y+bounds.Min.Y).Y
			normalized := uint8(float64(srcVal-minVal) / rangeVal * 255.0)
			result.SetGray(x, y, color.Gray{Y: normalized})
		}
	}

	return result
}

// toGrayscale converts any image to grayscale
func toGrayscale(img image.Image) *image.Gray {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray.Set(x, y, img.At(x, y))
		}
	}

	return gray
}
