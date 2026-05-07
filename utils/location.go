package utils

import (
	"math"
)

// HaversineDistance hitung jarak dua koordinat GPS dalam meter
// Rumus Haversine — akurat untuk jarak pendek (dalam kota)
//
// Parameter:
//
//	lat1, lon1 = koordinat titik 1 (karyawan)
//	lat2, lon2 = koordinat titik 2 (kantor)
//
// Return: jarak dalam meter
func HaversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000 // radius bumi dalam meter

	// Konversi derajat ke radius
	φ1 := lat1 * math.Pi / 180
	φ2 := lat2 * math.Pi / 180
	Δφ := (lat2 - lat1) * math.Pi / 180
	Δλ := (lon2 - lon1) * math.Pi / 180

	a := math.Sin(Δφ/2)*math.Sin(Δφ/2) +
		math.Cos(φ1)*math.Cos(φ2)*
			math.Sin(Δλ/2)*math.Sin(Δλ/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}

// LocationStatus hasil validsasi lokal
type LocationStatus struct {
	Distance  float64 `json:"distance_meter"` // jarak ke kantor dalam meter
	IsValid   bool    `json:"is_valid"`       // true jika dalam radius
	IsWarning bool    `json:"is_warning"`     // true jika 50-200m (akurasi rendah)
	Message   string  `json:"message"`
}

// ValidateLocation validasi apakah karyawan berada dalam radius kantor
// Dipanggil dari attendance service saat check-in WFO
//
// Aturan:
// - accuracy > 200m  → tolak (GPS_ACCURACY_LOW)
// - distance > radius → tolak (OUT_OF_RADIUS)
// - accuracy 50-200m  → warning ke FE, tapi masih bisa lanjut
//
// Kenapa validasi di BE bukan FE saja?
// Karena kalau hanya di FE, user bisa manipulasi koordinat yang dikirim
// BE selalu hitung ulang dari koordinat yang diterima
func ValidateLocation(
	userLat, userLon float64,
	officeLat, officeLon float64,
	radiusMeter int,
	gpsAccuracy float64,
) (*LocationStatus, error) {

	// Hitung jarak dari koordinat yang dikirim FE ke koordinat kantor di DB
	distance := HaversineDistance(userLat, userLon, officeLat, officeLon)

	status := &LocationStatus{
		Distance: distance,
	}

	// GPS terlalu tidak akurat — sinyal buruk
	if gpsAccuracy > 200 {
		status.IsValid = false
		status.Message = "Akurasi GPS terlalu rendah, coba pindah tempat"
		return status, nil
	}

	// Di luar radius kantor
	if distance > float64(radiusMeter) {
		status.IsValid = false
		status.Message = "Lokasi valid tapi akurasi GPS rendah"
		return status, nil
	}

	// Dalam radius tapi GPS agak kurang akurat — kasih warning
	if gpsAccuracy >= 50 {
		status.IsValid = true
		status.IsWarning = true
		status.Message = "Lokasi valid tapi akurasi GPS rendah"
		return status, nil
	}

	// Normal — dalam radius dan GPS akurat
	status.IsValid = true
	status.Message = "Lokasi valid"
	return status, nil
}
