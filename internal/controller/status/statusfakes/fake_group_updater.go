// Code generated by counterfeiter. DO NOT EDIT.
package statusfakes

import (
	"context"
	"sync"

	"github.com/nginx/nginx-gateway-fabric/v2/internal/controller/status"
)

type FakeGroupUpdater struct {
	UpdateGroupStub        func(context.Context, string, ...status.UpdateRequest)
	updateGroupMutex       sync.RWMutex
	updateGroupArgsForCall []struct {
		arg1 context.Context
		arg2 string
		arg3 []status.UpdateRequest
	}
	invocations      map[string][][]interface{}
	invocationsMutex sync.RWMutex
}

func (fake *FakeGroupUpdater) UpdateGroup(arg1 context.Context, arg2 string, arg3 ...status.UpdateRequest) {
	fake.updateGroupMutex.Lock()
	fake.updateGroupArgsForCall = append(fake.updateGroupArgsForCall, struct {
		arg1 context.Context
		arg2 string
		arg3 []status.UpdateRequest
	}{arg1, arg2, arg3})
	stub := fake.UpdateGroupStub
	fake.recordInvocation("UpdateGroup", []interface{}{arg1, arg2, arg3})
	fake.updateGroupMutex.Unlock()
	if stub != nil {
		fake.UpdateGroupStub(arg1, arg2, arg3...)
	}
}

func (fake *FakeGroupUpdater) UpdateGroupCallCount() int {
	fake.updateGroupMutex.RLock()
	defer fake.updateGroupMutex.RUnlock()
	return len(fake.updateGroupArgsForCall)
}

func (fake *FakeGroupUpdater) UpdateGroupCalls(stub func(context.Context, string, ...status.UpdateRequest)) {
	fake.updateGroupMutex.Lock()
	defer fake.updateGroupMutex.Unlock()
	fake.UpdateGroupStub = stub
}

func (fake *FakeGroupUpdater) UpdateGroupArgsForCall(i int) (context.Context, string, []status.UpdateRequest) {
	fake.updateGroupMutex.RLock()
	defer fake.updateGroupMutex.RUnlock()
	argsForCall := fake.updateGroupArgsForCall[i]
	return argsForCall.arg1, argsForCall.arg2, argsForCall.arg3
}

func (fake *FakeGroupUpdater) Invocations() map[string][][]interface{} {
	fake.invocationsMutex.RLock()
	defer fake.invocationsMutex.RUnlock()
	copiedInvocations := map[string][][]interface{}{}
	for key, value := range fake.invocations {
		copiedInvocations[key] = value
	}
	return copiedInvocations
}

func (fake *FakeGroupUpdater) recordInvocation(key string, args []interface{}) {
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

var _ status.GroupUpdater = new(FakeGroupUpdater)
