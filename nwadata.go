package nwa

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type NwaData struct {
	reader       io.ReadSeeker // Reader to read the NWA data from
	channels     int           // channels
	bps          int           // bits per sample
	freq         int           // samples per second
	complevel    int           // compression level
	userunlength int           // run length encoding
	blocks       int           // block count
	datasize     int           // all data size
	compdatasize int           // compressed data size
	samplecount  int           // all samples
	blocksize    int           // samples per block
	restsize     int           // samples of the last block
	curblock     int
	offsets      []int
	tmpdata      bytes.Buffer
}

func NewNwaData(r io.ReadSeeker) (*NwaData, error) {
	if r == nil {
		return nil, errors.New("Reader is nil.")
	}
	return &NwaData{reader: r}, nil
}

func (d *NwaData) ReadHeader() error {
	d.curblock = -1
	buffer := new(bytes.Buffer)
	if count, err := io.CopyN(buffer, d.reader, 0x2c); count != 0x2c || err != nil {
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

	d.channels = int(channels)
	d.bps = int(bps)
	d.freq = int(freq)
	d.complevel = int(complevel)
	d.userunlength = int(userunlength)
	d.blocks = int(blocks)
	d.datasize = int(datasize)
	d.compdatasize = int(compdatasize)
	d.samplecount = int(samplecount)
	d.blocksize = int(blocksize)
	d.restsize = int(restsize)

	if d.complevel == -1 { // 無圧縮rawデータ
		// 適当に決め打ちする
		d.blocksize = 65536
		d.restsize = (d.datasize % (d.blocksize * (d.bps / 8))) / (d.bps / 8)
		var rest int
		if d.restsize > 0 {
			rest = 1
		}
		d.blocks = d.datasize/(d.blocksize*(d.bps/8)) + rest
	}
	if d.blocks <= 0 || d.blocks > 1000000 {
		// １時間を超える曲ってのはないでしょ
		return fmt.Errorf("Blocks are too large: %d\n", d.blocks)
	}
	if d.complevel == -1 {
		return nil
	}

	// Read the offset indexes
	// TODO: Test me!
	d.offsets = make([]int, d.blocks)
	buffer = new(bytes.Buffer)
	offsetsize := int64(d.blocks * 4)
	if count, err := io.CopyN(buffer, d.reader, offsetsize); count != offsetsize || err != nil {
		if err == nil {
			err = fmt.Errorf("Can't read the offset block. Read %X bytes", count)
		}
		return err
	}

	var i int
	for i = 0; i < d.blocks; i++ {
		binary.Read(buffer, binary.LittleEndian, &d.offsets[i])
	}
	return nil
}

func (d *NwaData) CheckHeader() error {
	if d.complevel != -1 && d.offsets == nil {
		return errors.New("No offsets set even thought they are needed")
	}
	if d.channels != 1 && d.channels != 2 {
		return fmt.Errorf("This library only supports mono / stereo data: data has %d channels\n", d.channels)
	}
	if d.bps != 8 && d.bps != 16 {
		return fmt.Errorf("This library only supports 8 / 16bit data: data is %d bits\n", d.bps)
	}
	if d.complevel == -1 {
		var byps int = d.bps / 8 // bytes per sample
		if d.datasize != d.samplecount*byps {
			return fmt.Errorf("Invalid datasize: datasize %d != samplecount %d * samplesize %d\n", d.datasize, d.samplecount, byps)
		}
		if d.samplecount != (d.blocks-1)*d.blocksize+d.restsize {
			return fmt.Errorf("Total sample count is invalid: samplecount %d != %d*%d+%d(block*blocksize+lastblocksize)\n", d.samplecount, d.blocks-1, d.blocksize, d.restsize)
		}
		return nil
	}
	if d.complevel < 0 || d.complevel > 5 {
		return fmt.Errorf("This library supports only compression level from -1 to 5: the compression level of the data is %d\n", d.complevel)
	}
	if d.offsets[d.blocks-1] >= d.compdatasize {
		return fmt.Errorf("The last offset overruns the file.\n")
	}
	var byps int = d.bps / 8 // bytes per sample
	if d.datasize != d.samplecount*byps {
		return fmt.Errorf("Invalid datasize: datasize %d != samplecount %d * samplesize %d\n", d.datasize, d.samplecount, byps)
	}
	if d.samplecount != (d.blocks-1)*d.blocksize+d.restsize {
		return fmt.Errorf("Total sample count is invalid: samplecount %d != %d*%d+%d(block*blocksize+lastblocksize).\n", d.samplecount, d.blocks-1, d.blocksize, d.restsize)
	}
	return nil
}

func (d *NwaData) BlockLength() (int, error) {
	if d.complevel != -1 {
		if d.offsets == nil {
			return 0, errors.New("BlockLength could not be calculcated: No offsets set!")
		}
	}
	return d.blocksize * (d.bps / 8), nil
}

/*
**data は BlockLength 以上の長さを持つこと
** 返り値は作成したデータの長さ。終了時は 0。
** エラー時は -1
 */
func (d *NwaData) Decode(data io.Writer, skipcount int) int64 {
	// Uncompressed wave data stream
	if d.complevel == -1 {
		if d.curblock == -1 {
			// If it's the first block we have to write the wave header
			written, _ := io.Copy(data, makeWavHeader(d.datasize, d.channels, d.bps, d.freq))
			d.curblock++
			// TODO: We might need to seek to the offset+0x2c, not really sure right now
			// Since we have allready read a header and should be in the data segment right
			// now with the cursor
			//d.reader.Seek(offset_start+0x2c)
			return written
		}
		if skipcount > d.blocksize/d.channels {
			skipcount -= d.blocksize / d.channels
			d.reader.Seek((int64)(d.blocksize*(d.bps/8)), 1)
			d.curblock++
			return -2
		}
		if d.curblock < d.blocks {
			readsize := d.blocksize
			if skipcount != 0 {
				d.reader.Seek((int64)(skipcount*d.channels*(d.bps/8)), 1)
				readsize -= skipcount * d.channels
				skipcount = 0
			}
			d.curblock++
			ret, err := io.CopyN(data, d.reader, (int64)(readsize*(d.bps/8)))
			if err != nil {
				return -1
			}
			return ret
		}
		return -1
	}

	// Compressed (NWA) wave data stream
	if d.offsets == nil {
		return -1
	}
	if d.blocks == d.curblock {
		return 0
	}
	if d.curblock == -1 {
		// If it's the first block we have to write the wave header
		written, _ := io.Copy(data, makeWavHeader(d.datasize, d.channels, d.bps, d.freq))
		d.curblock++
		return written
	}

	// 今回読み込む／デコードするデータの大きさを得る
	var curblocksize, curcompsize int
	if d.curblock != d.blocks-1 {
		curblocksize = d.blocksize * (d.bps / 8)
		curcompsize = d.offsets[d.curblock+1] - d.offsets[d.curblock]
		if curblocksize >= d.blocksize*(d.bps/8)*2 {
			return -1
		} // Fatal error
	} else {
		curblocksize = d.restsize * (d.bps / 8)
		curcompsize = d.blocksize * (d.bps / 8) * 2
	}
	if skipcount > d.blocksize/d.channels {
		skipcount -= d.blocksize / d.channels
		d.reader.Seek((int64)(curcompsize), 1)
		d.curblock++
		return -2
	}

	// データ読み込み
	d.tmpdata.Reset()
	io.CopyN(&d.tmpdata, d.reader, (int64)(curcompsize))

	// 展開
	//TODO: Implement me
	//decodeData(d.tmpdata, d.reader, curcompsize, curblocksize)
	fmt.Printf("Curblock: %d", d.curblock)

	retsize := curblocksize
	if skipcount != 0 {
		skip := skipcount * d.channels * (d.bps / 8)
		retsize -= skip
		// TODO: Why do we need this skipping?
		// The next command worries me a little bit
		//memmove(data, data+skipc, skip)
		skipcount = 0
	}
	d.curblock++
	return (int64)(retsize)
}
