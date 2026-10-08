package quarantine

import "testing"

func TestCap(t *testing.T) {
	tests := []struct {
		name  string
		tests int
		want  int
	}{
		{name: "none", tests: 0, want: 0},
		{name: "lab sized", tests: 6, want: 1},
		{name: "nineteen", tests: 19, want: 1},
		{name: "twenty", tests: 20, want: 1},
		{name: "forty", tests: 40, want: 2},
		{name: "hundred", tests: 100, want: 5},
		{name: "thousand", tests: 1000, want: 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Cap(tt.tests, 5, 10); got != tt.want {
				t.Fatalf("Cap(%d) = %d, want %d", tt.tests, got, tt.want)
			}
		})
	}
}
