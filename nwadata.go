package nwa

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type NwaData struct {
	reader       io.Reader // Reader to read the NWA data from
	channels     int       // channels
	bps          int       // bits per sample
	freq         int       // samples per second
	complevel    int       // compression level
	userunlength int       // run length encoding
	blocks       int       // block count
	datasize     int       // all data size
	compdatasize int       // compressed data size
	samplecount  int       // all samples
	blocksize    int       // samples per block
	restsize     int       // samples of the last block
	curblock     int
	offsets      []int

	tmpdata bytes.Buffer
}

func NewNwaData(r io.Reader) (*NwaData, error) {
	if r == nil {
		return nil, errors.New("Reader is nil.")
	}
	return &NwaData{reader: r}, nil
}

func (nd *NwaData) ReadHeader() error {
	nd.curblock = -1
	buffer := new(bytes.Buffer)
	if count, err := io.CopyN(buffer, nd.reader, 0x2c); count != 0x2c || err != nil {
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

	nd.channels = int(channels)
	nd.bps = int(bps)
	nd.freq = int(freq)
	nd.complevel = int(complevel)
	nd.userunlength = int(userunlength)
	nd.blocks = int(blocks)
	nd.datasize = int(datasize)
	nd.compdatasize = int(compdatasize)
	nd.samplecount = int(samplecount)
	nd.blocksize = int(blocksize)
	nd.restsize = int(restsize)

	// uncompressed wave
	if nd.complevel == -1 {
		nd.blocksize = 65536
		nd.restsize = (nd.datasize % (nd.blocksize * (nd.bps / 8))) / (nd.bps / 8)
		var rest int
		if nd.restsize > 0 {
			rest = 1
		}
		nd.blocks = nd.datasize/(nd.blocksize*(nd.bps/8)) + rest
	}
	if nd.blocks <= 0 || nd.blocks > 1000000 {
		// There can't be a music file over 1 hr
		return fmt.Errorf("Blocks are too large: %d\n", nd.blocks)
	}
	if nd.complevel == -1 {
		return nil
	}

	// Read the offset indexes
	nd.offsets = make([]int, nd.blocks)
	buffer = new(bytes.Buffer)
	offsetsize := int64(nd.blocks * 4)
	if count, err := io.CopyN(buffer, nd.reader, offsetsize); count != offsetsize || err != nil {
		if err == nil {
			err = fmt.Errorf("Can't read the offset block. Read %X bytes", count)
		}
		return err
	}

	var i int
	for i = 0; i < nd.blocks; i++ {
		binary.Read(buffer, binary.LittleEndian, &nd.offsets[i])
	}
	return nil
}

func (nd *NwaData) CheckHeader() error {
	if nd.complevel != -1 && nd.offsets == nil {
		return errors.New("No offsets set even thought they are needed")
	}
	if nd.channels != 1 && nd.channels != 2 {
		return fmt.Errorf("This library only supports mono / stereo data: data has %d channels\n", nd.channels)
	}
	if nd.bps != 8 && nd.bps != 16 {
		return fmt.Errorf("This library only supports 8 / 16bit data: data is %d bits\n", nd.bps)
	}
	if nd.complevel == -1 {
		var byps int = nd.bps / 8 // bytes per sample
		if nd.datasize != nd.samplecount*byps {
			return fmt.Errorf("Invalid datasize: datasize %d != samplecount %d * samplesize %d\n", nd.datasize, nd.samplecount, byps)
		}
		if nd.samplecount != (nd.blocks-1)*nd.blocksize+nd.restsize {
			return fmt.Errorf("Total sample count is invalid: samplecount %d != %d*%d+%d(block*blocksize+lastblocksize)\n", nd.samplecount, nd.blocks-1, nd.blocksize, nd.restsize)
		}
		return nil
	}
	if nd.complevel < 0 || nd.complevel > 5 {
		return fmt.Errorf("This library supports only compression level from -1 to 5: the compression level of the data is %d\n", nd.complevel)
	}
	if nd.offsets[nd.blocks-1] >= nd.compdatasize {
		return fmt.Errorf("The last offset overruns the file.\n")
	}
	var byps int = nd.bps / 8 // bytes per sample
	if nd.datasize != nd.samplecount*byps {
		return fmt.Errorf("Invalid datasize: datasize %d != samplecount %d * samplesize %d\n", nd.datasize, nd.samplecount, byps)
	}
	if nd.samplecount != (nd.blocks-1)*nd.blocksize+nd.restsize {
		return fmt.Errorf("Total sample count is invalid: samplecount %d != %d*%d+%d(block*blocksize+lastblocksize).\n", nd.samplecount, nd.blocks-1, nd.blocksize, nd.restsize)
	}
	return nil
}

func (nd *NwaData) BlockLength() (int, error) {
	if nd.complevel != -1 {
		if nd.offsets == nil {
			return 0, errors.New("BlockLength could not be calculcated: No offsets set!")
		}
	}
	return nd.blocksize * (nd.bps / 8), nil
}

/*
**data は BlockLength 以上の長さを持つこと
** 返り値は作成したデータの長さ。終了時は 0。
** エラー時は -1
 */
func (nd *NwaData) Decode(writer io.Writer) int64 {
	// Uncompressed wave data stream
	if nd.complevel == -1 {
		if nd.curblock == -1 {
			// If it's the first block we have to write the wave header
			written, _ := io.Copy(writer, makeWavHeader(nd.datasize, nd.channels, nd.bps, nd.freq))
			nd.curblock++
			return written
		}
		if nd.curblock < nd.blocks {
			nd.curblock++
			ret, err := io.CopyN(writer, nd.reader, (int64)(nd.blocksize*(nd.bps/8)))
			if err != nil {
				return -1
			}
			return ret
		}
		return -1
	}

	// Compressed (NWA) wave data stream
	if nd.offsets == nil {
		return -1
	}
	if nd.blocks == nd.curblock {
		return 0
	}
	if nd.curblock == -1 {
		// If it's the first block we have to write the wave header
		written, _ := io.Copy(writer, makeWavHeader(nd.datasize, nd.channels, nd.bps, nd.freq))
		nd.curblock++
		return written
	}

	// 今回読み込む／デコードするデータの大きさを得る
	var curblocksize, curcompsize int
	if nd.curblock != nd.blocks-1 {
		curblocksize = nd.blocksize * (nd.bps / 8)
		curcompsize = nd.offsets[nd.curblock+1] - nd.offsets[nd.curblock]
		if curblocksize >= nd.blocksize*(nd.bps/8)*2 {
			return -1
		} // Fatal error
	} else {
		curblocksize = nd.restsize * (nd.bps / 8)
		curcompsize = nd.blocksize * (nd.bps / 8) * 2
	}

	// Read in the block data
	nd.tmpdata.Reset()
	io.CopyN(&nd.tmpdata, nd.reader, (int64)(curcompsize))

	// Decode the block
	nd.decodeBlock(writer)

	nd.curblock++
	return (int64)(curblocksize)
}

func (nd *NwaData) decodeBlock(writer io.Writer) {
	var d [2]int
	var flipflag, runlength int = 0, 0

	// Read the first data (with full accuracy)
	if nd.bps == 8 {
		var tmp int8
		binary.Read(&nd.tmpdata, binary.LittleEndian, &tmp)
		d[0] = int(tmp)
	} else { // bps == 16bit
		binary.Read(&nd.tmpdata, binary.LittleEndian, &d[0])
	}
	// Stereo
	if nd.channels == 2 {
		if nd.bps == 8 {
			var tmp int8
			binary.Read(&nd.tmpdata, binary.LittleEndian, &tmp)
			d[1] = int(tmp)
		} else { // bps == 16bit
			binary.Read(&nd.tmpdata, binary.LittleEndian, &d[1])
		}
	}

	br := newBitReader(&nd.tmpdata)
	for true {
		if br.CheckError() != nil {
			break
		}

		// If we are not in a copy loop (RLE), read in the data
		if runlength == 0 {
			mantissa := br.ReadBits(3)
			// Branching according to the mantissa: 0, 1-6, 7
			switch {
			case mantissa == 7:
				{
					// 7: big exponent
					// In case we are using RLE (complevel==5) this is disabled
					if br.ReadBits(1) == 1 {
						d[flipflag] = 0
					} else {
						var bits, shift uint
						if nd.complevel >= 3 {
							bits = 8
							shift = 9
						} else {
							bits = 8 - uint(nd.complevel)
							shift = 2 + 7 + uint(nd.complevel)
						}
						mask1 := uint(1 << (bits - 1))
						mask2 := uint((1 << (bits - 1)) - 1)
						b := br.ReadBits(bits)
						if b&mask1 == 1 {
							d[flipflag] -= int((b & mask2) << shift)
						} else {
							d[flipflag] += int((b & mask2) << shift)
						}
					}
				}
			case mantissa != 0:
				{
					// 1-6 : normal differencial
					var bits, shift uint
					if nd.complevel >= 3 {
						bits = uint(nd.complevel) + 3
						shift = 1 + mantissa
					} else {
						bits = 5 - uint(nd.complevel)
						shift = 2 + mantissa + uint(nd.complevel)
					}
					mask1 := uint(1 << (bits - 1))
					mask2 := uint((1 << (bits - 1)) - 1)
					b := br.ReadBits(bits)
					if b&mask1 == 1 {
						d[flipflag] -= int((b & mask2) << shift)
					} else {
						d[flipflag] += int((b & mask2) << shift)
					}
				}
			case mantissa == 0:
				{
					// Skips when not using RLE
					if nd.userunlength == 1 {
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
		if nd.bps == 8 {
			if br.CheckError() != nil {
				break
			}
			// TODO write into the writer 8bit!
			// *outdata++ = d[flipflag]
		} else {
			if br.CheckError() != nil {
				break
			}
			// TODO write into the writer 16bit!
			//write_little_endian_short(outdata, d[flipflag])
			//outdata += 2
		}
		if nd.channels == 2 {
			// changing the channel
			flipflag = flipflag ^ 1
		}
	}
}
