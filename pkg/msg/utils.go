package msg

import (
	"crypto/rand"
	"encoding/binary"
)

func random(n int) []byte {
	handle := make([]byte, n)
	_, err := rand.Read(handle)
	if err != nil {
		panic(err)
	}
	return handle
}

func pack(fmt string, args ...any) []byte {
	// calculate size
	size := 0
	for i, code := range fmt {
		switch code {
		case 's':
			size += len(args[i].(string))
		case 'b':
			size += len(args[i].([]byte))
		case 'o':
			size += 1
		case 'h':
			size += 2
		case 'i':
			size += 4
		case 'q':
			size += 8
		default:
			panic("invalid format")
		}
	}

	// create buffer
	buffer := make([]byte, size)

	// write arguments
	pos := 0
	for i, code := range fmt {
		switch code {
		case 's':
			copy(buffer[pos:], args[i].(string))
			pos += len(args[i].(string))
		case 'b':
			copy(buffer[pos:], args[i].([]byte))
			pos += len(args[i].([]byte))
		case 'o':
			buffer[pos] = args[i].(byte)
			pos++
		case 'h':
			binary.LittleEndian.PutUint16(buffer[pos:], args[i].(uint16))
			pos += 2
		case 'i':
			binary.LittleEndian.PutUint32(buffer[pos:], args[i].(uint32))
			pos += 4
		case 'q':
			binary.LittleEndian.PutUint64(buffer[pos:], args[i].(uint64))
			pos += 8
		default:
			panic("invalid format")
		}
	}

	return buffer
}
