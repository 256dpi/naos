package tree

import (
	"fmt"
	"strings"
	"testing"
)

// sizes returns the generated partition sizes by name.
func sizes(t *testing.T, table string) map[string]int64 {
	t.Helper()

	result := map[string]int64{}
	for _, line := range strings.Split(table, "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		row := strings.Split(line, ",")
		if len(row) < 5 {
			continue
		}
		size, err := parseSize(strings.TrimSpace(row[4]))
		if err != nil {
			t.Fatalf("failed to parse size of %q: %v", row[0], err)
		}
		result[strings.TrimSpace(row[0])] = size
	}

	return result
}

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

func TestPartitionsGenerate(t *testing.T) {
	for _, p := range []Partitions{
		{Total: 4, Alpha: 45, Beta: 45, Storage: 10},
		{Total: 4, Alpha: 40, Beta: 40, Storage: 20},
		{Total: 8, Alpha: 45, Beta: 45, Storage: 10},
		{Total: 16, Alpha: 30, Beta: 30, Storage: 40},
		{Total: 4, Alpha: 80, Beta: 0, Storage: 20},
	} {
		t.Run(fmt.Sprintf("%dMB-%d-%d-%d", p.Total, p.Alpha, p.Beta, p.Storage), func(t *testing.T) {
			table, err := p.generate()
			if err != nil {
				t.Fatal(err)
			}
			got := sizes(t, table)

			// every sized partition must be present and non-empty
			names := []string{"alpha", "storage", "coredump"}
			if p.Beta > 0 {
				names = append(names, "beta")
			}
			for _, name := range names {
				if got[name] <= 0 {
					t.Errorf("missing or empty partition %q", name)
				}
			}

			// everything must fit the flash, with 64K taken up by the
			// partitions before "alpha" and 64K by the trailing coredump
			//
			// note: this is not an exact fit, see the TODO in generate()
			total := 64*1024 + got["alpha"] + got["beta"] + got["storage"] + got["coredump"]
			if want := int64(p.Total) * 1024 * 1024; total > want {
				t.Errorf("partitions add up to %d, which exceeds %d", total, want)
			}
		})
	}
}

func TestPartitionsDoNotAddUp(t *testing.T) {
	_, err := (&Partitions{Total: 4, Alpha: 50, Beta: 40, Storage: 5}).generate()
	if err == nil {
		t.Fatal("expected an error")
	}
}
