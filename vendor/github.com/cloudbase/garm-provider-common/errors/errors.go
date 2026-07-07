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

package errors

import (
	"fmt"

	"github.com/cloudbase/garm-provider-common/params"
)

var (
	// ErrUnauthorized is returned when a user does not have
	// authorization to perform a request
	ErrUnauthorized = NewUnauthorizedError("Unauthorized")
	// ErrNotFound is returned if an object is not found in
	// the database.
	ErrNotFound = NewNotFoundError("not found")
	// ErrDuplicateUser is returned when creating a user, if the
	// user already exists.
	ErrDuplicateEntity = NewDuplicateUserError("duplicate")
	// ErrBadRequest is returned is a malformed request is sent
	ErrBadRequest = NewBadRequestError("invalid request")
	// ErrTimeout is returned when a timeout occurs.
	ErrTimeout          = NewTimeoutError("timed out")
	ErrUnprocessable    = NewUnprocessableError("cannot process request")
	ErrNoPoolsAvailable = NewNoPoolsAvailableError("no pools available")
	ErrNoCapacity       = NewNoCapacityError("no capacity available")
)

type baseError struct {
	msg string
}

func (b *baseError) Error() string {
	return b.msg
}

// NewProviderError returns a new ProviderError
func NewProviderError(msg string, a ...any) error {
	return &ProviderError{
		baseError{
			msg: fmt.Sprintf(msg, a...),
		},
	}
}

// UnauthorizedError is returned when a request is unauthorized
type ProviderError struct {
	baseError
}

func (p *ProviderError) Is(target error) bool {
	if target == nil {
		return false
	}

	_, ok := target.(*ProviderError)
	return ok
}

// NewMissingSecretError returns a new MissingSecretError
func NewMissingSecretError(msg string, a ...any) error {
	return &MissingSecretError{
		baseError{
			msg: fmt.Sprintf(msg, a...),
		},
	}
}

// MissingSecretError is returned the secret to validate a webhook is missing
type MissingSecretError struct {
	baseError
}

func (p *MissingSecretError) Is(target error) bool {
	if target == nil {
		return false
	}

	_, ok := target.(*MissingSecretError)
	return ok
}

// NewUnauthorizedError returns a new UnauthorizedError
func NewUnauthorizedError(msg string) error {
	return &UnauthorizedError{
		baseError{
			msg: msg,
		},
	}
}

// UnauthorizedError is returned when a request is unauthorized
type UnauthorizedError struct {
	baseError
}

func (p *UnauthorizedError) Is(target error) bool {
	if target == nil {
		return false
	}

	_, ok := target.(*UnauthorizedError)
	return ok
}

// NewNotFoundError returns a new NotFoundError
func NewNotFoundError(msg string, a ...any) error {
	return &NotFoundError{
		baseError{
			msg: fmt.Sprintf(msg, a...),
		},
	}
}

// NotFoundError is returned when a resource is not found
type NotFoundError struct {
	baseError
}

func (p *NotFoundError) Is(target error) bool {
	if target == nil {
		return false
	}

	_, ok := target.(*NotFoundError)
	return ok
}

// NewDuplicateUserError returns a new DuplicateUserError
func NewDuplicateUserError(msg string) error {
	return &DuplicateUserError{
		baseError{
			msg: msg,
		},
	}
}

// DuplicateUserError is returned when a duplicate user is requested
type DuplicateUserError struct {
	baseError
}

func (p *DuplicateUserError) Is(target error) bool {
	if target == nil {
		return false
	}

	_, ok := target.(*DuplicateUserError)
	return ok
}

// NewBadRequestError returns a new BadRequestError
func NewBadRequestError(msg string, a ...any) error {
	return &BadRequestError{
		baseError{
			msg: fmt.Sprintf(msg, a...),
		},
	}
}

// BadRequestError is returned when a malformed request is received
type BadRequestError struct {
	baseError
}

func (p *BadRequestError) Is(target error) bool {
	if target == nil {
		return false
	}

	_, ok := target.(*BadRequestError)
	return ok
}

// NewConflictError returns a new ConflictError
func NewConflictError(msg string, a ...any) error {
	return &ConflictError{
		baseError{
			msg: fmt.Sprintf(msg, a...),
		},
	}
}

// ConflictError is returned when a conflicting request is made
type ConflictError struct {
	baseError
}

func (p *ConflictError) Is(target error) bool {
	if target == nil {
		return false
	}

	_, ok := target.(*ConflictError)
	return ok
}

// NewTimeoutError returns a new TimoutError
func NewTimeoutError(msg string, a ...any) error {
	return &TimoutError{
		baseError{
			msg: fmt.Sprintf(msg, a...),
		},
	}
}

// TimoutError is returned when an operation times out.
type TimoutError struct {
	baseError
}

func (p *TimoutError) Is(target error) bool {
	if target == nil {
		return false
	}

	_, ok := target.(*TimoutError)
	return ok
}

// NewUnprocessableError returns a new UnprocessableError
func NewUnprocessableError(msg string, a ...any) error {
	return &UnprocessableError{
		baseError{
			msg: fmt.Sprintf(msg, a...),
		},
	}
}

// UnprocessableError is returned when a request cannot be processed.
type UnprocessableError struct {
	baseError
}

func (p *UnprocessableError) Is(target error) bool {
	if target == nil {
		return false
	}

	_, ok := target.(*UnprocessableError)
	return ok
}

// NewNoPoolsAvailableError returns a new NoPoolsAvailableError
func NewNoPoolsAvailableError(msg string, a ...any) error {
	return &NoPoolsAvailableError{
		baseError{
			msg: fmt.Sprintf(msg, a...),
		},
	}
}

// NoPoolsAvailableError is returned when there are no pools available.
type NoPoolsAvailableError struct {
	baseError
}

func (p *NoPoolsAvailableError) Is(target error) bool {
	if target == nil {
		return false
	}

	_, ok := target.(*NoPoolsAvailableError)
	return ok
}

// NewNoCapacityError returns a new NoCapacityError
func NewNoCapacityError(msg string, a ...any) error {
	return &NoCapacityError{
		baseError{
			msg: fmt.Sprintf(msg, a...),
		},
	}
}

// NoCapacityError is returned when there is no capacity available.
type NoCapacityError struct {
	baseError
}

func (p *NoCapacityError) Is(target error) bool {
	if target == nil {
		return false
	}

	_, ok := target.(*NoCapacityError)
	return ok
}

// NewInstanceTransitionError returns an InstanceTransitionError describing an
// invalid instance status state machine transition.
func NewInstanceTransitionError(from, to params.InstanceStatus) error {
	return &InstanceTransitionError{
		baseError: baseError{
			msg: fmt.Sprintf("invalid instance status transition from %s to %s", from, to),
		},
		From: from,
		To:   to,
	}
}

// InstanceTransitionError is returned when a requested instance status
// transition is rejected by the state machine. It carries the current (From)
// and requested (To) statuses as their proper type, so callers working with a
// params.Instance can compare them directly (via errors.As) without casting
// through strings, and decide whether the refusal is benign for their intent
// (for example, the instance is already further along the same terminal path)
// or a genuine error to stop on. It also reports as a BadRequestError, so it
// maps to an HTTP 400.
type InstanceTransitionError struct {
	baseError
	From params.InstanceStatus
	To   params.InstanceStatus
}

func (e *InstanceTransitionError) Is(target error) bool {
	if target == nil {
		return false
	}

	switch target.(type) {
	case *InstanceTransitionError, *BadRequestError:
		return true
	default:
		return false
	}
}
