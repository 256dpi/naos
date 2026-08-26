package tree

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"sort"
	"unicode/utf8"

	"tinygo.org/x/espflasher/pkg/nvs"

	"github.com/256dpi/naos/pkg/utils"
)

// Config will write settings and parameters to an attached device.
func Config(naosPath string, values map[string]string, port, baudRate string, out io.Writer) error {
	// read flasher arguments, as the flash settings depend on the target
	args, err := readFlasherArgs(naosPath)
	if err != nil {
		return err
	}

	// connect to device
	utils.Log(out, "Connecting...")
	flasher, err := connectDevice(port, baudRate, args, out)
	if err != nil {
		return err
	}
	defer flasher.Close()

	// reset device on return, to leave it running the application
	defer flasher.Reset()

	// find NVS partition
	nvsPart, err := findDevicePartition(flasher, naosPath, "nvs")
	if err != nil {
		return err
	}

	// generate image
	utils.Log(out, "Generating image...")
	image, err := generateImage(values, int(nvsPart.Size))
	if err != nil {
		return err
	}

	// flash image
	utils.Log(out, "Flashing...")
	err = flasher.FlashImage(image, uint32(nvsPart.Offset), progressLogger(out))
	if err != nil {
		return err
	}

	return nil
}

// chunkMaxSize is the maximum size of a single blob chunk, which is the space
// left on a page beside the chunk's own entry.
const chunkMaxSize = nvs.EntrySize * (nvs.EntriesPerPage - 1)

// generateImage will generate an NVS partition image that stores the provided
// parameters.
func generateImage(values map[string]string, size int) ([]byte, error) {
	// assemble entries, sorting the keys to get a stable order
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var entries []nvs.Entry
	for _, key := range keys {
		// check size, as the chunk index and count are stored as a single byte
		if len(values[key]) > 255*chunkMaxSize {
			return nil, fmt.Errorf("value of %q is too big", key)
		}

		entries = append(entries, blobEntries(key, []byte(values[key]))...)
	}

	return nvs.GenerateNVS(entries, size)
}

// blobEntries will assemble the entries that store the provided value: one data
// entry per chunk and an index entry that describes the chunks. The generator
// only supports the single entry blobs of the earlier format, which are limited
// to one page, therefore the chunked entries written by ESP-IDF itself are
// assembled as raw entries.
func blobEntries(key string, value []byte) []nvs.Entry {
	// split value into chunks, storing empty values as a single empty chunk
	var chunks [][]byte
	for offset := 0; offset < len(value); offset += chunkMaxSize {
		chunks = append(chunks, value[offset:min(offset+chunkMaxSize, len(value))])
	}
	if len(chunks) == 0 {
		chunks = append(chunks, nil)
	}

	// prepare data entries
	entries := make([]nvs.Entry, 0, len(chunks)+1)
	for i, chunk := range chunks {
		// prepare data, padding the chunk to full entries with the erased state
		data := bytes.Repeat([]byte{0xff}, 8+alignSize(len(chunk)))
		copy(data[8:], chunk)

		// write header ("size, reserved, CRC")
		binary.LittleEndian.PutUint16(data[0:2], uint16(len(chunk)))
		binary.LittleEndian.PutUint32(data[4:8], espCRC32(chunk))

		// collect entry
		entries = append(entries, nvs.Entry{
			Namespace:  "naos",
			Key:        key,
			Raw:        true,
			TypeByte:   typeBlobData,
			Span:       uint8(1 + alignSize(len(chunk))/nvs.EntrySize),
			ChunkIndex: uint8(i),
			Data:       data,
		})
	}

	// prepare index ("size, chunk count, chunk start, reserved")
	index := bytes.Repeat([]byte{0xff}, 8)
	binary.LittleEndian.PutUint32(index[0:4], uint32(len(value)))
	index[4] = uint8(len(chunks))
	index[5] = 0

	// collect index entry
	entries = append(entries, nvs.Entry{
		Namespace:  "naos",
		Key:        key,
		Raw:        true,
		TypeByte:   typeBlobIndex,
		Span:       1,
		ChunkIndex: chunkIndexNone,
		Data:       index,
	})

	return entries
}

