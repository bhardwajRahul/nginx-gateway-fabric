// Code generated by counterfeiter. DO NOT EDIT.
package kubernetesfakes

import (
	"context"
	"sync"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/framework/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type FakeReader struct {
	GetStub        func(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error
	getMutex       sync.RWMutex
	getArgsForCall []struct {
		arg1 context.Context
		arg2 client.ObjectKey
		arg3 client.Object
		arg4 []client.GetOption
	}
	getReturns struct {
		result1 error
	}
	getReturnsOnCall map[int]struct {
		result1 error
	}
	ListStub        func(context.Context, client.ObjectList, ...client.ListOption) error
	listMutex       sync.RWMutex
	listArgsForCall []struct {
		arg1 context.Context
		arg2 client.ObjectList
		arg3 []client.ListOption
	}
	listReturns struct {
		result1 error
	}
	listReturnsOnCall map[int]struct {
		result1 error
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeReader) Get(arg1 context.Context, arg2 client.ObjectKey, arg3 client.Object, arg4 ...client.GetOption) error {
	fake.getMutex.Lock()
	ret, specificReturn := fake.getReturnsOnCall[len(fake.getArgsForCall)]
	fake.getArgsForCall = append(fake.getArgsForCall, struct {
		arg1 context.Context
		arg2 client.ObjectKey
		arg3 client.Object
		arg4 []client.GetOption
	}{arg1, arg2, arg3, arg4})
	stub := fake.GetStub
	fakeReturns := fake.getReturns
	fake.recordInvocation("Get", []interface{}{arg1, arg2, arg3, arg4})
	fake.getMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3, arg4...)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeReader) GetCallCount() int {
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	return len(fake.getArgsForCall)
}

func (fake *FakeReader) GetCalls(stub func(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = stub
}

func (fake *FakeReader) GetArgsForCall(i int) (context.Context, client.ObjectKey, client.Object, []client.GetOption) {
	fake.getMutex.RLock()
	defer fake.getMutex.RUnlock()
	argsForCall := fake.getArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3, argsForCall.arg4
}

func (fake *FakeReader) GetReturns(result1 error) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = nil
	fake.getReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeReader) GetReturnsOnCall(i int, result1 error) {
	fake.getMutex.Lock()
	defer fake.getMutex.Unlock()
	fake.GetStub = nil
	if fake.getReturnsOnCall == nil {
		fake.getReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.getReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeReader) List(arg1 context.Context, arg2 client.ObjectList, arg3 ...client.ListOption) error {
	fake.listMutex.Lock()
	ret, specificReturn := fake.listReturnsOnCall[len(fake.listArgsForCall)]
	fake.listArgsForCall = append(fake.listArgsForCall, struct {
		arg1 context.Context
		arg2 client.ObjectList
		arg3 []client.ListOption
	}{arg1, arg2, arg3})
	stub := fake.ListStub
	fakeReturns := fake.listReturns
	fake.recordInvocation("List", []interface{}{arg1, arg2, arg3})
	fake.listMutex.Unlock()
	if stub != nil {
		return stub(arg1, arg2, arg3...)
	}
	if specificReturn {
		return ret.result1
	}
	return fakeReturns.result1
}

func (fake *FakeReader) ListCallCount() int {
	fake.listMutex.RLock()
	defer fake.listMutex.RUnlock()
	return len(fake.listArgsForCall)
}

func (fake *FakeReader) ListCalls(stub func(context.Context, client.ObjectList, ...client.ListOption) error) {
	fake.listMutex.Lock()
	defer fake.listMutex.Unlock()
	fake.ListStub = stub
}

func (fake *FakeReader) ListArgsForCall(i int) (context.Context, client.ObjectList, []client.ListOption) {
	fake.listMutex.RLock()
	defer fake.listMutex.RUnlock()
	argsForCall := fake.listArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeReader) ListReturns(result1 error) {
	fake.listMutex.Lock()
	defer fake.listMutex.Unlock()
	fake.ListStub = nil
	fake.listReturns = struct {
		result1 error
	}{result1}
}

func (fake *FakeReader) ListReturnsOnCall(i int, result1 error) {
	fake.listMutex.Lock()
	defer fake.listMutex.Unlock()
	fake.ListStub = nil
	if fake.listReturnsOnCall == nil {
		fake.listReturnsOnCall = make(map[int]struct {
			result1 error
		})
	}
	fake.listReturnsOnCall[i] = struct {
		result1 error
	}{result1}
}

func (fake *FakeReader) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeReader) recordInvocation(key string, args []interface{}) {
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

var _ kubernetes.Reader = new(FakeReader)
