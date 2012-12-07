package nwa

import (
	"errors"
	"io"
)

type nwaData struct {
	Channels int
	Bps      int // bits per sample
	Freq     int // samples per second
	Blocks   int // block count
	Datasize int // all data size

	reader       io.Reader // Reader to read the NWA data from
	complevel    int       // compression level
	use_unlength int       // run length encoding
	compdatasize int       // compressed data size
	samplecount  int       // all samples
	blocksize    int       // samples per block
	restsize     int       // samples of the last block
	dummy2       int       // ? : 0x89
	curblock     int
	offsets      []int
	offset_start int
	filesize     int
	tmpdata      []byte
}

func (d *nwaData) OpenReader(r io.Reader) error {
	if r == nil {
		return errors.New("io: Reader is nil.")
	}
	d.reader = r
	return nil
}

func (d *nwaData) ReadHeader() error {
	return errors.New("Not implemented yet")
}

func (d *nwaData) CheckHeader() error {
	return errors.New("Not implemented yet")
}

func (d *nwaData) BlockLength() (int, error) {
	if d.complevel != -1 {
		if d.offsets == nil {
			return 0, errors.New("BlockLength could not be calculcated: No offsets set!")
		}
		if d.tmpdata == nil {
			return 0, errors.New("BlockLength could not be calculcated: No data slice set!")
		}
	}
	return d.blocksize * (d.Bps / 8), nil
}

/* Original comment of the function:
**data は BlockLength 以上の長さを持つこと
** 返り値は作成したデータの長さ。終了時は 0。
** エラー時は -1
 */
func (d *nwaData) Decode(data []byte, skipcount int) (int, error) {
	return 0, errors.New("Not implemented yet")
}
