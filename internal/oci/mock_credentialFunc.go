// Code generated by mockery v2.42.1. DO NOT EDIT.

package oci

import (
	mock "github.com/stretchr/testify/mock"
	auth "oras.land/oras-go/v2/registry/remote/auth"
)

// mockCredentialFunc is an autogenerated mock type for the credentialFunc type
type mockCredentialFunc struct {
	mock.Mock
}

// Execute provides a mock function with given fields: registry
func (_m *mockCredentialFunc) Execute(registry string) auth.CredentialFunc {
	ret := _m.Called(registry)

	if len(ret) == 0 {
		panic("no return value specified for Execute")
	}

	var r0 auth.CredentialFunc
	if rf, ok := ret.Get(0).(func(string) auth.CredentialFunc); ok {
		r0 = rf(registry)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(auth.CredentialFunc)
		}
	}

	return r0
}

// newMockCredentialFunc creates a new instance of mockCredentialFunc. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func newMockCredentialFunc(t interface {
	mock.TestingT
	Cleanup(func())
}) *mockCredentialFunc {
	mock := &mockCredentialFunc{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}