package face

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

// ─────────────────────────────────────────
// Interface — abstraksi face recognition engine
//
// Semua implementasi (AWS, InsightFace, dll) harus implement interface ini.
// Service hanya tahu interface ini — swap engine tidak perlu ubah service.
// ─────────────────────────────────────────

type FaceEngine interface {
	// Compare — bandingkan foto dari FE dengan foto referensi
	//
	// inputImage    : bytes JPEG/PNG yang dikirim FE saat checkin
	// referencePath : path foto referensi di disk (dari tabel employee_faces)
	//
	// Return:
	//   score float64 — kemiripan 0.0 (beda orang) sampai 1.0 (identik)
	//   error         — engine error (network, timeout, dll)
	Compare(inputImage []byte, referencePath string) (float64, error)

	// DetectFace — hitung jumlah wajah dalam gambar
	// Dipakai saat karyawan register foto referensi untuk validasi:
	//   0 wajah → NO_FACE_DETECTED
	//   >1 wajah → MULTIPLE_FACES
	//   1 wajah → lolos
	DetectFace(image []byte) (int, error)
}

// ─────────────────────────────────────────
// IMPLEMENTASI 1: AWS Rekognition
//
// Rekomendasi untuk MVP — setup cepat, tidak perlu manage model.
// Biaya: ~$0.001 per CompareFaces request.
//
// Cara pakai di main.go:
//   engine := face.NewAWSEngine("ap-southeast-1")
//   faceService := &face.Service{Repo: faceRepo, Engine: engine}
// ─────────────────────────────────────────

// AWSEngine — implementasi FaceEngine menggunakan AWS Rekognition
// Menggunakan HTTP langsung ke AWS API supaya tidak perlu SDK besar.
// Jika mau pakai SDK: go get github.com/aws/aws-sdk-go
type AWSEngine struct {
	Region          string
	AccessKeyID     string
	SecretAccessKey string
	httpClient      *http.Client
}

// NewAWSEngine — buat instance AWSEngine
// Ambil credentials dari environment variable supaya tidak hardcode
func NewAWSEngine(region string) *AWSEngine {
	return &AWSEngine{
		Region:          region,
		AccessKeyID:     os.Getenv("AWS_ACCESS_KEY_ID"),
		SecretAccessKey: os.Getenv("AWS_SECRET_ACCESS_KEY"),
		httpClient:      &http.Client{},
	}
}

// Compare — implementasi dengan AWS Rekognition CompareFaces
// Jika pakai SDK aws-sdk-go, ganti body ini dengan SDK call
func (e *AWSEngine) Compare(inputImage []byte, referencePath string) (float64, error) {
	// Load foto referensi dari disk
	refBytes, err := os.ReadFile(referencePath)
	if err != nil {
		return 0, fmt.Errorf("gagal baca foto referensi: %w", err)
	}

	// Kirim ke AWS Rekognition via SDK atau HTTP
	// Contoh ini menggunakan struktur request AWS SDK v1
	// Untuk implementasi penuh, pakai: github.com/aws/aws-sdk-go/service/rekognition
	//
	// result, err := rekognitionClient.CompareFaces(&rekognition.CompareFacesInput{
	//     SourceImage: &rekognition.Image{Bytes: refBytes},
	//     TargetImage: &rekognition.Image{Bytes: inputImage},
	//     SimilarityThreshold: aws.Float64(0), // ambil semua match
	// })
	//
	// if len(result.FaceMatches) == 0 {
	//     return 0.0, nil
	// }
	// score := float64(*result.FaceMatches[0].Similarity) / 100.0
	// return score, nil

	// Placeholder supaya file bisa compile — replace dengan SDK call di atas
	_ = refBytes
	_ = inputImage
	return 0.0, fmt.Errorf("AWS engine belum dikonfigurasi — implementasi dengan SDK")
}

// DetectFace — deteksi wajah menggunakan AWS Rekognition DetectFaces
func (e *AWSEngine) DetectFace(image []byte) (int, error) {
	// Implementasi dengan SDK:
	// result, err := rekognitionClient.DetectFaces(&rekognition.DetectFacesInput{
	//     Image: &rekognition.Image{Bytes: image},
	// })
	// return len(result.FaceDetails), nil

	_ = image
	return 0, fmt.Errorf("AWS engine belum dikonfigurasi")
}

