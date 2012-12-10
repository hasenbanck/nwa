package nwa

import (
	"bytes"
	"encoding/binary"
	"io"
)

type bitReader struct {
	in      io.ByteReader
	current byte
	bit_pos uint
	err     error
}

func newBitReader(in io.ByteReader) *bitReader {
	var br bitReader
	br.in = in
	br.bit_pos = 8
	return &br
}

func (br *bitReader) readAtMost(n uint) (read uint, bits uint) {
	bits = uint(br.current)
	bits = bits >> uint(br.bit_pos)
	bits = bits & ((1 << uint(n)) - 1)
	read = 8 - br.bit_pos
	if read > n {
		read = n
	}
	br.bit_pos += read
	if br.bit_pos == 8 {
		br.bit_pos = 0
		br.current, br.err = br.in.ReadByte()
		if br.err != nil {
			panic(br.err)
		}
	}
	return
}

func (br *bitReader) ReadBits(n uint) uint {
	var bits uint
	var pos uint = 0
	for n > 0 {
		read, next := br.readAtMost(n)
		bits = bits | (next << uint(pos))
		pos += read
		n -= read
	}
	return bits
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
