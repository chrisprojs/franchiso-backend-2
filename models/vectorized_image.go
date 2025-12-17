package models

type VectorizedImage struct {
	FilePath string    `json:"file_path"`
	Vector   []float64 `json:"vector"`
}
