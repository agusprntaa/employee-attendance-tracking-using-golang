package attendance

import "testing"

func TestRequiresFaceToken(t *testing.T) {
	tests := []struct {
		name     string
		workType string
		want     bool
	}{
		{name: "wfo requires face token", workType: "WFO", want: true},
		{name: "wfa does not require face token", workType: "WFA", want: false},
		{name: "empty defaults to wfo", workType: "", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := requiresFaceToken(CheckInRequest{WorkType: tt.workType}); got != tt.want {
				t.Fatalf("requiresFaceToken(%q) = %v, want %v", tt.workType, got, tt.want)
			}
		})
	}
}
