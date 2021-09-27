package keycloak

import (
	"github.com/gin-gonic/gin"
)

func LoggedInCheck() func(tc *TokenContainer, ctx *gin.Context) bool {
	return func(tc *TokenContainer, ctx *gin.Context) bool {
		ctx.Set("token", *tc.KeyCloakToken)
		ctx.Set("uid", tc.KeyCloakToken.PreferredUsername)
		uid := tc.KeyCloakToken.PreferredUsername
		if len(uid) > 0 {
			return true
		}
		return false
	}
}
