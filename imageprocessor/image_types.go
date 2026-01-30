package imageprocessor

import (
	"image"
	"image/color"
	"sync"
)

// FloatMatrix represents a 2D matrix of float64 values for DCT operations
type FloatMatrix struct {
	Data   [][]float64
	Width  int
	Height int
}

// Memory pools for commonly used matrix sizes
var (
	// Pool for 32x32 matrices (used in pHash DCT)
	floatMatrix32Pool = sync.Pool{
		New: func() interface{} {
			return newFloatMatrixDirect(32, 32)
		},
	}

	// Pool for 8x8 matrices (used for low-frequency extraction)
	floatMatrix8Pool = sync.Pool{
		New: func() interface{} {
			return newFloatMatrixDirect(8, 8)
		},
	}

	// Pool for hash byte slices (8 bytes = 64 bits)
	hashBytesPool = sync.Pool{
		New: func() interface{} {
			b := make([]byte, 0, 8)
			return &b
		},
	}

	// Pool for 64-element float64 slices (used for median calculation)
	float64SlicePool = sync.Pool{
		New: func() interface{} {
			s := make([]float64, 64)
			return &s
		},
	}
)

// newFloatMatrixDirect creates a FloatMatrix without using pools (for pool initialization)
func newFloatMatrixDirect(width, height int) *FloatMatrix {
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

// NewFloatMatrix creates a new FloatMatrix, using pools for common sizes
func NewFloatMatrix(width, height int) *FloatMatrix {
	// Use pooled matrices for common sizes
	if width == 32 && height == 32 {
		m := floatMatrix32Pool.Get().(*FloatMatrix)
		m.Reset()
		return m
	}
	if width == 8 && height == 8 {
		m := floatMatrix8Pool.Get().(*FloatMatrix)
		m.Reset()
		return m
	}
	// Fall back to direct allocation for other sizes
	return newFloatMatrixDirect(width, height)
}

// Reset clears all values in the matrix to zero
func (m *FloatMatrix) Reset() {
	for y := 0; y < m.Height; y++ {
		for x := 0; x < m.Width; x++ {
			m.Data[y][x] = 0
		}
	}
}

// Release returns the matrix to the appropriate pool
func (m *FloatMatrix) Release() {
	if m.Width == 32 && m.Height == 32 {
		floatMatrix32Pool.Put(m)
	} else if m.Width == 8 && m.Height == 8 {
		floatMatrix8Pool.Put(m)
	}
	// Other sizes are left for GC
}

// GetHashBytes gets a byte slice from the pool
func GetHashBytes() *[]byte {
	return hashBytesPool.Get().(*[]byte)
}

// ReleaseHashBytes returns a byte slice to the pool
func ReleaseHashBytes(b *[]byte) {
	*b = (*b)[:0] // Reset length but keep capacity
	hashBytesPool.Put(b)
}

// GetFloat64Slice gets a 64-element float64 slice from the pool
func GetFloat64Slice() *[]float64 {
	return float64SlicePool.Get().(*[]float64)
}

// ReleaseFloat64Slice returns a float64 slice to the pool
func ReleaseFloat64Slice(s *[]float64) {
	float64SlicePool.Put(s)
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
