package main

import (
	"reflect"
	"testing"
	"time"
)

func TestIndex(t *testing.T) {
	setupTestRepo(t)
	var expectedIndex stagedFileMap = make(stagedFileMap)
	expectedIndex["foo"] = stageMetadata{"123", time.Now().UTC().Unix(), 123}
	expectedIndex["bar"] = stageMetadata{"456", time.Now().UTC().Unix(), 456}

	if err := writeIndex(expectedIndex); err != nil {
		t.Fatal(err)
	}

	actualIndex, err := readIndex()
	if err != nil {
		t.Fatal(err)
	}

	if reflect.DeepEqual(expectedIndex, actualIndex) == false {
		t.Fatalf("Index written and read incorrectly: want %v, got %v", expectedIndex, actualIndex)
	}
}
