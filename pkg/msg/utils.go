package msg

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
)

func b2u(b bool) uint8 {
	return b2v(b, uint8(1), uint8(0))
}

func b2v[T any](b bool, t, f T) T {
	if b {
		return t
	} else {
		return f
	}
}

func random(n int) []byte {
	handle := make([]byte, n)
	_, err := rand.Read(handle)
	if err != nil {
		panic(err)
	}
	return handle
}

// Pack a byte buffer as specified by the format.
func Pack(fmt string, args ...any) []byte {
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

// ErrShortBuffer is returned when the buffer is too short for unpacking.
var ErrShortBuffer = errors.New("short buffer")

// Unpack a byte buffer as specified by the format.
func Unpack(fmt string, buffer []byte) ([]any, error) {
	// prepare result
	result := make([]any, 0, len(fmt))

	// read arguments
	pos := 0
	for _, code := range fmt {
		switch code {
		case 's':
			if pos >= len(buffer) {
				return nil, ErrShortBuffer
			}
			end := bytes.IndexByte(buffer[pos:], 0)
			if end == -1 {
				end = len(buffer) - pos
			}
			result = append(result, string(buffer[pos:pos+end]))
			pos += end + 1
		case 'b':
			result = append(result, buffer[pos:])
			pos += len(buffer) - pos
		case 'o':
			if pos+1 > len(buffer) {
				return nil, ErrShortBuffer
			}
			result = append(result, buffer[pos])
			pos++
		case 'h':
			if pos+2 > len(buffer) {
				return nil, ErrShortBuffer
			}
			result = append(result, binary.LittleEndian.Uint16(buffer[pos:]))
			pos += 2
		case 'i':
			if pos+4 > len(buffer) {
				return nil, ErrShortBuffer
			}
			result = append(result, binary.LittleEndian.Uint32(buffer[pos:]))
			pos += 4
		case 'q':
			if pos+8 > len(buffer) {
				return nil, ErrShortBuffer
			}
			result = append(result, binary.LittleEndian.Uint64(buffer[pos:]))
			pos += 8
		default:
			panic("invalid format")
		}
	}

	return result, nil
}
