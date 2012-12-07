// Package nwa provides functions to decode RealLive NWA files. It works with
// all io.Reader compatible data sources.
package nwa

import (
	"io"
)

// Decode returns the dencoded sound as a slice in WAVE format.
func DecodeAsWav(r io.Reader) ([]byte, error) {
	//var bs int
	var d nwaData
	var err error

	if err = d.OpenReader(r); err != nil {
		return nil, err
	}
	if err = d.ReadHeader(); err != nil {
		return nil, err
	}
	if err = d.CheckHeader(); err != nil {
		return nil, err
	}
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
