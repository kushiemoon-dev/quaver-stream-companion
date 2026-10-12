package main

import (
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/kushiemoon-dev/quaver-stream-companion/internal/keys"
)

type idleSampler struct{}

func (idleSampler) Down(k []keys.Key) []bool { return make([]bool, len(k)) }

// testSamplerCreated counts the creations that went through the test factory.
var testSamplerCreated atomic.Int64

func useFakeSampler() {
	newSampler = func() (keys.Sampler, error) {
		testSamplerCreated.Add(1)
		return idleSampler{}, nil
	}
}

func TestRealSamplerFactoryIsNotReachedByTheTests(t *testing.T) {
	if reflect.ValueOf(newSampler).Pointer() == reflect.ValueOf(keys.NewSampler).Pointer() {
		t.Fatal("cmd tests must run with the fake sampler factory")
	}
}
