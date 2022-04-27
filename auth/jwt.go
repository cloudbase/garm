package auth

import (
	"runner-manager/params"
	"runner-manager/runner/common"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/pkg/errors"
)

// InstanceJWTClaims holds JWT claims
type InstanceJWTClaims struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	PoolID string `json:"provider_id"`
	// Scope is either repository or organization
	Scope common.PoolType `json:"scope"`
	// Entity is the repo or org name
	Entity string `json:"entity"`
	jwt.StandardClaims
}

func NewInstanceJWTToken(instance params.Instance, secret, entity string, poolType common.PoolType) (string, error) {
	// make TTL configurable?
	expireToken := time.Now().Add(3 * time.Hour).Unix()
	claims := InstanceJWTClaims{
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expireToken,
			Issuer:    "runner-manager",
		},
		ID:     instance.ID,
		Name:   instance.Name,
		PoolID: instance.PoolID,
		Scope:  poolType,
		Entity: entity,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		return "", errors.Wrap(err, "signing token")
	}

	return tokenString, nil
}
