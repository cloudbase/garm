// Code generated by go-swagger; DO NOT EDIT.

package credentials

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
	"github.com/go-openapi/swag"

	garm_params "github.com/cloudbase/garm/params"
)

// NewUpdateGiteaCredentialsParams creates a new UpdateGiteaCredentialsParams object,
// with the default timeout for this client.
//
// Default values are not hydrated, since defaults are normally applied by the API server side.
//
// To enforce default values in parameter, use SetDefaults or WithDefaults.
func NewUpdateGiteaCredentialsParams() *UpdateGiteaCredentialsParams {
	return &UpdateGiteaCredentialsParams{
		timeout: cr.DefaultTimeout,
	}
}

// NewUpdateGiteaCredentialsParamsWithTimeout creates a new UpdateGiteaCredentialsParams object
// with the ability to set a timeout on a request.
func NewUpdateGiteaCredentialsParamsWithTimeout(timeout time.Duration) *UpdateGiteaCredentialsParams {
	return &UpdateGiteaCredentialsParams{
		timeout: timeout,
	}
}

// NewUpdateGiteaCredentialsParamsWithContext creates a new UpdateGiteaCredentialsParams object
// with the ability to set a context for a request.
func NewUpdateGiteaCredentialsParamsWithContext(ctx context.Context) *UpdateGiteaCredentialsParams {
	return &UpdateGiteaCredentialsParams{
		Context: ctx,
	}
}

// NewUpdateGiteaCredentialsParamsWithHTTPClient creates a new UpdateGiteaCredentialsParams object
// with the ability to set a custom HTTPClient for a request.
func NewUpdateGiteaCredentialsParamsWithHTTPClient(client *http.Client) *UpdateGiteaCredentialsParams {
	return &UpdateGiteaCredentialsParams{
		HTTPClient: client,
	}
}

/*
UpdateGiteaCredentialsParams contains all the parameters to send to the API endpoint

	for the update gitea credentials operation.

	Typically these are written to a http.Request.
*/
type UpdateGiteaCredentialsParams struct {

	/* Body.

	   Parameters used when updating a Gitea credential.
	*/
	Body garm_params.UpdateGiteaCredentialsParams

	/* ID.

	   ID of the Gitea credential.
	*/
	ID int64

	timeout    time.Duration
	Context    context.Context
	HTTPClient *http.Client
}

// WithDefaults hydrates default values in the update gitea credentials params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *UpdateGiteaCredentialsParams) WithDefaults() *UpdateGiteaCredentialsParams {
	o.SetDefaults()
	return o
}

// SetDefaults hydrates default values in the update gitea credentials params (not the query body).
//
// All values with no default are reset to their zero value.
func (o *UpdateGiteaCredentialsParams) SetDefaults() {
	// no default values defined for this parameter
}

// WithTimeout adds the timeout to the update gitea credentials params
func (o *UpdateGiteaCredentialsParams) WithTimeout(timeout time.Duration) *UpdateGiteaCredentialsParams {
	o.SetTimeout(timeout)
	return o
}

// SetTimeout adds the timeout to the update gitea credentials params
func (o *UpdateGiteaCredentialsParams) SetTimeout(timeout time.Duration) {
	o.timeout = timeout
}

// WithContext adds the context to the update gitea credentials params
func (o *UpdateGiteaCredentialsParams) WithContext(ctx context.Context) *UpdateGiteaCredentialsParams {
	o.SetContext(ctx)
	return o
}

// SetContext adds the context to the update gitea credentials params
func (o *UpdateGiteaCredentialsParams) SetContext(ctx context.Context) {
	o.Context = ctx
}

// WithHTTPClient adds the HTTPClient to the update gitea credentials params
func (o *UpdateGiteaCredentialsParams) WithHTTPClient(client *http.Client) *UpdateGiteaCredentialsParams {
	o.SetHTTPClient(client)
	return o
}

// SetHTTPClient adds the HTTPClient to the update gitea credentials params
func (o *UpdateGiteaCredentialsParams) SetHTTPClient(client *http.Client) {
	o.HTTPClient = client
}

// WithBody adds the body to the update gitea credentials params
func (o *UpdateGiteaCredentialsParams) WithBody(body garm_params.UpdateGiteaCredentialsParams) *UpdateGiteaCredentialsParams {
	o.SetBody(body)
	return o
}

// SetBody adds the body to the update gitea credentials params
func (o *UpdateGiteaCredentialsParams) SetBody(body garm_params.UpdateGiteaCredentialsParams) {
	o.Body = body
}

// WithID adds the id to the update gitea credentials params
func (o *UpdateGiteaCredentialsParams) WithID(id int64) *UpdateGiteaCredentialsParams {
	o.SetID(id)
	return o
}

// SetID adds the id to the update gitea credentials params
func (o *UpdateGiteaCredentialsParams) SetID(id int64) {
	o.ID = id
}

// WriteToRequest writes these params to a swagger request
func (o *UpdateGiteaCredentialsParams) WriteToRequest(r runtime.ClientRequest, reg strfmt.Registry) error {

	if err := r.SetTimeout(o.timeout); err != nil {
		return err
	}
	var res []error
	if err := r.SetBodyParam(o.Body); err != nil {
		return err
	}

	// path param id
	if err := r.SetPathParam("id", swag.FormatInt64(o.ID)); err != nil {
		return err
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}
