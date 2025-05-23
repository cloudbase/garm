// Code generated by go-swagger; DO NOT EDIT.

package scalesets

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/go-openapi/runtime"
	"github.com/go-openapi/strfmt"

	apiserver_params "github.com/cloudbase/garm/apiserver/params"
)

// DeleteScaleSetReader is a Reader for the DeleteScaleSet structure.
type DeleteScaleSetReader struct {
	formats strfmt.Registry
}

// ReadResponse reads a server response into the received o.
func (o *DeleteScaleSetReader) ReadResponse(response runtime.ClientResponse, consumer runtime.Consumer) (interface{}, error) {
	result := NewDeleteScaleSetDefault(response.Code())
	if err := result.readResponse(response, consumer, o.formats); err != nil {
		return nil, err
	}
	if response.Code()/100 == 2 {
		return result, nil
	}
	return nil, result
}

// NewDeleteScaleSetDefault creates a DeleteScaleSetDefault with default headers values
func NewDeleteScaleSetDefault(code int) *DeleteScaleSetDefault {
	return &DeleteScaleSetDefault{
		_statusCode: code,
	}
}

/*
DeleteScaleSetDefault describes a response with status code -1, with default header values.

APIErrorResponse
*/
type DeleteScaleSetDefault struct {
	_statusCode int

	Payload apiserver_params.APIErrorResponse
}

// IsSuccess returns true when this delete scale set default response has a 2xx status code
func (o *DeleteScaleSetDefault) IsSuccess() bool {
	return o._statusCode/100 == 2
}

// IsRedirect returns true when this delete scale set default response has a 3xx status code
func (o *DeleteScaleSetDefault) IsRedirect() bool {
	return o._statusCode/100 == 3
}

// IsClientError returns true when this delete scale set default response has a 4xx status code
func (o *DeleteScaleSetDefault) IsClientError() bool {
	return o._statusCode/100 == 4
}

// IsServerError returns true when this delete scale set default response has a 5xx status code
func (o *DeleteScaleSetDefault) IsServerError() bool {
	return o._statusCode/100 == 5
}

// IsCode returns true when this delete scale set default response a status code equal to that given
func (o *DeleteScaleSetDefault) IsCode(code int) bool {
	return o._statusCode == code
}

// Code gets the status code for the delete scale set default response
func (o *DeleteScaleSetDefault) Code() int {
	return o._statusCode
}

func (o *DeleteScaleSetDefault) Error() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[DELETE /scalesets/{scalesetID}][%d] DeleteScaleSet default %s", o._statusCode, payload)
}

func (o *DeleteScaleSetDefault) String() string {
	payload, _ := json.Marshal(o.Payload)
	return fmt.Sprintf("[DELETE /scalesets/{scalesetID}][%d] DeleteScaleSet default %s", o._statusCode, payload)
}

func (o *DeleteScaleSetDefault) GetPayload() apiserver_params.APIErrorResponse {
	return o.Payload
}

func (o *DeleteScaleSetDefault) readResponse(response runtime.ClientResponse, consumer runtime.Consumer, formats strfmt.Registry) error {

	// response payload
	if err := consumer.Consume(response.Body(), &o.Payload); err != nil && err != io.EOF {
		return err
	}

	return nil
}