// ─────────────────────────────────────────
// IMPLEMENTASI 2: InsightFace self-hosted (Python microservice)
//
// Untuk production — gratis per-request, data tidak keluar server.
// Perlu deploy Python service terpisah dengan Docker.
//
// Cara deploy Python service:
//   docker pull insightface/model-zoo
//   atau buat sendiri dengan FastAPI + insightface library
//
// Cara pakai di main.go:
//   engine := face.NewInsightFaceEngine("http://localhost:8001")
//   faceService := &face.Service{Repo: faceRepo, Engine: engine}
// ─────────────────────────────────────────

// InsightFaceEngine — implementasi FaceEngine via HTTP ke Python microservice
type InsightFaceEngine struct {
	ServiceURL string // contoh: "http://localhost:8001"
	httpClient *http.Client
}

// NewInsightFaceEngine — buat instance InsightFaceEngine
func NewInsightFaceEngine(serviceURL string) *InsightFaceEngine {
	return &InsightFaceEngine{
		ServiceURL: serviceURL,
		httpClient: &http.Client{},
	}
}

// insightCompareResponse — response JSON dari Python service
type insightCompareResponse struct {
	Similarity float64 `json:"similarity"`
	FaceCount  int     `json:"face_count"`
	Error      string  `json:"error,omitempty"`
}

// Compare — kirim dua gambar ke Python InsightFace service
// Python service jalankan model ArcFace dan return similarity score
func (e *InsightFaceEngine) Compare(inputImage []byte, referencePath string) (float64, error) {
	refBytes, err := os.ReadFile(referencePath)
	if err != nil {
		return 0, fmt.Errorf("gagal baca foto referensi: %w", err)
	}

	// Build multipart form dengan dua field: input_image dan reference_image
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	inputPart, err := writer.CreateFormFile("input_image", "input.jpg")
	if err != nil {
		return 0, err
	}
	inputPart.Write(inputImage)

	refPart, err := writer.CreateFormFile("reference_image", "reference.jpg")
	if err != nil {
		return 0, err
	}
	refPart.Write(refBytes)

	writer.Close()

	req, err := http.NewRequest("POST", e.ServiceURL+"/compare", body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("gagal hubungi face engine: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var result insightCompareResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return 0, fmt.Errorf("gagal parse response engine: %w", err)
	}

	if result.Error != "" {
		return 0, fmt.Errorf("face engine error: %s", result.Error)
	}

	return result.Similarity, nil
}

// insightDetectResponse — response JSON dari Python service untuk deteksi
type insightDetectResponse struct {
	FaceCount int    `json:"face_count"`
	Error     string `json:"error,omitempty"`
}

// DetectFace — kirim gambar ke Python service untuk hitung jumlah wajah
func (e *InsightFaceEngine) DetectFace(image []byte) (int, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("image", "face.jpg")
	if err != nil {
		return 0, err
	}
	part.Write(image)
	writer.Close()

	req, err := http.NewRequest("POST", e.ServiceURL+"/detect", body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("gagal hubungi face engine: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var result insightDetectResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return 0, fmt.Errorf("gagal parse response engine: %w", err)
	}

	if result.Error != "" {
		return 0, fmt.Errorf("face engine error: %s", result.Error)
	}

	return result.FaceCount, nil
}

// ─────────────────────────────────────────
// IMPLEMENTASI 3: MockEngine — untuk testing
//
// Selalu return 1 wajah terdeteksi dan score 0.95 (di atas threshold).
// JANGAN dipakai di production.
//
// Cara pakai di main.go:
//   engine := &face.MockEngine{}
//   faceService := &face.Service{Repo: faceRepo, Engine: engine}
// ─────────────────────────────────────────

type MockEngine struct{}

func (m *MockEngine) Compare(inputImage []byte, referencePath string) (float64, error) {
	return 0.95, nil // selalu match, di atas threshold 0.80
}

func (m *MockEngine) DetectFace(image []byte) (int, error) {
	return 1, nil // selalu terdeteksi tepat 1 wajah
}