// alignSize will round the provided size up to full entries.
func alignSize(size int) int {
	return (size + nvs.EntrySize - 1) / nvs.EntrySize * nvs.EntrySize
}

// espCRC32 will calculate the CRC as used by NVS.
func espCRC32(data []byte) uint32 {
	return crc32.Update(0xffffffff, crc32.IEEETable, data)
}

// ReadConfig will read the parameters stored on an attached device.
func ReadConfig(naosPath, port, baudRate string, out io.Writer) (map[string]string, error) {
	// connect to device
	utils.Log(out, "Connecting...")
	flasher, err := connectDevice(port, baudRate, nil, out)
	if err != nil {
		return nil, err
	}
	defer flasher.Close()

	// reset device on return, to leave it running the application
	defer flasher.Reset()

	// find NVS partition
	nvsPart, err := findDevicePartition(flasher, naosPath, "nvs")
	if err != nil {
		return nil, err
	}

	// read partition
	utils.Log(out, "Reading...")
	data, err := flasher.ReadFlash(uint32(nvsPart.Offset), uint32(nvsPart.Size), progressLogger(out))
	if err != nil {
		return nil, err
	}

	// parse partition
	entries, err := nvs.ParseNVS(data)
	if err != nil {
		return nil, err
	}

	return collectValues(entries, out), nil
}

// The entry types ESP-IDF uses to store a blob as a set of chunks, and the
// chunk index used by entries that are not a chunk. The NVS parser does not
// decode chunked blobs and yields their entries as raw entries instead.
const (
	typeBlobData   = 0x42
	typeBlobIndex  = 0x48
	chunkIndexNone = 0xff
)

// collectValues will extract the parameters stored in the "naos" namespace from
// the provided NVS entries. Values are stored as blobs, which ESP-IDF splits
// into chunks that must be reassembled in order.
func collectValues(entries []nvs.Entry, out io.Writer) map[string]string {
	// collect values and chunks
	values := map[string]string{}
	chunks := map[string]map[uint8][]byte{}
	for _, entry := range entries {
		// skip other namespaces
		if entry.Namespace != "naos" {
			continue
		}

		// handle legacy single entry blobs
		if !entry.Raw && entry.Type == "blob" {
			if value, ok := entry.Value.([]byte); ok {
				values[entry.Key] = string(value)
			}
			continue
		}

		// skip other entries, especially the blob index entries, which only
		// describe the chunks collected below
		if !entry.Raw || entry.TypeByte != typeBlobData {
			continue
		}

		// get size, which is stored in the first two bytes of the entry header
		if len(entry.Data) < 8 {
			continue
		}
		size := int(binary.LittleEndian.Uint16(entry.Data[0:2]))
		if 8+size > len(entry.Data) {
			continue
		}

		// store chunk
		if chunks[entry.Key] == nil {
			chunks[entry.Key] = map[uint8][]byte{}
		}
		chunks[entry.Key][entry.ChunkIndex] = entry.Data[8 : 8+size]
	}

	// assemble chunked values
	for key, parts := range chunks {
		// sort chunks
		indexes := make([]int, 0, len(parts))
		for index := range parts {
			indexes = append(indexes, int(index))
		}
		sort.Ints(indexes)

		// concatenate chunks
		var value []byte
		for _, index := range indexes {
			value = append(value, parts[uint8(index)]...)
		}

		values[key] = string(value)
	}

	// drop binary values, as they cannot be represented in the configuration
	for key, value := range values {
		if !utf8.ValidString(value) {
			utils.Log(out, fmt.Sprintf("Skipping binary value: %s", key))
			delete(values, key)
		}
	}

	return values
}
