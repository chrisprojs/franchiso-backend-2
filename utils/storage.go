package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"path/filepath"

	"github.com/disintegration/imaging"
)

// Helper function to upload file to storage proxy
func UploadToStorageProxy(fileHeader *multipart.FileHeader) (string, error) {
	file, err := fileHeader.Open()
	if err != nil {
		return "", err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", fileHeader.Filename)
	if err != nil {
		return "", err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return "", err
	}
	writer.Close()

	resp, err := http.Post("http://localhost:8081/upload", writer.FormDataContentType(), body)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status code: %d", resp.StatusCode)
	}

	var result struct {
		FileUrl string `json:"fileUrl"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", err
	}
	return result.FileUrl, nil
}

// DeleteFromStorageProxy deletes a file from storage proxy by file URL or filename
func DeleteFromStorageProxy(fileURL string) error {
	// fileURL example: "/file/abc123.jpg" or "http://localhost:8081/file/abc123.jpg"

	// Normalize filename
	parts := strings.Split(fileURL, "/")
	filename := parts[len(parts)-1]

	if filename == "" {
		return fmt.Errorf("invalid filename")
	}

	req, err := http.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("http://localhost:8081/file/%s", filename),
		nil,
	)
	if err != nil {
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete file (%d): %s", resp.StatusCode, string(body))
	}

	return nil
}

func ImageProcessing(
	fileHeader *multipart.FileHeader,
) (*bytes.Buffer, string, error) {

	file, err := fileHeader.Open()
	if err != nil {
		return nil, "", err
	}
	defer file.Close()

	// Decode ANY supported image type
	img, _, err := image.Decode(file)
	if err != nil {
		return nil, "", fmt.Errorf("failed to decode image: %w", err)
	}

	// Crop to square (center)
	bounds := img.Bounds()
	size := bounds.Dx()
	if bounds.Dy() < size {
		size = bounds.Dy()
	}

	cropped := imaging.CropCenter(img, size, size)

	// Encode as TinyJPG-style JPEG
	buf := new(bytes.Buffer)
	err = jpeg.Encode(buf, cropped, &jpeg.Options{
		Quality: 75, // TinyJPG-like compression (70–80 sweet spot)
	})
	if err != nil {
		return nil, "", fmt.Errorf("failed to encode jpeg: %w", err)
	}

	// Always return jpeg
	return buf, "jpeg", nil
}

// BufferToFileHeader mengubah buffer hasil crop menjadi *multipart.FileHeader agar bisa diupload
func BufferToFileHeader(buf *bytes.Buffer, filename, format string) *multipart.FileHeader {
	// Pastikan ekstensi filename sesuai format output (selalu jpg/jpeg)
	ext := strings.ToLower(filepath.Ext(filename))
	if format == "jpeg" || format == "jpg" {
		if ext != ".jpg" && ext != ".jpeg" {
			filename = strings.TrimSuffix(filename, ext) + ".jpeg"
		}
	}

	// Buat multipart writer ke buffer baru
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", filename)
	part.Write(buf.Bytes())
	writer.Close()

	// Buat multipart.Reader dari body
	reader := multipart.NewReader(body, writer.Boundary())
	form, _ := reader.ReadForm(int64(body.Len()))
	files := form.File["file"]
	if len(files) > 0 {
		return files[0]
	}
	return nil
}
