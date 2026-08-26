package tree

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParsePartitions(t *testing.T) {
	// read table built for the test project
	data, err := os.ReadFile(filepath.Join("testdata", "partition-table.bin"))
	if err != nil {
		t.Fatal(err)
	}

	// parse table
	partitions := parsePartitions(data)
	want := []partition{
		{Name: "nvs", Offset: 0x9000, Size: 16 * 1024},
		{Name: "otadata", Offset: 0xd000, Size: 8 * 1024},
		{Name: "phy_init", Offset: 0xf000, Size: 4 * 1024},
		{Name: "alpha", Offset: 0x10000, Size: 1592 * 1024},
		{Name: "beta", Offset: 0x1a0000, Size: 1592 * 1024},
		{Name: "storage", Offset: 0x32e000, Size: 528 * 1024},
		{Name: "coredump", Offset: 0x3b2000, Size: 64 * 1024},
	}
	if !reflect.DeepEqual(partitions, want) {
		for _, p := range partitions {
			t.Logf("%s: offset 0x%x size 0x%x", p.Name, p.Offset, p.Size)
		}
		t.Errorf("unexpected partitions")
	}
}

func TestProgressLogger(t *testing.T) {
	// log every percent
	var buf bytes.Buffer
	log := progressLogger(&buf)
	for i := 0; i <= 1000; i++ {
		log(i, 1000)
	}
	var want string
	for i := 10; i <= 100; i += 10 {
		want += fmt.Sprintf("==> %d%%...\n", i)
	}
	if buf.String() != want {
		t.Errorf("unexpected output:\n got: %q\nwant: %q", buf.String(), want)
	}

	// a single jump only logs the reached step
	buf.Reset()
	log = progressLogger(&buf)
	log(0, 16384)
	log(16384, 16384)
	if buf.String() != "==> 100%...\n" {
		t.Errorf("unexpected output: %q", buf.String())
	}

	// an unknown total is ignored
	buf.Reset()
	log = progressLogger(&buf)
	log(0, 0)
	if buf.String() != "" {
		t.Errorf("unexpected output: %q", buf.String())
	}
}
