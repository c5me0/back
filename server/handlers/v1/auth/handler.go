// Package auth handles phone sign-in and sign-out.
package auth

import (
	"regexp"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"cameo/internal/ent"
	"cameo/server/services/otp"
	"cameo/server/services/purchase"
	"cameo/server/services/session"
)

var phoneRule = validation.Match(regexp.MustCompile(`^\+[1-9]\d{7,14}$`)).Error("must be an E.164 phone number")

type Handler struct {
	db       *ent.Client
	session  *session.Service
	otp      otp.Provider
	purchase *purchase.Service
}

func NewHandler(db *ent.Client, session *session.Service, otp otp.Provider, purchase *purchase.Service) *Handler {
	return &Handler{db: db, session: session, otp: otp, purchase: purchase}
}
