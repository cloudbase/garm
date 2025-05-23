// Code generated by go-swagger; DO NOT EDIT.

package endpoints

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	apiserver_params "github.com/cloudbase/garm/apiserver/params"
	garm_params "github.com/cloudbase/garm/params"
)

// UpdateGithubEndpointReader is a Reader for the UpdateGithubEndpoint structure.
type UpdateGithubEndpointReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *UpdateGithubEndpointReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewUpdateGithubEndpointOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewUpdateGithubEndpointDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewUpdateGithubEndpointOK creates a UpdateGithubEndpointOK with default headers values
func NewUpdateGithubEndpointOK() *UpdateGithubEndpointOK {
	return &UpdateGithubEndpointOK{}
}

/*
UpdateGithubEndpointOK describes a response with status code 200, with default header values.

ForgeEndpoint
*/
type UpdateGithubEndpointOK struct {
	Payload garm_params.ForgeEndpoint
}

// IsSuccess returns true when this update github endpoint o k response has a 2xx status code
func (o *UpdateGithubEndpointOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this update github endpoint o k response has a 3xx status code
func (o *UpdateGithubEndpointOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this update github endpoint o k response has a 4xx status code
func (o *UpdateGithubEndpointOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this update github endpoint o k response has a 5xx status code
func (o *UpdateGithubEndpointOK) IsServerError() bool {
	return false
}

// IsCode returns true when this update github endpoint o k response a status code equal to that given
func (o *UpdateGithubEndpointOK) IsCode(code int) bool {
	return code == 200
}

// Code gets the status code for the update github endpoint o k response
func (o *UpdateGithubEndpointOK) Code() int {
	return 200
}

func (o *UpdateGithubEndpointOK) Error() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[PUT /github/endpoints/{name}][%d] updateGithubEndpointOK %s", 200, payload)
}

func (o *UpdateGithubEndpointOK) String() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[PUT /github/endpoints/{name}][%d] updateGithubEndpointOK %s", 200, payload)
}

func (o *UpdateGithubEndpointOK) GetPayload() garm_params.ForgeEndpoint {
	return o.Payload
}

func (o *UpdateGithubEndpointOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewUpdateGithubEndpointDefault creates a UpdateGithubEndpointDefault with default headers values
func NewUpdateGithubEndpointDefault(code int) *UpdateGithubEndpointDefault {
	return &UpdateGithubEndpointDefault{
		_statusCode: code,
	}
}

/*
UpdateGithubEndpointDefault describes a response with status code -1, with default header values.

APIErrorResponse
*/
type UpdateGithubEndpointDefault struct {
	_statusCode int

	Payload apiserver_params.APIErrorResponse
}

// IsSuccess returns true when this update github endpoint default response has a 2xx status code
func (o *UpdateGithubEndpointDefault) IsSuccess() bool {
	return o._statusCode/100 == 2
}

// IsRedirect returns true when this update github endpoint default response has a 3xx status code
func (o *UpdateGithubEndpointDefault) IsRedirect() bool {
	return o._statusCode/100 == 3
}

// IsClientError returns true when this update github endpoint default response has a 4xx status code
func (o *UpdateGithubEndpointDefault) IsClientError() bool {
	return o._statusCode/100 == 4
}

// IsServerError returns true when this update github endpoint default response has a 5xx status code
func (o *UpdateGithubEndpointDefault) IsServerError() bool {
	return o._statusCode/100 == 5
}

// IsCode returns true when this update github endpoint default response a status code equal to that given
func (o *UpdateGithubEndpointDefault) IsCode(code int) bool {
	return o._statusCode == code
}

// Code gets the status code for the update github endpoint default response
func (o *UpdateGithubEndpointDefault) Code() int {
	return o._statusCode
}

func (o *UpdateGithubEndpointDefault) Error() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[PUT /github/endpoints/{name}][%d] UpdateGithubEndpoint default %s", o._statusCode, payload)
}

func (o *UpdateGithubEndpointDefault) String() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[PUT /github/endpoints/{name}][%d] UpdateGithubEndpoint default %s", o._statusCode, payload)
}

func (o *UpdateGithubEndpointDefault) GetPayload() apiserver_params.APIErrorResponse {
	return o.Payload
}

func (o *UpdateGithubEndpointDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
