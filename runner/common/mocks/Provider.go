// Code generated by mockery v2.42.0. DO NOT EDIT.

package mocks

import (
	context "context"

	garm_provider_commonparams "github.com/cloudbase/garm-provider-common/params"
	mock "github.com/stretchr/testify/mock"

	params "github.com/cloudbase/garm/params"
	"github.com/cloudbase/garm/runner/common"
)

// Provider is an autogenerated mock type for the Provider type
type Provider struct {
	mock.Mock
}

// AsParams provides a mock function with given fields:
func (_m *Provider) AsParams() params.Provider {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for AsParams")
	}

	var r0 params.Provider
	if rf, ok := ret.Get(0).(func() params.Provider); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(params.Provider)
	}

	return r0
}

// CreateInstance provides a mock function with given fields: ctx, bootstrapParams
func (_m *Provider) CreateInstance(ctx context.Context, bootstrapParams garm_provider_commonparams.BootstrapInstance, createInstanceParams common.CreateInstanceParams) (garm_provider_commonparams.ProviderInstance, error) {
	ret := _m.Called(ctx, bootstrapParams)

	if len(ret) == 0 {
		panic("no return value specified for CreateInstance")
	}

	var r0 garm_provider_commonparams.ProviderInstance
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, garm_provider_commonparams.BootstrapInstance) (garm_provider_commonparams.ProviderInstance, error)); ok {
		return rf(ctx, bootstrapParams)
	}
	if rf, ok := ret.Get(0).(func(context.Context, garm_provider_commonparams.BootstrapInstance) garm_provider_commonparams.ProviderInstance); ok {
		r0 = rf(ctx, bootstrapParams)
	} else {
		r0 = ret.Get(0).(garm_provider_commonparams.ProviderInstance)
	}

	if rf, ok := ret.Get(1).(func(context.Context, garm_provider_commonparams.BootstrapInstance) error); ok {
		r1 = rf(ctx, bootstrapParams)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// DeleteInstance provides a mock function with given fields: ctx, instance
func (_m *Provider) DeleteInstance(ctx context.Context, instance string, deleteInstanceParams common.DeleteInstanceParams) error {
	ret := _m.Called(ctx, instance)

	if len(ret) == 0 {
		panic("no return value specified for DeleteInstance")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, instance)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DisableJITConfig provides a mock function with given fields:
func (_m *Provider) DisableJITConfig() bool {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for DisableJITConfig")
	}

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// GetInstance provides a mock function with given fields: ctx, instance
func (_m *Provider) GetInstance(ctx context.Context, instance string, getInstanceParams common.GetInstanceParams) (garm_provider_commonparams.ProviderInstance, error) {
	ret := _m.Called(ctx, instance)

	if len(ret) == 0 {
		panic("no return value specified for GetInstance")
	}

	var r0 garm_provider_commonparams.ProviderInstance
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) (garm_provider_commonparams.ProviderInstance, error)); ok {
		return rf(ctx, instance)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) garm_provider_commonparams.ProviderInstance); ok {
		r0 = rf(ctx, instance)
	} else {
		r0 = ret.Get(0).(garm_provider_commonparams.ProviderInstance)
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, instance)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListInstances provides a mock function with given fields: ctx, poolID
func (_m *Provider) ListInstances(ctx context.Context, poolID string, listInstancesParams common.ListInstancesParams) ([]garm_provider_commonparams.ProviderInstance, error) {
	ret := _m.Called(ctx, poolID)

	if len(ret) == 0 {
		panic("no return value specified for ListInstances")
	}

	var r0 []garm_provider_commonparams.ProviderInstance
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, string) ([]garm_provider_commonparams.ProviderInstance, error)); ok {
		return rf(ctx, poolID)
	}
	if rf, ok := ret.Get(0).(func(context.Context, string) []garm_provider_commonparams.ProviderInstance); ok {
		r0 = rf(ctx, poolID)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]garm_provider_commonparams.ProviderInstance)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, string) error); ok {
		r1 = rf(ctx, poolID)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// RemoveAllInstances provides a mock function with given fields: ctx
func (_m *Provider) RemoveAllInstances(ctx context.Context, removeAllInstances common.RemoveAllInstancesParams) error {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for RemoveAllInstances")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context) error); ok {
		r0 = rf(ctx)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Start provides a mock function with given fields: ctx, instance
func (_m *Provider) Start(ctx context.Context, instance string, startParams common.StartParams) error {
	ret := _m.Called(ctx, instance)

	if len(ret) == 0 {
		panic("no return value specified for Start")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, instance)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// Stop provides a mock function with given fields: ctx, instance
func (_m *Provider) Stop(ctx context.Context, instance string, stopParams common.StopParams) error {
	ret := _m.Called(ctx, instance)

	if len(ret) == 0 {
		panic("no return value specified for Stop")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, string) error); ok {
		r0 = rf(ctx, instance)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// NewProvider creates a new instance of Provider. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewProvider(t interface {
	mock.TestingT
	Cleanup(func())
}) *Provider {
	mock := &Provider{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
