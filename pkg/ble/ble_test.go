package ble

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TODO: Resolve dependency on real device.

func TestConfig(t *testing.T) {
	if testing.Short() {
		return
	}
	
	err := Config(map[string]string{
		"var_s": "foo",
	}, 2*time.Second, os.Stdout)
	assert.NoError(t, err)
}
