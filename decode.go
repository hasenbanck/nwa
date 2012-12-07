// Package nwa provides functions to decode RealLive NWA files. It works with
// all io.Reader compatible data sources.
package nwa

import (
	"io"
)

// Decode returns the dencoded sound as a slice in WAVE format.
func DecodeAsWav(r io.Reader) ([]byte, error) {
	data, err := NewNwaData(r)
	if err = data.ReadHeader(); err != nil {
		return nil, err
	}
	if err = data.CheckHeader(); err != nil {
		return nil, err
	}
	//var bs int
	//if bs, err = d.BlockLength(); err != nil {return nil, err}
	/**
	data := make([]byte, bs)
	while( (read, err = h.Decode(data, skip_count)); read != 0) {
		if (err != nil) {
			break
		} else {
			continue
		}
	}
	return data, nil
	**/

	return nil, nil
}

// DecoeKoeAsWav returns the decoded sound at the given offset/length
// from a KOE file as a slice in WAVE format.
func DecodeKoeAsWav(r io.Reader, offset int, length int) ([]byte, error) {
	// TODO: Could be NWA or VORBIS!
	return nil, nil
}
