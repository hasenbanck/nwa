// Package nwa provides functions to decode RealLive NWA files. It works with
// all io.Reader compatible data sources.
package nwa

import (
	"io"
)

// Decode returns the decoded sound in WAVE format as an io.Reader.
func DecodeAsWav(r io.Reader) (io.Reader, error) {
	nwadata, err := NewNwaFile(r)
	if err != nil {
		return nil, err
	}
	return nwadata, nil
}
