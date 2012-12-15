package nwa

import (
	"bytes"
	"encoding/binary"
	"io"
	"unsafe"
)

// Only works for little endian data
func getBits(data *uintptr, shift *uint, bits uint) uint {
	if *shift > 8 {
		*data++
		*shift -= 8
	}
	ret := uint16((*(*uint16)(unsafe.Pointer(*data + 1))<<8)|*(*uint16)(unsafe.Pointer(*data))) >> *shift
	*shift += bits
	return uint(ret & ((1 << bits) - 1))
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
