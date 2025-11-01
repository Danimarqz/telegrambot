package metrics

import "testing"

func TestHuman(t *testing.T) {
	tests := []struct {
		name  string
		value uint64
		want  string
	}{
		{name: "zero", value: 0, want: "0B"},
		{name: "bytes", value: 512, want: "512B"},
		{name: "kilobytes", value: 1024, want: "1.0KB"},
		{name: "megabytes", value: 5 * 1024 * 1024, want: "5.0MB"},
		{name: "gigabytes", value: 3 * 1024 * 1024 * 1024, want: "3.0GB"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := human(tt.value); got != tt.want {
				t.Errorf("human(%d) = %s, want %s", tt.value, got, tt.want)
			}
		})
	}
}
