// Code generated by go-swagger; DO NOT EDIT.

package organizations

// This file was generated by the swagger tool.
// Editing this file might prove futile when you re-run the swagger generate command

import (
	"github.com/go-openapi/runtime"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
)

// New creates a new organizations API client.
func New(transport runtime.ClientTransport, formats strfmt.Registry) ClientService {
	return &Client{transport: transport, formats: formats}
}

// New creates a new organizations API client with basic auth credentials.
// It takes the following parameters:
// - host: http host (github.com).
// - basePath: any base path for the API client ("/v1", "/v3").
// - scheme: http scheme ("http", "https").
// - user: user for basic authentication header.
// - password: password for basic authentication header.
func NewClientWithBasicAuth(host, basePath, scheme, user, password string) ClientService {
	transport := httptransport.New(host, basePath, []string{scheme})
	transport.DefaultAuthentication = httptransport.BasicAuth(user, password)
	return &Client{transport: transport, formats: strfmt.Default}
}

// New creates a new organizations API client with a bearer token for authentication.
// It takes the following parameters:
// - host: http host (github.com).
// - basePath: any base path for the API client ("/v1", "/v3").
// - scheme: http scheme ("http", "https").
// - bearerToken: bearer token for Bearer authentication header.
func NewClientWithBearerToken(host, basePath, scheme, bearerToken string) ClientService {
	transport := httptransport.New(host, basePath, []string{scheme})
	transport.DefaultAuthentication = httptransport.BearerToken(bearerToken)
	return &Client{transport: transport, formats: strfmt.Default}
}

/*
Client for organizations API
*/
type Client struct {
	transport runtime.ClientTransport
	formats   strfmt.Registry
}

// ClientOption may be used to customize the behavior of Client methods.
type ClientOption func(*runtime.ClientOperation)

// ClientService is the interface for Client methods
type ClientService interface {
	CreateOrg(params *CreateOrgParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*CreateOrgOK, error)

	CreateOrgPool(params *CreateOrgPoolParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*CreateOrgPoolOK, error)

	CreateOrgScaleSet(params *CreateOrgScaleSetParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*CreateOrgScaleSetOK, error)

	DeleteOrg(params *DeleteOrgParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) error

	DeleteOrgPool(params *DeleteOrgPoolParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) error

	GetOrg(params *GetOrgParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*GetOrgOK, error)

	GetOrgPool(params *GetOrgPoolParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*GetOrgPoolOK, error)

	GetOrgWebhookInfo(params *GetOrgWebhookInfoParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*GetOrgWebhookInfoOK, error)

	InstallOrgWebhook(params *InstallOrgWebhookParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*InstallOrgWebhookOK, error)

	ListOrgInstances(params *ListOrgInstancesParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*ListOrgInstancesOK, error)

	ListOrgPools(params *ListOrgPoolsParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*ListOrgPoolsOK, error)

	ListOrgScaleSets(params *ListOrgScaleSetsParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*ListOrgScaleSetsOK, error)

	ListOrgs(params *ListOrgsParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*ListOrgsOK, error)

	UninstallOrgWebhook(params *UninstallOrgWebhookParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) error

	UpdateOrg(params *UpdateOrgParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*UpdateOrgOK, error)

	UpdateOrgPool(params *UpdateOrgPoolParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*UpdateOrgPoolOK, error)

	SetTransport(transport runtime.ClientTransport)
}

