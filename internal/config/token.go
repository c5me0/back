package config

import (
	"encoding/hex"
	"fmt"

	"github.com/DeltaLaboratory/fwt/v2"
	"github.com/cloudflare/circl/sign/ed25519"
	"github.com/cloudflare/circl/sign/ed448"
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

type TokenAlgorithm string

const (
	TokenAlgorithmEd25519    TokenAlgorithm = "ed25519"
	TokenAlgorithmEd448      TokenAlgorithm = "ed448"
	TokenAlgorithmHMACSHA256 TokenAlgorithm = "hmac-sha256"
	TokenAlgorithmHMACSHA512 TokenAlgorithm = "hmac-sha512"
	TokenAlgorithmBLAKE2b256 TokenAlgorithm = "blake2b-256"
	TokenAlgorithmBLAKE2b512 TokenAlgorithm = "blake2b-512"
	TokenAlgorithmBLAKE3     TokenAlgorithm = "blake3"
)

func (v TokenAlgorithm) Validate() error {
	return validation.Validate(v, validation.Required, validation.In(
		TokenAlgorithmEd25519, TokenAlgorithmEd448, TokenAlgorithmHMACSHA256,
		TokenAlgorithmHMACSHA512, TokenAlgorithmBLAKE2b256, TokenAlgorithmBLAKE2b512, TokenAlgorithmBLAKE3,
	), validation.Skip)
}

type Token struct {
	// Secret is the hex encoded secret key used to sign the FWT.
	Secret string `json:"secret"`

	// Algorithm is the algorithm used to sign the FWT.
	Algorithm TokenAlgorithm `json:"algorithm"`
}

func (t Token) Validate() error {
	return validation.ValidateStruct(&t,
		validation.Field(&t.Secret, validation.Required),
		validation.Field(&t.Algorithm),
	)
}

func (t Token) Signer() (*fwt.Signer, error) {
	decodedSecret, err := hex.DecodeString(t.Secret)
	if err != nil {
		return nil, fmt.Errorf("failed to decode secret: %w", err)
	}

	switch t.Algorithm {
	case TokenAlgorithmEd25519:
		if len(decodedSecret) != ed25519.SeedSize {
			return nil, fmt.Errorf("invalid secret size for ed25519: %d", len(decodedSecret))
		}

		return fwt.NewSigner(fwt.NewEd25519Signer(decodedSecret))
	case TokenAlgorithmEd448:
		if len(decodedSecret) != ed448.SeedSize {
			return nil, fmt.Errorf("invalid secret size for ed448: %d", len(decodedSecret))
		}

		return fwt.NewSigner(fwt.NewEd448Signer(decodedSecret))
	case TokenAlgorithmHMACSHA256:
		return fwt.NewSigner(fwt.NewHMACSha256Signer(decodedSecret))
	case TokenAlgorithmHMACSHA512:
		return fwt.NewSigner(fwt.NewHMACSha512Signer(decodedSecret))
	case TokenAlgorithmBLAKE2b256:
		return fwt.NewSigner(fwt.NewBlake2b256Signer(decodedSecret))
	case TokenAlgorithmBLAKE2b512:
		return fwt.NewSigner(fwt.NewBlake2b512Signer(decodedSecret))
	case TokenAlgorithmBLAKE3:
		return fwt.NewSigner(fwt.NewBlake3Signer(decodedSecret))
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", t.Algorithm)
	}
}

func (t Token) Verifier() (*fwt.Verifier, error) {
	decodedSecret, err := hex.DecodeString(t.Secret)
	if err != nil {
		return nil, fmt.Errorf("failed to decode secret: %w", err)
	}

	switch t.Algorithm {
	case TokenAlgorithmEd25519:
		if len(decodedSecret) != ed25519.SeedSize {
			return nil, fmt.Errorf("invalid secret size for ed25519: %d", len(decodedSecret))
		}

		key := ed25519.NewKeyFromSeed(decodedSecret)
		//nolint:errcheck // always this type
		return fwt.NewVerifier(fwt.NewEd25519Verifier(key.Public().(ed25519.PublicKey)))
	case TokenAlgorithmEd448:
		if len(decodedSecret) != ed448.SeedSize {
			return nil, fmt.Errorf("invalid secret size for ed448: %d", len(decodedSecret))
		}

		key := ed448.NewKeyFromSeed(decodedSecret)
		//nolint:errcheck // always this type
		return fwt.NewVerifier(fwt.NewEd448Verifier(key.Public().(ed448.PublicKey)))
	case TokenAlgorithmHMACSHA256:
		return fwt.NewVerifier(fwt.NewHMACSha256Verifier(decodedSecret))
	case TokenAlgorithmHMACSHA512:
		return fwt.NewVerifier(fwt.NewHMACSha512Verifier(decodedSecret))
	case TokenAlgorithmBLAKE2b256:
		return fwt.NewVerifier(fwt.NewBlake2b256Verifier(decodedSecret))
	case TokenAlgorithmBLAKE2b512:
		return fwt.NewVerifier(fwt.NewBlake2b512Verifier(decodedSecret))
	case TokenAlgorithmBLAKE3:
		return fwt.NewVerifier(fwt.NewBlake3Verifier(decodedSecret))
	default:
		return nil, fmt.Errorf("unsupported algorithm: %s", t.Algorithm)
	}
}
