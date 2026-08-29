package main

import (
	"errors"
	"reflect"
	"testing"
)

func TestStartNodeNetworkingStartsPubSubBeforeConsensus(t *testing.T) {
	var events []string

	err := startNodeNetworking(
		func() error {
			events = append(events, "pubsub")
			return nil
		},
		func() error {
			events = append(events, "consensus")
			return nil
		},
	)
	if err != nil {
		t.Fatalf("startNodeNetworking returned error: %v", err)
	}

	want := []string{"pubsub", "consensus"}
	if !reflect.DeepEqual(events, want) {
		t.Fatalf("startup order = %v, want %v", events, want)
	}
}

func TestStartNodeNetworkingStopsWhenPubSubFails(t *testing.T) {
	pubsubErr := errors.New("pubsub failed")
	consensusCalled := false

	err := startNodeNetworking(
		func() error { return pubsubErr },
		func() error {
			consensusCalled = true
			return nil
		},
	)
	if !errors.Is(err, pubsubErr) {
		t.Fatalf("error = %v, want %v", err, pubsubErr)
	}
	if consensusCalled {
		t.Fatal("consensus started after pubsub failure")
	}
}
