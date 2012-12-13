package nwa

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type NwaFile struct {
	Channels int // channels
	Bps      int // bits per sample
	Freq     int // samples per second
	Datasize int // all data size

	reader       io.Reader // Reader to read the NWA file from
	complevel    int       // compression level
	userunlength int       // run length encoding
	blocks       int       // block count
	compdatasize int       // compressed data size
	samplecount  int       // all samples
	blocksize    int       // samples per block
	restsize     int       // samples of the last block
	curblock     int
	offsets      []int

	tmpdata bytes.Buffer
	outdata *bytes.Buffer
}

func NewNwaFile(r io.Reader) (*NwaFile, error) {
	if r == nil {
		return nil, errors.New("Reader is nil.")
	}

	nf := &NwaFile{reader: r}

	if err := nf.readHeader(); err != nil {
		return nil, err
	}

	if err := nf.checkHeader(); err != nil {
		return nil, err
	}

	return nf, nil
}

func (nf *NwaFile) readHeader() error {
	nf.curblock = -1
	buffer := new(bytes.Buffer)
	if count, err := io.CopyN(buffer, nf.reader, 0x2c); count != 0x2c || err != nil {
		if err == nil {
			err = fmt.Errorf("Can't read the header. Read 0x%X bytes\n", count)
		}
		return err
	}

	var channels, bps int16
	var freq, complevel, userunlength, blocks, datasize, compdatasize, samplecount, blocksize, restsize, dummy int32

	binary.Read(buffer, binary.LittleEndian, &channels)
	binary.Read(buffer, binary.LittleEndian, &bps)
	binary.Read(buffer, binary.LittleEndian, &freq)
	binary.Read(buffer, binary.LittleEndian, &complevel)
	binary.Read(buffer, binary.LittleEndian, &userunlength)
	binary.Read(buffer, binary.LittleEndian, &blocks)
	binary.Read(buffer, binary.LittleEndian, &datasize)
	binary.Read(buffer, binary.LittleEndian, &compdatasize)
	binary.Read(buffer, binary.LittleEndian, &samplecount)
	binary.Read(buffer, binary.LittleEndian, &blocksize)
	binary.Read(buffer, binary.LittleEndian, &restsize)
	binary.Read(buffer, binary.LittleEndian, &dummy)

	nf.Channels = int(channels)
	nf.Bps = int(bps)
	nf.Freq = int(freq)
	nf.Datasize = int(datasize)
	nf.complevel = int(complevel)
	nf.userunlength = int(userunlength)
	nf.blocks = int(blocks)
	nf.compdatasize = int(compdatasize)
	nf.samplecount = int(samplecount)
	nf.blocksize = int(blocksize)
	nf.restsize = int(restsize)

	// Uncompressed wave
	if nf.complevel == -1 {
		nf.blocksize = 65536
		nf.restsize = (nf.datasize % (nf.blocksize * (nf.Bps / 8))) / (nf.Bps / 8)
		var rest int = 0
		if nf.restsize > 0 {
			rest = 1
		}
		nf.blocks = nf.Datasize/(nf.blocksize*(nf.Bps/8)) + rest
	}
	if nf.blocks <= 0 || nf.blocks > 1000000 {
		// There can't be a file with over 1hr music
		return fmt.Errorf("Blocks are too large: %d\n", nf.blocks)
	}
	if nf.complevel == -1 {
		return nil
	}

	// Read the offset index
	nf.offsets = make([]int, nf.blocks)
	for i := 0; i < nf.blocks; i++ {
		var tmp int32
		if err := binary.Read(nf.reader, binary.LittleEndian, &tmp); err != nil {
			return errors.New("Couldn't read the offset block")
		}
		nf.offsets[i] = int(tmp)
	}

	return nil
}

