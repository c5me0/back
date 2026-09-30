package session

import (
	"time"

	"github.com/DeltaLaboratory/fwt/v2"

	"cameo/internal/ent"
)

const sessionValidity = time.Hour * 24 * 90

type Service struct {
	db       *ent.Client
	signer   *fwt.Signer
	verifier *fwt.Verifier
}

func NewService(db *ent.Client, signer *fwt.Signer, verifier *fwt.Verifier) *Service {
	return &Service{
		db:       db,
		signer:   signer,
		verifier: verifier,
	}
}
