// Code generated by go-swagger; DO NOT EDIT.

package scalesets

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"context"
	"net/http"
	"time"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/runtime"
	cr "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

// NewDeleteScaleSetParams creates a new DeleteScaleSetParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewDeleteScaleSetParams() *DeleteScaleSetParams {
	return &DeleteScaleSetParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewDeleteScaleSetParamsWithTimeout creates a new DeleteScaleSetParams object
// with the ability to set a timeout on a request.
func NewDeleteScaleSetParamsWithTimeout(timeout time.Duration) *DeleteScaleSetParams {
	return &DeleteScaleSetParams{
		timeout: timeout,
	}
}

// NewDeleteScaleSetParamsWithContext creates a new DeleteScaleSetParams object
// with the ability to set a context for a request.
func NewDeleteScaleSetParamsWithContext(ctx context.Context) *DeleteScaleSetParams {
	return &DeleteScaleSetParams{
		Context: ctx,
	}
}

// NewDeleteScaleSetParamsWithHTTPClient creates a new DeleteScaleSetParams object
// with the ability to set a custom HTTPClient for a request.
func NewDeleteScaleSetParamsWithHTTPClient(client *http.Client) *DeleteScaleSetParams {
	return &DeleteScaleSetParams{
		HTTPClient: client,
	}
}

/*
DeleteScaleSetParams contains all the parameters to send to the API endpoint

	for the delete scale set operation.

	Typically these are written to a http.Request.
*/
type DeleteScaleSetParams struct {

	/* ScalesetID.

	   ID of the scale set to delete.
	*/
	ScalesetID string

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the delete scale set params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *DeleteScaleSetParams) WithDefaults() *DeleteScaleSetParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the delete scale set params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *DeleteScaleSetParams) SetDefaults() {
	// no default values defined for this parameter
}

// WithTimeout adds the timeout to the delete scale set params
func (o *DeleteScaleSetParams) WithTimeout(timeout time.Duration) *DeleteScaleSetParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the delete scale set params
func (o *DeleteScaleSetParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the delete scale set params
func (o *DeleteScaleSetParams) WithContext(ctx context.Context) *DeleteScaleSetParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the delete scale set params
func (o *DeleteScaleSetParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the delete scale set params
func (o *DeleteScaleSetParams) WithHTTPClient(client *http.Client) *DeleteScaleSetParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the delete scale set params
func (o *DeleteScaleSetParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithScalesetID adds the scalesetID to the delete scale set params
func (o *DeleteScaleSetParams) WithScalesetID(scalesetID string) *DeleteScaleSetParams {
	o.SetScalesetID(scalesetID)
	return o
}

// SetScalesetID adds the scalesetId to the delete scale set params
func (o *DeleteScaleSetParams) SetScalesetID(scalesetID string) {
	o.ScalesetID = scalesetID
}

// WriteToRequest writes these params to a swagger request
func (o *DeleteScaleSetParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error

	// path param scalesetID
	if err := r.SetPathParam("scalesetID", o.ScalesetID); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
