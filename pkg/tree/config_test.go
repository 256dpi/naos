package tree

import "testing"

func TestParseSize(t *testing.T) {
	for _, item := range []struct {
		str  string
		want int64
	}{
		{"0x4000", 0x4000},
		{"0x61999", 0x61999},
		{"16K", 16 * 1024},
		{"1756K", 1756 * 1024},
		{"1M", 1024 * 1024},
		{"4096", 4096},
	} {
		got, err := parseSize(item.str)
		if err != nil {
			t.Fatalf("%s: %v", item.str, err)
		}
		if got != item.want {
			t.Errorf("%s = %d, want %d", item.str, got, item.want)
		}
	}

	// the generator never emits these, but they must not parse as zero
	for _, str := range []string{"", "abc", "0xZZ"} {
		if _, err := parseSize(str); err == nil {
			t.Errorf("expected %q to fail", str)
		}
	}
}