func (nf *NwaFile) checkHeader() error {
	if nf.complevel != -1 && nf.offsets == nil {
		return errors.New("No offsets set even thought they are needed")
	}
	if nf.Channels != 1 && nf.Channels != 2 {
		return fmt.Errorf("This library only supports mono / stereo data: data has %d channels\n", nf.Channels)
	}
	if nf.Bps != 8 && nf.Bps != 16 {
		return fmt.Errorf("This library only supports 8 / 16bit data: data is %d bits\n", nf.Bps)
	}
	if nf.complevel == -1 {
		var byps int = nf.Bps / 8 // Bytes per sample
		if nf.Datasize != nf.samplecount*byps {
			return fmt.Errorf("Invalid datasize: datasize %d != samplecount %d * samplesize %d\n", nf.Datasize, nf.samplecount, byps)
		}
		if nf.samplecount != (nf.blocks-1)*nf.blocksize+nf.restsize {
			return fmt.Errorf("Total sample count is invalid: samplecount %d != %d*%d+%d(block*blocksize+lastblocksize)\n", nf.samplecount, nf.blocks-1, nf.blocksize, nf.restsize)
		}
		return nil
	}
	if nf.complevel < 0 || nf.complevel > 5 {
		return fmt.Errorf("This library supports only compression level from -1 to 5: the compression level of the data is %d\n", nf.complevel)
	}
	if nf.offsets[nf.blocks-1] >= nf.compdatasize {
		return fmt.Errorf("The last offset overruns the file.\n")
	}
	var byps int = nf.Bps / 8 // Bytes per sample
	if nf.Datasize != nf.samplecount*byps {
		return fmt.Errorf("Invalid datasize: datasize %d != samplecount %d * samplesize %d\n", nf.Datasize, nf.samplecount, byps)
	}
	if nf.samplecount != (nf.blocks-1)*nf.blocksize+nf.restsize {
		return fmt.Errorf("Total sample count is invalid: samplecount %d != %d*%d+%d(block*blocksize+lastblocksize)\n", nf.samplecount, nf.blocks-1, nf.blocksize, nf.restsize)
	}
	return nil
}

// DecodeBlock decodes one block with each call. Returns the length of the written bytes.
// If the value is -1 there has been an error and 0 signals that there are no blocks left
// to decode.
func (nf *NwaFile) Read(p []byte) (n int, err error) {
	nf.outdata = bytes.NewBuffer(p)

	// Uncompressed wave data stream
	if nf.complevel == -1 {
		if nf.curblock == -1 {
			// If it's the first block we have to write the wave header
			nf.curblock++
			written, err := io.Copy(nf.outdata, makeWavHeader(nf.Datasize, nf.Channels, nf.Bps, nf.Freq))
			return int(written), err
		}
		if nf.curblock <= nf.blocks {
			nf.curblock++
			written, err := io.CopyN(nf.outdata, nf.reader, (int64)(nf.blocksize*(nf.Bps/8)))
			return int(written), err
		}
		return 0, errors.New("This shouldn't happen. Please report me")
	}

	// Compressed (NWA) wave data stream
	if nf.offsets == nil {
		return 0, errors.New("No offsets set")
	}
	if nf.blocks == nf.curblock {
		return 0, io.EOF
	}
	if nf.curblock == -1 {
		// If it's the first block we have to write the wave header
		nf.curblock++
		written, err := io.Copy(nf.outdata, makeWavHeader(nf.Datasize, nf.Channels, nf.Bps, nf.Freq))
		return int(written), err
	}

	// Calculate the size of the decoded block
	var curblocksize, curcompsize int
	if nf.curblock != nf.blocks-1 {
		curblocksize = nf.blocksize * (nf.Bps / 8)
		curcompsize = nf.offsets[nf.curblock+1] - nf.offsets[nf.curblock]
		if curblocksize >= nf.blocksize*(nf.Bps/8)*2 {
			return 0, errors.New("Calculated blocksize is too big")
		} // Fatal error
	} else {
		curblocksize = nf.restsize * (nf.Bps / 8)
		curcompsize = nf.blocksize * (nf.Bps / 8) * 2
	}

	// Read in the block data
	nf.tmpdata.Reset()
	io.CopyN(&nf.tmpdata, nf.reader, (int64)(curcompsize))

	// Decode the compressed block
	nf.decode(curblocksize)

	nf.curblock++
	return curblocksize, nil
}

