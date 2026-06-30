package face

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ─────────────────────────────────────────
// Interface FaceEngine
//
// Abstraksi engine face recognition.
// Service hanya tahu interface ini — bisa swap engine tanpa ubah service.
// ─────────────────────────────────────────

type FaceEngine interface {
	// DetectAndEmbed — deteksi wajah dalam gambar, return embedding vektor
	//
	// imageBase64 : foto yang dikirim FE, sudah di-encode ke base64 string
	//
	// Return:
	//   embedding []float64 — vektor 512 dimensi representasi wajah
	//   faceCount int       — jumlah wajah terdeteksi (0, 1, atau >1)
	//   error               — network error, timeout, dll
	//
	// Jika faceCount != 1, embedding akan nil.
	// Caller (service) yang bertanggung jawab cek faceCount sebelum pakai embedding.
	DetectAndEmbed(imageBase64 string) (embedding []float64, faceCount int, err error)

	// ComparEmbeddings — hitung cosine similarity antara dua embedding
	//
	// embeddingNew    : embedding dari foto yang baru dikirim FE
	// embeddingStored : embedding yang tersimpan di DB (dari registrasi)
	//
	// Return:
	//   similarity float64 — nilai 0.0 (beda orang) sampai 1.0 (identik)
	//   error               — network error
	//
	// Perbandingan dilakukan di Python karena numpy lebih efisien untuk
	// operasi vektor daripada implementasi manual di Go.
	CompareEmbeddings(embeddingNew []float64, embeddingStored []float64) (float64, error)
}

// ─────────────────────────────────────────
// InsightFaceEngine — implementasi via Python microservice
//
// Python service berjalan di port 8001 dengan dua endpoint:
//   POST /detect  — terima base64, return embedding + face_count
//   POST /compare — terima dua embedding, return similarity score
//
// Cara pakai di main.go:
//   engine := face.NewInsightFaceEngine("http://localhost:8001")
// ─────────────────────────────────────────

type InsightFaceEngine struct {
	ServiceURL string
	httpClient *http.Client
}

func NewInsightFaceEngine(serviceURL string) *InsightFaceEngine {
	return &InsightFaceEngine{
		ServiceURL: serviceURL,
		httpClient: &http.Client{},
	}
}

// detectResponse — struktur JSON dari Python /detect
type detectResponse struct {
	Detected  bool      `json:"detected"`
	FaceCount int       `json:"face_count"`
	Embedding []float64 `json:"embedding"`
	Error     string    `json:"error,omitempty"`
}

// DetectAndEmbed — kirim base64 ke Python, dapatkan embedding
func (e *InsightFaceEngine) DetectAndEmbed(imageBase64 string) ([]float64, int, error) {
	// ✦ DIUBAH: dari multipart/file ke JSON body dengan base64
	// Alasan: Python service kita sudah didesain terima base64,
	// lebih sederhana dan tidak perlu handle multipart di Python
	payload := map[string]string{"image": imageBase64}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal encode payload: %w", err)
	}

	req, err := http.NewRequest("POST", e.ServiceURL+"/detect", bytes.NewReader(body))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("gagal hubungi face engine: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var result detectResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return nil, 0, fmt.Errorf("gagal parse response engine: %w", err)
	}
	if result.Error != "" {
		return nil, result.FaceCount, fmt.Errorf("face engine: %s", result.Error)
	}

	return result.Embedding, result.FaceCount, nil
}

// compareRequest — struktur JSON untuk Python /compare
type compareRequest struct {
	EmbeddingNew    []float64 `json:"embedding_new"`
	EmbeddingStored []float64 `json:"embedding_stored"`
}

// compareResponse — struktur JSON dari Python /compare
type compareResponse struct {
	Similarity float64 `json:"similarity"`
	Match      bool    `json:"match"`
	Error      string  `json:"error,omitempty"`
}

// CompareEmbeddings — kirim dua embedding ke Python, dapatkan similarity score
func (e *InsightFaceEngine) CompareEmbeddings(embeddingNew []float64, embeddingStored []float64) (float64, error) {
	payload := compareRequest{
		EmbeddingNew:    embeddingNew,
		EmbeddingStored: embeddingStored,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("gagal encode payload: %w", err)
	}

	req, err := http.NewRequest("POST", e.ServiceURL+"/compare", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("gagal hubungi face engine: %w", err)
	}
	defer resp.Body.Close()

	respBytes, _ := io.ReadAll(resp.Body)
	var result compareResponse
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return 0, fmt.Errorf("gagal parse response engine: %w", err)
	}
	if result.Error != "" {
		return 0, fmt.Errorf("face engine: %s", result.Error)
	}

	return result.Similarity, nil
}

// ─────────────────────────────────────────
// MockEngine — untuk testing tanpa Python service
//
// Cara pakai di development:
//   engine := face.NewMockEngine(0.85) // selalu return 0.85 similarity
// ─────────────────────────────────────────

type MockEngine struct {
	FixedSimilarity float64
	FixedFaceCount  int
}

func NewMockEngine(similarity float64) *MockEngine {
	return &MockEngine{FixedSimilarity: similarity, FixedFaceCount: 1}
}

func (m *MockEngine) DetectAndEmbed(imageBase64 string) ([]float64, int, error) {
	// Return embedding dummy 512 dimensi semua bernilai 0.1
	embedding := make([]float64, 512)
	for i := range embedding {
		embedding[i] = 0.1
	}
	return embedding, m.FixedFaceCount, nil
}

func (m *MockEngine) CompareEmbeddings(embeddingNew []float64, embeddingStored []float64) (float64, error) {
	return m.FixedSimilarity, nil
}
