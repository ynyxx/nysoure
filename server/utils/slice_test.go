package utils

import (
	"reflect"
	"testing"
)

func TestIntersectPreserveOrder(t *testing.T) {
	tests := []struct {
		name  string
		base  []uint
		other []uint
		want  []uint
	}{
		{
			name:  "keeps base order",
			base:  []uint{5, 1, 4, 2},
			other: []uint{2, 5, 9},
			want:  []uint{5, 2},
		},
		{
			name:  "empty base",
			base:  nil,
			other: []uint{1, 2},
			want:  []uint{},
		},
		{
			name:  "empty other",
			base:  []uint{1, 2},
			other: nil,
			want:  []uint{},
		},
		{
			name:  "deduplicates base",
			base:  []uint{1, 2, 1, 3},
			other: []uint{1, 3},
			want:  []uint{1, 3},
		},
		{
			name:  "no overlap",
			base:  []uint{1, 2},
			other: []uint{3, 4},
			want:  []uint{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IntersectPreserveOrder(tt.base, tt.other)
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("IntersectPreserveOrder() = %v, want %v", got, tt.want)
			}
		})
	}
}
