// Package nwa provides functions to decode RealLive NWA files. It works with
// all io.Reader compatible data sources.
package nwa

import (
	"bytes"
	"io"
)

// Decode returns the decoded sound in WAVE format as a io.Reader.
func DecodeAsWav(r io.Reader) (io.Reader, error) {
	var err error
	var nwafile *NwaFile
	if nwafile, err = NewNwaFile(r); err != nil {
		return nil, err
	}
	outdata := new(bytes.Buffer)
	if _, err = io.Copy(outdata, nwafile); err != nil {
		return nil, err
	}
	return outdata, nil
}
