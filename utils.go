package nwa

import (
//"encoding/binary"
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
