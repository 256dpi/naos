package tree

import "testing"

func TestCoredumpLength(t *testing.T) {
	for _, item := range []struct {
		header []byte
		want   uint32
		fail   bool
	}{
		// a stored dump reports its own length
		{[]byte{0x34, 0x12, 0, 0}, 0x1234, false},
		// an erased partition has no dump
		{[]byte{0xff, 0xff, 0xff, 0xff}, 0, true},
		// an implausible length falls back to the partition size
		{[]byte{0, 0, 0, 0}, 0x10000, false},
		{[]byte{0, 0, 0xff, 0}, 0x10000, false},
	} {
		got, err := coredumpLength(item.header, 0x10000)
		if (err != nil) != item.fail {
			t.Errorf("%x: unexpected error: %v", item.header, err)
		} else if got != item.want {
			t.Errorf("%x: got %d, want %d", item.header, got, item.want)
		}
	}
}
