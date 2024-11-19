package msg

import "crypto/rand"

func random(n int) []byte {
	handle := make([]byte, n)
	_, err := rand.Read(handle)
	if err != nil {
		panic(err)
	}
	return handle
}
