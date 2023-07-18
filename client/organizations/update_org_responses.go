// Code generated by go-swagger; DO NOT EDIT.

package organizations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	apiserver_params "github.com/cloudbase/garm/apiserver/params"
	garm_params "github.com/cloudbase/garm/params"
)

// UpdateOrgReader is a Reader for the UpdateOrg structure.
type UpdateOrgReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *UpdateOrgReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewUpdateOrgOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewUpdateOrgDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewUpdateOrgOK creates a UpdateOrgOK with default headers values
func NewUpdateOrgOK() *UpdateOrgOK {
	return &UpdateOrgOK{}
}

/*
UpdateOrgOK describes a response with status code 200, with default header values.

Organization
*/
type UpdateOrgOK struct {
	Payload garm_params.Organization
}

// IsSuccess returns true when this update org o k response has a 2xx status code
func (o *UpdateOrgOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this update org o k response has a 3xx status code
func (o *UpdateOrgOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this update org o k response has a 4xx status code
func (o *UpdateOrgOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this update org o k response has a 5xx status code
func (o *UpdateOrgOK) IsServerError() bool {
	return false
}

// IsCode returns true when this update org o k response a status code equal to that given
func (o *UpdateOrgOK) IsCode(code int) bool {
	return code == 200
}

// Code gets the status code for the update org o k response
func (o *UpdateOrgOK) Code() int {
	return 200
}

func (o *UpdateOrgOK) Error() string {
	return fmt.Sprintf("[PUT /organizations/{orgID}][%d] updateOrgOK  %+v", 200, o.Payload)
}

func (o *UpdateOrgOK) String() string {
	return fmt.Sprintf("[PUT /organizations/{orgID}][%d] updateOrgOK  %+v", 200, o.Payload)
}

func (o *UpdateOrgOK) GetPayload() garm_params.Organization {
	return o.Payload
}

func (o *UpdateOrgOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewUpdateOrgDefault creates a UpdateOrgDefault with default headers values
func NewUpdateOrgDefault(code int) *UpdateOrgDefault {
	return &UpdateOrgDefault{
		_statusCode: code,
	}
}

/*
UpdateOrgDefault describes a response with status code -1, with default header values.

APIErrorResponse
*/
type UpdateOrgDefault struct {
	_statusCode int

	Payload apiserver_params.APIErrorResponse
}

// IsSuccess returns true when this update org default response has a 2xx status code
func (o *UpdateOrgDefault) IsSuccess() bool {
	return o._statusCode/100 == 2
}

// IsRedirect returns true when this update org default response has a 3xx status code
func (o *UpdateOrgDefault) IsRedirect() bool {
	return o._statusCode/100 == 3
}

// IsClientError returns true when this update org default response has a 4xx status code
func (o *UpdateOrgDefault) IsClientError() bool {
	return o._statusCode/100 == 4
}

// IsServerError returns true when this update org default response has a 5xx status code
func (o *UpdateOrgDefault) IsServerError() bool {
	return o._statusCode/100 == 5
}

// IsCode returns true when this update org default response a status code equal to that given
func (o *UpdateOrgDefault) IsCode(code int) bool {
	return o._statusCode == code
}

// Code gets the status code for the update org default response
func (o *UpdateOrgDefault) Code() int {
	return o._statusCode
}

func (o *UpdateOrgDefault) Error() string {
	return fmt.Sprintf("[PUT /organizations/{orgID}][%d] UpdateOrg default  %+v", o._statusCode, o.Payload)
}

func (o *UpdateOrgDefault) String() string {
	return fmt.Sprintf("[PUT /organizations/{orgID}][%d] UpdateOrg default  %+v", o._statusCode, o.Payload)
}

func (o *UpdateOrgDefault) GetPayload() apiserver_params.APIErrorResponse {
	return o.Payload
}

func (o *UpdateOrgDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}