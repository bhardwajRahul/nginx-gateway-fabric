// Code generated by counterfeiter. DO NOT EDIT.
package provisionerfakes

import (
	"context"
	"sync"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/provisioner"
	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/state/graph"
)

type FakeProvisioner struct {
	RegisterGatewayStub        func(context.Context, *graph.Gateway, string) error
	registerGatewayMutex       sync.RWMutex
	registerGatewayArgsForCall []struct {
		arg1 context.Context
		arg2 *graph.Gateway
		arg3 string
	}
	registerGatewayReturns struct {
		result1 error
	}
	registerGatewayReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeProvisioner) RegisterGateway(arg1 context.Context, arg2 *graph.Gateway, arg3 string) error {
	fake.registerGatewayMutex.Lock()
	ret, specificReturn := fake.registerGatewayReturnsOnCall[len(fake.registerGatewayArgsForCall)]
	fake.registerGatewayArgsForCall = append(fake.registerGatewayArgsForCall, struct {
		arg1 context.Context
		arg2 *graph.Gateway
		arg3 string
	}{arg1, arg2, arg3})
	stub := fake.RegisterGatewayStub
	fakeReturns := fake.registerGatewayReturns
	fake.recordInvocation("RegisterGateway", []interface{}{arg1, arg2, arg3})
	fake.registerGatewayMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeProvisioner) RegisterGatewayCallCount() int {
	fake.registerGatewayMutex.RLock()
	defer fake.registerGatewayMutex.RUnlock()
	return len(fake.registerGatewayArgsForCall)
}

func (fake *FakeProvisioner) RegisterGatewayCalls(stub func(context.Context, *graph.Gateway, string) error) {
	fake.registerGatewayMutex.Lock()
	defer fake.registerGatewayMutex.Unlock()
	fake.RegisterGatewayStub = stub
}

func (fake *FakeProvisioner) RegisterGatewayArgsForCall(i int) (context.Context, *graph.Gateway, string) {
	fake.registerGatewayMutex.RLock()
	defer fake.registerGatewayMutex.RUnlock()
	argsForCall := fake.registerGatewayArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeProvisioner) RegisterGatewayReturns(result1 error) {
	fake.registerGatewayMutex.Lock()
	defer fake.registerGatewayMutex.Unlock()
	fake.RegisterGatewayStub = nil
	fake.registerGatewayReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeProvisioner) RegisterGatewayReturnsOnCall(i int, result1 error) {
	fake.registerGatewayMutex.Lock()
	defer fake.registerGatewayMutex.Unlock()
	fake.RegisterGatewayStub = nil
	if fake.registerGatewayReturnsOnCall == nil {
		fake.registerGatewayReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.registerGatewayReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeProvisioner) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeProvisioner) recordInvocation(key string, args []interface{}) {
	fake.invocationsMutex.Lock()
	defer fake.invocationsMutex.Unlock()
	if fake.invocations == nil {
		fake.invocations = map[string][][]interface{}{}
	}
	if fake.invocations[key] == nil {
		fake.invocations[key] = [][]interface{}{}
	}
	fake.invocations[key] = append(fake.invocations[key], args)
}

var _ provisioner.Provisioner = new(FakeProvisioner)