// If you wanted to gain more speed, this would be the place to start. You
// could try to use an array or slice (not a ByteReader), so that you could
// speed up the bitReader.ReadBits() method.
func (nf *NwaFile) decode(outsize int) {
	d := [...]int{0, 0}
	var flipflag, runlength int = 0, 0

	// Read the first data (with full accuracy)
	if nf.Bps == 8 {
		var tmp uint8
		binary.Read(&nf.tmpdata, binary.LittleEndian, &tmp)
		d[0] = int(tmp)
	} else { // bps == 16bit
		var tmp uint16
		binary.Read(&nf.tmpdata, binary.LittleEndian, &tmp)
		d[0] = int(tmp)
	}
	// Stereo
	if nf.Channels == 2 {
		if nf.Bps == 8 {
			var tmp uint8
			binary.Read(&nf.tmpdata, binary.LittleEndian, &tmp)
			d[1] = int(tmp)
		} else { // bps == 16bit
			var tmp uint16
			binary.Read(&nf.tmpdata, binary.LittleEndian, &tmp)
			d[1] = int(tmp)
		}
	}

	br := newBitReader(&nf.tmpdata)
	dsize := outsize / (nf.Bps / 8)
	for i := 0; i < dsize; i++ {
		// If we are not in a copy loop (RLE), read in the data
		if runlength == 0 {
			exponent := br.ReadBits(3)
			// Branching according to the mantissa: 0, 1-6, 7
			switch {
			case exponent == 7:
				{
					// 7: big exponent
					// In case we are using RLE (complevel==5) this is disabled
					if br.ReadBits(1) == 1 {
						d[flipflag] = 0
					} else {
						var bits, shift uint
						if nf.complevel >= 3 {
							bits = 8
							shift = 9
						} else {
							bits = 8 - uint(nf.complevel)
							shift = 2 + 7 + uint(nf.complevel)
						}
						mask1 := uint(1 << (bits - 1))
						mask2 := uint((1 << (bits - 1)) - 1)
						b := br.ReadBits(bits)
						if b&mask1 != 0 {
							d[flipflag] -= int((b & mask2) << shift)
						} else {
							d[flipflag] += int((b & mask2) << shift)
						}
					}
				}
			case exponent != 0:
				{
					// 1-6 : normal differencial
					var bits, shift uint
					if nf.complevel >= 3 {
						bits = uint(nf.complevel) + 3
						shift = 1 + exponent
					} else {
						bits = 5 - uint(nf.complevel)
						shift = 2 + exponent + uint(nf.complevel)
					}
					mask1 := uint(1 << (bits - 1))
					mask2 := uint((1 << (bits - 1)) - 1)
					b := br.ReadBits(bits)
					if b&mask1 != 0 {
						d[flipflag] -= int((b & mask2) << shift)
					} else {
						d[flipflag] += int((b & mask2) << shift)
					}
				}
			case exponent == 0:
				{
					// Skips when not using RLE
					if nf.userunlength == 1 {
						runlength = int(br.ReadBits(1))
						if runlength == 1 {
							runlength = int(br.ReadBits(2))
							if runlength == 3 {
								runlength = int(br.ReadBits(8))
							}
						}
					}
				}
			}
		} else {
			runlength--
		}
		if nf.Bps == 8 {
			binary.Write(nf.outdata, binary.LittleEndian, uint8(d[flipflag]))
		} else {
			binary.Write(nf.outdata, binary.LittleEndian, int16(d[flipflag]))
		}
		if nf.Channels == 2 {
			// Changing the channel
			flipflag = flipflag ^ 1
		}
	}
}
