package utils

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/chrisprojs/Franchiso/models"
)

const VECTORIZE_IMAGE_URL = "http://localhost:5000/vectorize"

type VectorRequest struct {
	FilePath string `json:"file_path"`
}

type VectorResponse struct {
	Vector []float64 `json:"vector"`
}

// convertToVectorizedImage converts a file path string to VectorizedImage structure
// Vector is initialized as empty array with 512 dimensions (will be populated by vector service later)
func ConvertToVectorizedImage(filePath string) (models.VectorizedImage, error) {
	// 1. Handle empty paths immediately
	if filePath == "" {
		return models.VectorizedImage{
			FilePath: "",
			Vector:   make([]float64, 512), // Return empty zero-vector
		}, errors.New("file path tidak ditemukan")
	}

	// 2. Prepare payload for Python service
	payload := VectorRequest{FilePath: filePath}
	jsonData, err := json.Marshal(payload)
	if err != nil {
		fmt.Printf("Error marshalling payload: %v\n", err)
		return models.VectorizedImage{
			FilePath: filePath,
			Vector:   make([]float64, 512),
		}, errors.New("gagal vectorize image")
	}

	// 3. Make the HTTP Request
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Post(VECTORIZE_IMAGE_URL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Printf("Error calling vectorizer service: %v\n", err)
		return models.VectorizedImage{
			FilePath: filePath,
			Vector:   make([]float64, 512),
		}, errors.New("gagal vectorize image")
	}
	defer resp.Body.Close()

	// 4. Decode the response
	if resp.StatusCode != http.StatusOK {
		fmt.Printf("Vectorizer service returned status: %d\n", resp.StatusCode)
		return models.VectorizedImage{
			FilePath: filePath,
			Vector:   make([]float64, 512),
		}, errors.New("gagal vectorize image")
	}

	var vecResp VectorResponse
	if err := json.NewDecoder(resp.Body).Decode(&vecResp); err != nil {
		fmt.Printf("Error decoding vector response: %v\n", err)
		return models.VectorizedImage{
			FilePath: filePath,
			Vector:   make([]float64, 512),
		}, errors.New("gagal vectorize image")
	}

	// 5. Return the populated struct
	return models.VectorizedImage{
		FilePath: filePath,
		Vector:   vecResp.Vector,
	}, nil
}

// convertToVectorizedImages converts a slice of file path strings to slice of VectorizedImage structures
func ConvertToVectorizedImages(filePaths []string) ([]models.VectorizedImage, error) {
	if len(filePaths) == 0 {
		return []models.VectorizedImage{}, nil
	}
	var err error
	result := make([]models.VectorizedImage, len(filePaths))
	for i, path := range filePaths {
		result[i], err = ConvertToVectorizedImage(path)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}