/*
CreateOrg creates organization with the parameters given
*/
func (a *Client) CreateOrg(params *CreateOrgParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*CreateOrgOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewCreateOrgParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "CreateOrg",
		Method:             "POST",
		PathPattern:        "/organizations",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &CreateOrgReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*CreateOrgOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*CreateOrgDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
CreateOrgPool creates organization pool with the parameters given
*/
func (a *Client) CreateOrgPool(params *CreateOrgPoolParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*CreateOrgPoolOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewCreateOrgPoolParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "CreateOrgPool",
		Method:             "POST",
		PathPattern:        "/organizations/{orgID}/pools",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &CreateOrgPoolReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*CreateOrgPoolOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*CreateOrgPoolDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
CreateOrgScaleSet creates organization scale set with the parameters given
*/
func (a *Client) CreateOrgScaleSet(params *CreateOrgScaleSetParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*CreateOrgScaleSetOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewCreateOrgScaleSetParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "CreateOrgScaleSet",
		Method:             "POST",
		PathPattern:        "/organizations/{orgID}/scalesets",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &CreateOrgScaleSetReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*CreateOrgScaleSetOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*CreateOrgScaleSetDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
DeleteOrg deletes organization by ID
*/
func (a *Client) DeleteOrg(params *DeleteOrgParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) error {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewDeleteOrgParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "DeleteOrg",
		Method:             "DELETE",
		PathPattern:        "/organizations/{orgID}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &DeleteOrgReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	_, err := a.transport.Submit(op)
	if err != nil {
		return err
	}
	return nil
}

/*
DeleteOrgPool deletes organization pool by ID
*/
func (a *Client) DeleteOrgPool(params *DeleteOrgPoolParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) error {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewDeleteOrgPoolParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "DeleteOrgPool",
		Method:             "DELETE",
		PathPattern:        "/organizations/{orgID}/pools/{poolID}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &DeleteOrgPoolReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	_, err := a.transport.Submit(op)
	if err != nil {
		return err
	}
	return nil
}

/*
GetOrg gets organization by ID
*/
func (a *Client) GetOrg(params *GetOrgParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*GetOrgOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewGetOrgParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "GetOrg",
		Method:             "GET",
		PathPattern:        "/organizations/{orgID}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &GetOrgReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*GetOrgOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*GetOrgDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
GetOrgPool gets organization pool by ID
*/
func (a *Client) GetOrgPool(params *GetOrgPoolParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*GetOrgPoolOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewGetOrgPoolParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "GetOrgPool",
		Method:             "GET",
		PathPattern:        "/organizations/{orgID}/pools/{poolID}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &GetOrgPoolReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*GetOrgPoolOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*GetOrgPoolDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
GetOrgWebhookInfo gets information about the g a r m installed webhook on an organization
*/
func (a *Client) GetOrgWebhookInfo(params *GetOrgWebhookInfoParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*GetOrgWebhookInfoOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewGetOrgWebhookInfoParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "GetOrgWebhookInfo",
		Method:             "GET",
		PathPattern:        "/organizations/{orgID}/webhook",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &GetOrgWebhookInfoReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*GetOrgWebhookInfoOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*GetOrgWebhookInfoDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
	InstallOrgWebhook Install the GARM webhook for an organization. The secret configured on the organization will

be used to validate the requests.
*/
func (a *Client) InstallOrgWebhook(params *InstallOrgWebhookParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*InstallOrgWebhookOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewInstallOrgWebhookParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "InstallOrgWebhook",
		Method:             "POST",
		PathPattern:        "/organizations/{orgID}/webhook",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &InstallOrgWebhookReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*InstallOrgWebhookOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*InstallOrgWebhookDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
ListOrgInstances lists organization instances
*/
func (a *Client) ListOrgInstances(params *ListOrgInstancesParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*ListOrgInstancesOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListOrgInstancesParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "ListOrgInstances",
		Method:             "GET",
		PathPattern:        "/organizations/{orgID}/instances",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &ListOrgInstancesReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*ListOrgInstancesOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*ListOrgInstancesDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
ListOrgPools lists organization pools
*/
func (a *Client) ListOrgPools(params *ListOrgPoolsParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*ListOrgPoolsOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListOrgPoolsParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "ListOrgPools",
		Method:             "GET",
		PathPattern:        "/organizations/{orgID}/pools",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &ListOrgPoolsReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*ListOrgPoolsOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*ListOrgPoolsDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
ListOrgScaleSets lists organization scale sets
*/
func (a *Client) ListOrgScaleSets(params *ListOrgScaleSetsParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*ListOrgScaleSetsOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListOrgScaleSetsParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "ListOrgScaleSets",
		Method:             "GET",
		PathPattern:        "/organizations/{orgID}/scalesets",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &ListOrgScaleSetsReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*ListOrgScaleSetsOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*ListOrgScaleSetsDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
ListOrgs lists organizations
*/
func (a *Client) ListOrgs(params *ListOrgsParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*ListOrgsOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewListOrgsParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "ListOrgs",
		Method:             "GET",
		PathPattern:        "/organizations",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &ListOrgsReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*ListOrgsOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*ListOrgsDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
UninstallOrgWebhook uninstalls organization webhook
*/
func (a *Client) UninstallOrgWebhook(params *UninstallOrgWebhookParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) error {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewUninstallOrgWebhookParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "UninstallOrgWebhook",
		Method:             "DELETE",
		PathPattern:        "/organizations/{orgID}/webhook",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &UninstallOrgWebhookReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	_, err := a.transport.Submit(op)
	if err != nil {
		return err
	}
	return nil
}

/*
UpdateOrg updates organization with the parameters given
*/
func (a *Client) UpdateOrg(params *UpdateOrgParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*UpdateOrgOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewUpdateOrgParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "UpdateOrg",
		Method:             "PUT",
		PathPattern:        "/organizations/{orgID}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &UpdateOrgReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*UpdateOrgOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*UpdateOrgDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

/*
UpdateOrgPool updates organization pool with the parameters given
*/
func (a *Client) UpdateOrgPool(params *UpdateOrgPoolParams, authInfo runtime.ClientAuthInfoWriter, opts ...ClientOption) (*UpdateOrgPoolOK, error) {
	// TODO: Validate the params before sending
	if params == nil {
		params = NewUpdateOrgPoolParams()
	}
	op := &runtime.ClientOperation{
		ID:                 "UpdateOrgPool",
		Method:             "PUT",
		PathPattern:        "/organizations/{orgID}/pools/{poolID}",
		ProducesMediaTypes: []string{"application/json"},
		ConsumesMediaTypes: []string{"application/json"},
		Schemes:            []string{"http"},
		Params:             params,
		Reader:             &UpdateOrgPoolReader{formats: a.formats},
		AuthInfo:           authInfo,
		Context:            params.Context,
		Client:             params.HTTPClient,
	}
	for _, opt := range opts {
		opt(op)
	}

	result, err := a.transport.Submit(op)
	if err != nil {
		return nil, err
	}
	success, ok := result.(*UpdateOrgPoolOK)
	if ok {
		return success, nil
	}
	// unexpected success response
	unexpectedSuccess := result.(*UpdateOrgPoolDefault)
	return nil, runtime.NewAPIError("unexpected success response: content available as default response in error", unexpectedSuccess, unexpectedSuccess.Code())
}

// SetTransport changes the transport on the client
func (a *Client) SetTransport(transport runtime.ClientTransport) {
	a.transport = transport
}
