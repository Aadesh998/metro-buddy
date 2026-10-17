package httpx

import (
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type Principal struct {
	UserID    uuid.UUID
	PersonaID uuid.UUID
	TrustTier int
	// Restricted is set when this session's device failed app/device attestation.
	Restricted bool
}

const principalKey = "principal"

func SetPrincipal(c echo.Context, p *Principal) { c.Set(principalKey, p) }

func PrincipalFrom(c echo.Context) *Principal {
	p, _ := c.Get(principalKey).(*Principal)
	return p
}
