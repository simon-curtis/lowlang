package link

import "testing"

func TestXXH3Long(t *testing.T) {
	vectors := []struct {
		length int
		want   uint64
	}{
		{241, 0x82b1de299f6e411e},
		{1024, 0xf75e768c7cdd54b2},
		{1025, 0x667a5eabe344e5df},
		{2048, 0x9e5e4a8160109a5d},
		{2049, 0x242c61fe5ed5db14},
		{5000, 0x8e5898f51713d386},
		{20000, 0x05b36829a6c3c68f},
	}
	for _, v := range vectors {
		data := make([]byte, v.length)
		for i := range data {
			data[i] = byte((i*7 + 3) % 251)
		}
		if got := xxh3Long(data); got != v.want {
			t.Errorf("length %d: got %#x, want %#x", v.length, got, v.want)
		}
	}
}
