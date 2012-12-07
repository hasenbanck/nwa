package nwa

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type NwaData struct {
	reader        io.Reader // Reader to read the NWA data from
	channels      int16     // channels
	bps           int16     // bits per sample
	freq          int32     // samples per second
	blocks        int32     // block count
	datasize      int32     // all data size
	complevel     int32     // compression level
	use_runlength int32     // run length encoding
	compdatasize  int32     // compressed data size
	samplecount   int32     // all samples
	blocksize     int32     // samples per block
	restsize      int32     // samples of the last block
	dummy2        int32     // ? : 0x89
	curblock      int
	offsets       []int32
	filesize      int // TODO: Do we need this?
	tmpdata       []byte
}

func NewNwaData(r io.Reader) (*NwaData, error) {
	if r == nil {
		return nil, errors.New("Reader is nil.")
	}
	return &NwaData{reader: r}, nil
}

func (d *NwaData) ReadHeader() error {
	header := make([]byte, 0x2c)
	d.curblock = -1

	if count, err := d.reader.Read(header); count != 0x2c || err != nil {
		if err == nil {
			err = errors.New("Can't read the header")
		}
		return err
	}

	buffer := bytes.NewBuffer(header)
	binary.Read(buffer, binary.LittleEndian, &d.channels)
	binary.Read(buffer, binary.LittleEndian, &d.bps)
	binary.Read(buffer, binary.LittleEndian, &d.freq)
	binary.Read(buffer, binary.LittleEndian, &d.complevel)
	binary.Read(buffer, binary.LittleEndian, &d.use_runlength)
	binary.Read(buffer, binary.LittleEndian, &d.blocks)
	binary.Read(buffer, binary.LittleEndian, &d.datasize)
	binary.Read(buffer, binary.LittleEndian, &d.compdatasize)
	binary.Read(buffer, binary.LittleEndian, &d.samplecount)
	binary.Read(buffer, binary.LittleEndian, &d.blocksize)
	binary.Read(buffer, binary.LittleEndian, &d.restsize)
	binary.Read(buffer, binary.LittleEndian, &d.dummy2)

	if d.complevel == -1 { // 無圧縮rawデータ
		// 適当に決め打ちする
		d.blocksize = 65536
		d.restsize = (d.datasize % (d.blocksize * (int32)(d.bps/8))) / (int32)(d.bps/8)
		// Todo: Can we make the following 3 lines better to read?
		var rest int32
		if d.restsize > 0 {
			rest = 1
		}
		d.blocks = d.datasize/(d.blocksize*(int32)(d.bps/8)) + rest
	}
	if d.blocks <= 0 || d.blocks > 1000000 {
		// １時間を超える曲ってのはないでしょ
		return fmt.Errorf("Blocks are too large: %d\n", d.blocks)
	}

	// TODO: Do we need the filesize? io.Reader doesn't provide such a thing

	if d.complevel == -1 {
		return nil
	}

	// Read the offset indexes
	// TODO: Test me!
	d.offsets = make([]int32, d.blocks)
	offsetData := make([]byte, d.blocks*4)
	if count, err := d.reader.Read(offsetData); count != (int)(d.blocks*4) || err != nil {
		if err == nil {
			err = errors.New("Can't read the offset block'")
		}
		return err
	}
	buffer = bytes.NewBuffer(offsetData)
	var i int32
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
		var byps int32 = (int32)(d.bps / 8) // bytes per sample
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

	// TODO: If we NEED the filesize, check it here!

	if d.offsets[d.blocks-1] >= d.compdatasize {
		return fmt.Errorf("The last offset overruns the file.\n")
	}
	var byps int32 = (int32)(d.bps / 8) // bytes per sample
	if d.datasize != d.samplecount*byps {
		return fmt.Errorf("Invalid datasize: datasize %d != samplecount %d * samplesize %d\n", d.datasize, d.samplecount, byps)
	}
	if d.samplecount != (d.blocks-1)*d.blocksize+d.restsize {
		return fmt.Errorf("Total sample count is invalid: samplecount %d != %d*%d+%d(block*blocksize+lastblocksize).\n", d.samplecount, d.blocks-1, d.blocksize, d.restsize)
	}
	d.tmpdata = make([]byte, d.blocksize*byps*2) // これ以上の大きさはないだろう、、、
	return nil
}

func (d *NwaData) BlockLength() (int32, error) {
	if d.complevel != -1 {
		if d.offsets == nil {
			return 0, errors.New("BlockLength could not be calculcated: No offsets set!")
		}
		if d.tmpdata == nil {
			return 0, errors.New("BlockLength could not be calculcated: No data slice set!")
		}
	}
	return d.blocksize * (int32)(d.bps/8), nil
}

/* Original comment of the function:
**data は BlockLength 以上の長さを持つこと
** 返り値は作成したデータの長さ。終了時は 0。
** エラー時は -1
 */
func (d *NwaData) Decode(data []byte, skipcount int) (int, error) {
	return 0, errors.New("Not implemented yet")
}
