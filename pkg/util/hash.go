package util

import (
	"crypto/sha256"
	"encoding/base32"
)

// oneWayEncoding can be used to encode hex to a 62-character set (0 and 1 are duplicates) for use in
// short display names that are safe for use in kubernetes as resource names.
var oneWayNameEncoding = base32.NewEncoding("bcdfghijklmnpqrstvwxyz0123456789").WithPadding(base32.NoPadding)

// InputHash returns a string that hashes the unique parts of the input to avoid collisions.
func InputHash(inputs ...[]byte) string {
	hash := sha256.New()

	// the inputs form a part of the hash
	for _, s := range inputs {
		hash.Write(s)
	}

	// Object names can't be too long so we truncate the hash.
	return oneWayNameEncoding.EncodeToString(hash.Sum(nil)[:10])
}
