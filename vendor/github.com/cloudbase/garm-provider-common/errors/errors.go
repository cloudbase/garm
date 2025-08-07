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

import "fmt"

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
)

type baseError struct {
	msg string
}

func (b *baseError) Error() string {
	return b.msg
}

// NewProviderError returns a new ProviderError
func NewProviderError(msg string, a ...interface{}) error {
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
func NewMissingSecretError(msg string, a ...interface{}) error {
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
func NewNotFoundError(msg string, a ...interface{}) error {
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
func NewBadRequestError(msg string, a ...interface{}) error {
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
func NewConflictError(msg string, a ...interface{}) error {
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
func NewTimeoutError(msg string, a ...interface{}) error {
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
func NewUnprocessableError(msg string, a ...interface{}) error {
	return &TimoutError{
		baseError{
			msg: fmt.Sprintf(msg, a...),
		},
	}
}

// TimoutError is returned when an operation times out.
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

// NewNoPoolsAvailableError returns a new UnprocessableError
func NewNoPoolsAvailableError(msg string, a ...interface{}) error {
	return &TimoutError{
		baseError{
			msg: fmt.Sprintf(msg, a...),
		},
	}
}

// NoPoolsAvailableError is returned when anthere are not pools available.
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
