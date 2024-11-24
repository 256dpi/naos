package msg

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const binPath = "/Users/256dpi/Development/GitHub/256dpi/naos/com/test/build/naos.bin"

func TestUpdate(t *testing.T) {
	// read binary
	data, err := os.ReadFile(binPath)
	assert.NoError(t, err)

	dev := NewHTTPDevice("10.0.1.7")

	ch, err := dev.Open()
	assert.NoError(t, err)
	assert.NotNil(t, ch)

	s, err := OpenSession(ch)
	assert.NoError(t, err)
	assert.NotNil(t, s)

	err = Update(s, data, func(pos int) {
		fmt.Printf("progress: %f\n", float64(pos)/float64(len(data))*100)
	}, 30*time.Second)
	assert.NoError(t, err)

	err = s.End(time.Second)
	assert.NoError(t, err)

	ch.Close()
}
