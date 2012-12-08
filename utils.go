package nwa

import (
	"bytes"
	"encoding/binary"
	"io"
)

// NwaFileContainer has to be created with default values. It needs a data
// slice with the NWA file content with every other entry zeroed.
type NwaFileContainer struct {
	// TODO: We should probably use the io.Reader here
	data   []byte
	offset int
	shift  uint8
	ret    uint8
}

// getBits reads through the data and returns the requested bits. It will
// move the "cursor" forward with each call at bits range.
func (c *NwaFileContainer) getBits(bits uint8) uint8 {
	if c.shift > 8 {
		c.offset++
		c.shift -= 8
	}
	// TODO: This needs a reader...
	//binary.Read(c.data[c.offset], binary.LittleEndian, &c.ret)
	c.shift += bits
	return c.ret & ((1 << bits) - 1) // mask
}

func makeWavHeader(size int, channels int, bps int, freq int) io.Reader {
	var byps int16 = (int16(bps) + 7) >> 3
	wavheader := new(bytes.Buffer)
	binary.Write(wavheader, binary.LittleEndian, [...]byte{'R', 'I', 'F', 'F'})
	binary.Write(wavheader, binary.LittleEndian, int32(size)+0x24)
	binary.Write(wavheader, binary.LittleEndian, [...]byte{'W', 'A', 'V', 'E'})
	binary.Write(wavheader, binary.LittleEndian, [...]byte{'f', 'm', 't', ' '})
	binary.Write(wavheader, binary.LittleEndian, [...]byte{16, 0, 0, 0})
	binary.Write(wavheader, binary.LittleEndian, [...]byte{1, 0})
	binary.Write(wavheader, binary.LittleEndian, int16(channels))
	binary.Write(wavheader, binary.LittleEndian, int32(freq))
	binary.Write(wavheader, binary.LittleEndian, int32(byps)*int32(freq*channels))
	binary.Write(wavheader, binary.LittleEndian, byps*int16(channels))
	binary.Write(wavheader, binary.LittleEndian, int16(bps))
	binary.Write(wavheader, binary.LittleEndian, [...]byte{'d', 'a', 't', 'a'})
	binary.Write(wavheader, binary.LittleEndian, int32(size))
	return wavheader
}
