package auth

import (
	"context"

	"runner-manager/params"
)

type contextFlags string

const (
	isAdminKey  contextFlags = "is_admin"
	fullNameKey contextFlags = "full_name"
	// UserIDFlag is the User ID flag we set in the context
	UserIDFlag    contextFlags = "user_id"
	isEnabledFlag contextFlags = "is_enabled"
	jwtTokenFlag  contextFlags = "jwt_token"
)

// PopulateContext sets the appropriate fields in the context, based on
// the user object
func PopulateContext(ctx context.Context, user params.User) context.Context {
	ctx = SetUserID(ctx, user.ID)
	ctx = SetAdmin(ctx, user.IsAdmin)
	ctx = SetIsEnabled(ctx, user.Enabled)
	ctx = SetFullName(ctx, user.FullName)
	return ctx
}

// SetFullName sets the user full name in the context
func SetFullName(ctx context.Context, fullName string) context.Context {
	return context.WithValue(ctx, fullNameKey, fullName)
}

// FullName returns the full name from context
func FullName(ctx context.Context) string {
	name := ctx.Value(fullNameKey)
	if name == nil {
		return ""
	}
	return name.(string)
}

// SetJWTClaim will set the JWT claim in the context
func SetJWTClaim(ctx context.Context, claim JWTClaims) context.Context {
	return context.WithValue(ctx, jwtTokenFlag, claim)
}

// JWTClaim returns the JWT claim saved in the context
func JWTClaim(ctx context.Context) JWTClaims {
	jwtClaim := ctx.Value(jwtTokenFlag)
	if jwtClaim == nil {
		return JWTClaims{}
	}
	return jwtClaim.(JWTClaims)
}

// SetIsEnabled sets a flag indicating if account is enabled
func SetIsEnabled(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, isEnabledFlag, enabled)
}

// IsEnabled returns the a boolean indicating if the enabled flag is
// set and is true or false
func IsEnabled(ctx context.Context) bool {
	elem := ctx.Value(isEnabledFlag)
	if elem == nil {
		return false
	}
	return elem.(bool)
}

// SetAdmin sets the isAdmin flag on the context
func SetAdmin(ctx context.Context, isAdmin bool) context.Context {
	return context.WithValue(ctx, isAdminKey, isAdmin)
}

// IsAdmin returns a boolean indicating whether
// or not the context belongs to a logged in user
// and if that context has the admin flag set
func IsAdmin(ctx context.Context) bool {
	elem := ctx.Value(isAdminKey)
	if elem == nil {
		return false
	}
	return elem.(bool)
}

// SetUserID sets the userID in the context
func SetUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, UserIDFlag, userID)
}

// UserID returns the userID from the context
func UserID(ctx context.Context) string {
	userID := ctx.Value(UserIDFlag)
	if userID == nil {
		return ""
	}
	return userID.(string)
}

// GetAdminContext will return an admin context. This can be used internally
// when fetching users.
func GetAdminContext() context.Context {
	ctx := context.Background()
	ctx = SetUserID(ctx, "")
	ctx = SetAdmin(ctx, true)
	ctx = SetIsEnabled(ctx, true)
	return ctx
}
