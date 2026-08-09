package tree

import (
	"bytes"
	"testing"
)

func filterOutput(t *testing.T, chunks ...string) string {
	t.Helper()

	// filter chunks
	var buf bytes.Buffer
	filter := filterClosingMessage(&buf)
	for _, chunk := range chunks {
		_, err := filter.Write([]byte(chunk))
		if err != nil {
			t.Fatal(err)
		}
	}
	err := filter.Flush()
	if err != nil {
		t.Fatal(err)
	}

	return buf.String()
}

func TestClosingFilterDropsMessage(t *testing.T) {
	out := filterOutput(t, "[100/100] Done\n\nProject build complete. To flash, run:\n idf.py flash\nor\n idf.py -p PORT flash\n")
	if out != "[100/100] Done\n" {
		t.Fatalf("expected message to be dropped, got %q", out)
	}
}

func TestClosingFilterHandlesChunks(t *testing.T) {
	out := filterOutput(t, "[100/100] ", "Done\n\nApp build compl", "ete. To flash, run:\n idf.py app-flash\n")
	if out != "[100/100] Done\n" {
		t.Fatalf("expected message to be dropped, got %q", out)
	}
}

func TestClosingFilterKeepsOutput(t *testing.T) {
	out := filterOutput(t, "one\n\ntwo\n\nthree")
	if out != "one\n\ntwo\n\nthree" {
		t.Fatalf("expected output to be kept, got %q", out)
	}
}

func TestClosingFilterKeepsSimilarLines(t *testing.T) {
	out := filterOutput(t, "note: the project build completed\nProject build complete!\n")
	if out != "note: the project build completed\nProject build complete!\n" {
		t.Fatalf("expected output to be kept, got %q", out)
	}
}
