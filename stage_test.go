package main

import (
	"os"
	"path/filepath"
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

	if eq := reflect.DeepEqual(expectedIndex, actualIndex); !eq {
		t.Fatalf("Index written and read incorrectly: want %v, got %v", expectedIndex, actualIndex)
	}
}

func TestStageFile(t *testing.T) {
	setupTestRepo(t)
	testFile := "wug.txt"
	if err := os.WriteFile(testFile, []byte("This is a wug"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := stageFile(testFile); err != nil {
		t.Fatal(err)
	}

	index, err := readIndex()
	if err != nil {
		t.Fatal(err)
	}
	metadata, ok := index[testFile]
	if !ok {
		t.Fatalf("Staged file not in index: %v\n", index)
	}
	_, err = os.Stat(filepath.Join(".gitlet", "objects", metadata.Hash))
	if err != nil {
		t.Fatal("Staged file blob not found.")
	}

}
