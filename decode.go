// Package nwa provides functions to decode RealLive NWA files. It works with
// all io.Reader compatible data sources.
package nwa

import "io"

// Decode returns the dencoded sound as a slice in WAVE format.
func DecodeAsWav(r io.Reader)  (buffer []byte, err error) {
	return nil, nil
}

// DecoeKoeAsWav returns the decoded sound at the given offset/length
// from a KOE file as a slice in WAVE format.
func DecodeKoeAsWav(r io.Reader, offset int, length int) (buffer []byte, err error) {
	return nil, nil
}
