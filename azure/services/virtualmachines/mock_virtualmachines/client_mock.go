/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by MockGen. DO NOT EDIT.
// Source: ../client.go

// Package mock_virtualmachines is a generated GoMock package.
package mock_virtualmachines

import (
	context "context"
	reflect "reflect"

	compute "github.com/Azure/azure-sdk-for-go/services/compute/mgmt/2021-11-01/compute"
	autorest "github.com/Azure/go-autorest/autorest"
	azure "github.com/Azure/go-autorest/autorest/azure"
	gomock "go.uber.org/mock/gomock"
	v1beta1 "sigs.k8s.io/cluster-api-provider-azure/api/v1beta1"
	azure0 "sigs.k8s.io/cluster-api-provider-azure/azure"
)

// MockClient is a mock of Client interface.
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *MockClientMockRecorder
}

// MockClientMockRecorder is the mock recorder for MockClient.
type MockClientMockRecorder struct {
	mock *MockClient
}

// NewMockClient creates a new mock instance.
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &MockClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClient) EXPECT() *MockClientMockRecorder {
	return m.recorder
}

// CreateOrUpdateAsync mocks base method.
func (m *MockClient) CreateOrUpdateAsync(ctx context.Context, spec azure0.ResourceSpecGetter, parameters interface{}) (interface{}, azure.FutureAPI, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateOrUpdateAsync", ctx, spec, parameters)
	ret0, _ := ret[0].(interface{})
	ret1, _ := ret[1].(azure.FutureAPI)
	ret2, _ := ret[2].(error)
	return ret0, ret1, ret2
}

// CreateOrUpdateAsync indicates an expected call of CreateOrUpdateAsync.
func (mr *MockClientMockRecorder) CreateOrUpdateAsync(ctx, spec, parameters interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateOrUpdateAsync", reflect.TypeOf((*MockClient)(nil).CreateOrUpdateAsync), ctx, spec, parameters)
}

// DeleteAsync mocks base method.
func (m *MockClient) DeleteAsync(ctx context.Context, spec azure0.ResourceSpecGetter) (azure.FutureAPI, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteAsync", ctx, spec)
	ret0, _ := ret[0].(azure.FutureAPI)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeleteAsync indicates an expected call of DeleteAsync.
func (mr *MockClientMockRecorder) DeleteAsync(ctx, spec interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteAsync", reflect.TypeOf((*MockClient)(nil).DeleteAsync), ctx, spec)
}

// Get mocks base method.
func (m *MockClient) Get(arg0 context.Context, arg1 azure0.ResourceSpecGetter) (interface{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", arg0, arg1)
	ret0, _ := ret[0].(interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockClientMockRecorder) Get(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockClient)(nil).Get), arg0, arg1)
}

// GetByID mocks base method.
func (m *MockClient) GetByID(arg0 context.Context, arg1 string) (compute.VirtualMachine, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetByID", arg0, arg1)
	ret0, _ := ret[0].(compute.VirtualMachine)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetByID indicates an expected call of GetByID.
func (mr *MockClientMockRecorder) GetByID(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetByID", reflect.TypeOf((*MockClient)(nil).GetByID), arg0, arg1)
}

// GetResultIfDone mocks base method.
func (m *MockClient) GetResultIfDone(ctx context.Context, future *v1beta1.Future) (compute.VirtualMachine, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetResultIfDone", ctx, future)
	ret0, _ := ret[0].(compute.VirtualMachine)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetResultIfDone indicates an expected call of GetResultIfDone.
func (mr *MockClientMockRecorder) GetResultIfDone(ctx, future interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetResultIfDone", reflect.TypeOf((*MockClient)(nil).GetResultIfDone), ctx, future)
}

// IsDone mocks base method.
func (m *MockClient) IsDone(ctx context.Context, future azure.FutureAPI) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "IsDone", ctx, future)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// IsDone indicates an expected call of IsDone.
func (mr *MockClientMockRecorder) IsDone(ctx, future interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "IsDone", reflect.TypeOf((*MockClient)(nil).IsDone), ctx, future)
}

// Result mocks base method.
func (m *MockClient) Result(ctx context.Context, future azure.FutureAPI, futureType string) (interface{}, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Result", ctx, future, futureType)
	ret0, _ := ret[0].(interface{})
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Result indicates an expected call of Result.
func (mr *MockClientMockRecorder) Result(ctx, future, futureType interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Result", reflect.TypeOf((*MockClient)(nil).Result), ctx, future, futureType)
}

// MockgenericVMFuture is a mock of genericVMFuture interface.
type MockgenericVMFuture struct {
	ctrl     *gomock.Controller
	recorder *MockgenericVMFutureMockRecorder
}

// MockgenericVMFutureMockRecorder is the mock recorder for MockgenericVMFuture.
type MockgenericVMFutureMockRecorder struct {
	mock *MockgenericVMFuture
}

// NewMockgenericVMFuture creates a new mock instance.
func NewMockgenericVMFuture(ctrl *gomock.Controller) *MockgenericVMFuture {
	mock := &MockgenericVMFuture{ctrl: ctrl}
	mock.recorder = &MockgenericVMFutureMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockgenericVMFuture) EXPECT() *MockgenericVMFutureMockRecorder {
	return m.recorder
}

// DoneWithContext mocks base method.
func (m *MockgenericVMFuture) DoneWithContext(ctx context.Context, sender autorest.Sender) (bool, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DoneWithContext", ctx, sender)
	ret0, _ := ret[0].(bool)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DoneWithContext indicates an expected call of DoneWithContext.
func (mr *MockgenericVMFutureMockRecorder) DoneWithContext(ctx, sender interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DoneWithContext", reflect.TypeOf((*MockgenericVMFuture)(nil).DoneWithContext), ctx, sender)
}

// Result mocks base method.
func (m *MockgenericVMFuture) Result(client compute.VirtualMachinesClient) (compute.VirtualMachine, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Result", client)
	ret0, _ := ret[0].(compute.VirtualMachine)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Result indicates an expected call of Result.
func (mr *MockgenericVMFutureMockRecorder) Result(client interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Result", reflect.TypeOf((*MockgenericVMFuture)(nil).Result), client)
}
