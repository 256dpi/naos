package msg

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
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

func unpack(fmt string, buffer []byte) []any {
	// prepare result
	result := make([]any, 0, len(fmt))

	// read arguments
	pos := 0
	for _, code := range fmt {
		switch code {
		case 's':
			length := bytes.IndexByte(buffer[pos:], 0)
			if length == -1 {
				length = len(buffer) - pos
			}
			result = append(result, string(buffer[pos:pos+length]))
			pos += length
		case 'b':
			result = append(result, buffer[pos:])
			pos += len(buffer) - pos
		case 'o':
			result = append(result, buffer[pos])
			pos++
		case 'h':
			result = append(result, binary.LittleEndian.Uint16(buffer[pos:]))
			pos += 2
		case 'i':
			result = append(result, binary.LittleEndian.Uint32(buffer[pos:]))
			pos += 4
		case 'q':
			result = append(result, binary.LittleEndian.Uint64(buffer[pos:]))
			pos += 8
		default:
			panic("invalid format")
		}
	}

	return result
}
