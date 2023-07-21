// Copyright 2022 Cloudbase Solutions SRL
//
//    Licensed under the Apache License, Version 2.0 (the "License"); you may
//    not use this file except in compliance with the License. You may obtain
//    a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
//    WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
//    License for the specific language governing permissions and limitations
//    under the License.

package auth

import (
	"context"

	"github.com/cloudbase/garm/params"
)

type contextFlags string

const (
	isAdminKey     contextFlags = "is_admin"
	fullNameKey    contextFlags = "full_name"
	readMetricsKey contextFlags = "read_metrics"
	// UserIDFlag is the User ID flag we set in the context
	UserIDFlag    contextFlags = "user_id"
	isEnabledFlag contextFlags = "is_enabled"
	jwtTokenFlag  contextFlags = "jwt_token"

	instanceIDKey        contextFlags = "id"
	instanceNameKey      contextFlags = "name"
	instancePoolIDKey    contextFlags = "pool_id"
	instancePoolTypeKey  contextFlags = "scope"
	instanceEntityKey    contextFlags = "entity"
	instanceRunnerStatus contextFlags = "status"
	instanceTokenFetched contextFlags = "tokenFetched"
)

func SetInstanceID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, instanceIDKey, id)
}

func InstanceID(ctx context.Context) string {
	elem := ctx.Value(instanceIDKey)
	if elem == nil {
		return ""
	}
	return elem.(string)
}

func SetInstanceTokenFetched(ctx context.Context, fetched bool) context.Context {
	return context.WithValue(ctx, instanceTokenFetched, fetched)
}

func InstanceTokenFetched(ctx context.Context) bool {
	elem := ctx.Value(instanceTokenFetched)
	if elem == nil {
		return false
	}
	return elem.(bool)
}

func SetInstanceRunnerStatus(ctx context.Context, val params.RunnerStatus) context.Context {
	return context.WithValue(ctx, instanceRunnerStatus, val)
}

func InstanceRunnerStatus(ctx context.Context) params.RunnerStatus {
	elem := ctx.Value(instanceRunnerStatus)
	if elem == nil {
		return params.RunnerPending
	}
	return elem.(params.RunnerStatus)
}

func SetInstanceName(ctx context.Context, val string) context.Context {
	return context.WithValue(ctx, instanceNameKey, val)
}

func InstanceName(ctx context.Context) string {
	elem := ctx.Value(instanceNameKey)
	if elem == nil {
		return ""
	}
	return elem.(string)
}

func SetInstancePoolID(ctx context.Context, val string) context.Context {
	return context.WithValue(ctx, instancePoolIDKey, val)
}

func InstancePoolID(ctx context.Context) string {
	elem := ctx.Value(instancePoolIDKey)
	if elem == nil {
		return ""
	}
	return elem.(string)
}

func SetInstancePoolType(ctx context.Context, val string) context.Context {
	return context.WithValue(ctx, instancePoolTypeKey, val)
}

func InstancePoolType(ctx context.Context) string {
	elem := ctx.Value(instancePoolTypeKey)
	if elem == nil {
		return ""
	}
	return elem.(string)
}

func SetInstanceEntity(ctx context.Context, val string) context.Context {
	return context.WithValue(ctx, instanceEntityKey, val)
}

func InstanceEntity(ctx context.Context) string {
	elem := ctx.Value(instanceEntityKey)
	if elem == nil {
		return ""
	}
	return elem.(string)
}

func PopulateInstanceContext(ctx context.Context, instance params.Instance) context.Context {
	ctx = SetInstanceID(ctx, instance.ID)
	ctx = SetInstanceName(ctx, instance.Name)
	ctx = SetInstancePoolID(ctx, instance.PoolID)
	ctx = SetInstanceRunnerStatus(ctx, instance.RunnerStatus)
	ctx = SetInstanceTokenFetched(ctx, instance.TokenFetched)
	return ctx
}

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
