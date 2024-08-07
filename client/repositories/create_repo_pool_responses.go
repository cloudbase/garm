// Code generated by go-swagger; DO NOT EDIT.

package repositories

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

// CreateRepoPoolReader is a Reader for the CreateRepoPool structure.
type CreateRepoPoolReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *CreateRepoPoolReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	switch response.Code() {
	case 200:
		result := NewCreateRepoPoolOK()
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		return result, nil
	default:
		result := NewCreateRepoPoolDefault(response.Code())
		if err := result.readResponse(response, consumer, o.formats); err != nil {
			return nil, err
		}
		if response.Code()/100 == 2 {
			return result, nil
		}
		return nil, result
	}
}

// NewCreateRepoPoolOK creates a CreateRepoPoolOK with default headers values
func NewCreateRepoPoolOK() *CreateRepoPoolOK {
	return &CreateRepoPoolOK{}
}

/*
CreateRepoPoolOK describes a response with status code 200, with default header values.

Pool
*/
type CreateRepoPoolOK struct {
	Payload garm_params.Pool
}

// IsSuccess returns true when this create repo pool o k response has a 2xx status code
func (o *CreateRepoPoolOK) IsSuccess() bool {
	return true
}

// IsRedirect returns true when this create repo pool o k response has a 3xx status code
func (o *CreateRepoPoolOK) IsRedirect() bool {
	return false
}

// IsClientError returns true when this create repo pool o k response has a 4xx status code
func (o *CreateRepoPoolOK) IsClientError() bool {
	return false
}

// IsServerError returns true when this create repo pool o k response has a 5xx status code
func (o *CreateRepoPoolOK) IsServerError() bool {
	return false
}

// IsCode returns true when this create repo pool o k response a status code equal to that given
func (o *CreateRepoPoolOK) IsCode(code int) bool {
	return code == 200
}

// Code gets the status code for the create repo pool o k response
func (o *CreateRepoPoolOK) Code() int {
	return 200
}

func (o *CreateRepoPoolOK) Error() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[POST /repositories/{repoID}/pools][%d] createRepoPoolOK %s", 200, payload)
}

func (o *CreateRepoPoolOK) String() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[POST /repositories/{repoID}/pools][%d] createRepoPoolOK %s", 200, payload)
}

func (o *CreateRepoPoolOK) GetPayload() garm_params.Pool {
	return o.Payload
}

func (o *CreateRepoPoolOK) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}

// NewCreateRepoPoolDefault creates a CreateRepoPoolDefault with default headers values
func NewCreateRepoPoolDefault(code int) *CreateRepoPoolDefault {
	return &CreateRepoPoolDefault{
		_statusCode: code,
	}
}

/*
CreateRepoPoolDefault describes a response with status code -1, with default header values.

APIErrorResponse
*/
type CreateRepoPoolDefault struct {
	_statusCode int

	Payload apiserver_params.APIErrorResponse
}

// IsSuccess returns true when this create repo pool default response has a 2xx status code
func (o *CreateRepoPoolDefault) IsSuccess() bool {
	return o._statusCode/100 == 2
}

// IsRedirect returns true when this create repo pool default response has a 3xx status code
func (o *CreateRepoPoolDefault) IsRedirect() bool {
	return o._statusCode/100 == 3
}

// IsClientError returns true when this create repo pool default response has a 4xx status code
func (o *CreateRepoPoolDefault) IsClientError() bool {
	return o._statusCode/100 == 4
}

// IsServerError returns true when this create repo pool default response has a 5xx status code
func (o *CreateRepoPoolDefault) IsServerError() bool {
	return o._statusCode/100 == 5
}

// IsCode returns true when this create repo pool default response a status code equal to that given
func (o *CreateRepoPoolDefault) IsCode(code int) bool {
	return o._statusCode == code
}

// Code gets the status code for the create repo pool default response
func (o *CreateRepoPoolDefault) Code() int {
	return o._statusCode
}

func (o *CreateRepoPoolDefault) Error() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[POST /repositories/{repoID}/pools][%d] CreateRepoPool default %s", o._statusCode, payload)
}

func (o *CreateRepoPoolDefault) String() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[POST /repositories/{repoID}/pools][%d] CreateRepoPool default %s", o._statusCode, payload)
}

func (o *CreateRepoPoolDefault) GetPayload() apiserver_params.APIErrorResponse {
	return o.Payload
}

func (o *CreateRepoPoolDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
