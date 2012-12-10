// Package nwa provides functions to decode RealLive NWA files. It works with
// all io.Reader compatible data sources.
package nwa

import (
	"bytes"
	"errors"
	"io"
)

// Decode returns the dencoded sound as a slice in WAVE format.
func DecodeAsWav(r io.Reader) (io.Reader, error) {
	nwadata, err := NewNwaData(r)
	if err != nil {
		return nil, err
	}
	if err = nwadata.ReadHeader(); err != nil {
		return nil, err
	}
	if err = nwadata.CheckHeader(); err != nil {
		return nil, err
	}

	var ret int64 = -1
	data := new(bytes.Buffer)
	for ret != 0 {
		ret = nwadata.DecodeBlock(data)
		if ret == -1 {
			return nil, errors.New("This shouldn't happen! Report me!")
		}
	}
	return data, nil
}
