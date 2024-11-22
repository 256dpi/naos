package msg

import (
	"bytes"
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

func unpack(fmt string, buffer []byte, args ...any) {
	// read arguments
	pos := 0
	for i, code := range fmt {
		switch code {
		case 's':
			ptr := args[i].(*string)
			n := bytes.IndexByte(buffer[pos:], 0)
			if n == -1 {
				n = len(buffer) - pos
			}
			*ptr = string(buffer[pos : pos+n])
			pos += len(*ptr)
		case 'b':
			ptr := args[i].(*[]byte)
			*ptr = buffer[pos:]
			pos += len(*ptr)
		case 'o':
			ptr := args[i].(*byte)
			*ptr = buffer[pos]
			pos++
		case 'h':
			ptr := args[i].(*uint16)
			*ptr = binary.LittleEndian.Uint16(buffer[pos:])
			pos += 2
		case 'i':
			ptr := args[i].(*uint32)
			*ptr = binary.LittleEndian.Uint32(buffer[pos:])
			pos += 4
		case 'q':
			ptr := args[i].(*uint64)
			*ptr = binary.LittleEndian.Uint64(buffer[pos:])
			pos += 8
		default:
			panic("invalid format")
		}
	}
}
