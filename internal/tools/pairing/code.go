// Package pairing generates the codes users exchange to connect as a couple.
package pairing

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

var codeSpace = big.NewInt(1_000_000)

// Generate returns a uniformly random 6-digit code.
func Generate() string {
	n, err := rand.Int(rand.Reader, codeSpace)
	if err != nil {
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}
	return fmt.Sprintf("%06d", n.Int64())
}
