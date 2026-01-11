package crypto

import "io"

// SetRandReaderForTesting sets the random reader used by GenerateKeypair.
// This is intended for testing only. Returns a function to restore the original reader.
// Since this package is internal, this function cannot be accessed by external code.
func SetRandReaderForTesting(r io.Reader) func() {
	original := randReader
	randReader = r
	return func() { randReader = original }
}
