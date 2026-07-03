package service

import (
	"math"
	"testing"
)

func TestHaversine(t *testing.T) {
	tests := []struct {
		name                   string
		lat1, lon1, lat2, lon2 float64
		wantMeters             float64
		toleranceMeters        float64
	}{
		{
			name: "ponto identico",
			lat1: -23.5505, lon1: -46.6333,
			lat2: -23.5505, lon2: -46.6333,
			wantMeters: 0, toleranceMeters: 0.001,
		},
		{
			name: "aproximadamente 1 km na latitude",
			// 1 grau de latitude ~ 111.32 km; 0.009 graus ~ 1002 m
			lat1: -23.5505, lon1: -46.6333,
			lat2: -23.5415, lon2: -46.6333,
			wantMeters: 1001, toleranceMeters: 5,
		},
		{
			name: "praca da se ate masp (~2.3 km)",
			lat1: -23.5503, lon1: -46.6339,
			lat2: -23.5614, lon2: -46.6559,
			wantMeters: 2560, toleranceMeters: 100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := haversine(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if math.Abs(got-tt.wantMeters) > tt.toleranceMeters {
				t.Errorf("haversine() = %.2f m, esperado %.2f m (+/- %.2f)", got, tt.wantMeters, tt.toleranceMeters)
			}
		})
	}
}

func TestClassificarGeofence(t *testing.T) {
	postoLat, postoLon := -23.5505, -46.6333
	raioM := 100

	tests := []struct {
		name     string
		lat, lon float64
		want     string
	}{
		{"no centro do posto", postoLat, postoLon, "ok"},
		{"dentro do raio (~50 m)", postoLat + 0.00045, postoLon, "ok"},
		{"fora do raio (~200 m)", postoLat + 0.0018, postoLon, "desvio_rota"},
		{"muito longe (~1 km)", postoLat + 0.009, postoLon, "desvio_rota"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classificarGeofence(tt.lat, tt.lon, postoLat, postoLon, raioM)
			if got != tt.want {
				t.Errorf("classificarGeofence() = %q, esperado %q", got, tt.want)
			}
		})
	}
}
