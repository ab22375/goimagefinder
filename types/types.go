package types

// ImageInfo holds the image metadata and features
type ImageInfo struct {
	ID             int64  `json:"id"`
	Path           string `json:"path"`
	SourcePrefix   string `json:"source_prefix"`
	Format         string `json:"format"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	CreatedAt      string `json:"created_at"`
	ModifiedAt     string `json:"modified_at"`
	Size           int64  `json:"size"`
	AverageHash    string `json:"average_hash"`
	PerceptualHash string `json:"perceptual_hash"`
	IsRawFormat    bool   `json:"is_raw_format"`
}

// ImageMatch holds the similarity scores
type ImageMatch struct {
	Path         string
	SourcePrefix string
	SSIMScore    float64
}
