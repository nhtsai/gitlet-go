package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	setupTempDir(t)
	if err := newRepository(); err != nil {
		t.Fatal(err)
	}
	// check dirs and files
	for _, d := range []string{gitletDir, objectsDir, branchHeadsDir, remotesDir, headFile, indexFile} {
		if _, err := os.Stat(d); err != nil {
			t.Fatal(err)
		}
	}
	// check initial commit
	expectedHash := "7914794a7f0269202a9611b759450eb00d5dba47"
	if _, err := os.Stat(filepath.Join(objectsDir, expectedHash)); err != nil {
		t.Fatal(err)
	}
	// check HEAD file
	expectedHeadFile := filepath.Join(branchHeadsDir, "main")
	headBytes, err := os.ReadFile(headFile)
	if err != nil {
		t.Fatal(err)
	}
	actualHeadFile := string(bytes.TrimRight(headBytes, "\n"))
	if actualHeadFile != expectedHeadFile {
		t.Fatalf("Incorrect head file contents, want %v, got %v\n", expectedHeadFile, actualHeadFile)
	}
	// check main branch
	hashBytes, err := os.ReadFile(expectedHeadFile)
	if err != nil {
		t.Fatal(err)
	}
	actualBytes := string(bytes.TrimRight(hashBytes, "\n"))
	if actualBytes != expectedHash {
		t.Fatalf("Incorrect main branch head commit hash, want %v, got %v\n", expectedHash, actualBytes)
	}
}

func TestAdd(t *testing.T) {
	setupTestRepo(t)
	testFile := "wug.txt"
	if err := os.WriteFile(testFile, []byte("This is a wug"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := stageFile(testFile); err != nil {
		t.Fatal(err)
	}
	// check index for staged file
	index, err := readIndex()
	if err != nil {
		t.Fatal(err)
	}
	metadata, ok := index[testFile]
	if !ok {
		t.Fatalf("Staged file not in index: %v\n", index)
	}
	// check objects for staged file blob
	if _, err = os.Stat(filepath.Join(objectsDir, metadata.Hash)); err != nil {
		t.Fatal("Staged file blob not found.")
	}

}

func TestCommit(t *testing.T) {
	setupTestRepo(t)
	testFile := "wug.txt"
	if err := os.WriteFile(testFile, []byte("This is a wug"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := stageFile(testFile); err != nil {
		t.Fatal(err)
	}
	// check index before commit
	idx, err := readIndex()
	if err != nil {
		t.Fatal(err)
	}
	if len(idx) != 1 {
		t.Fatal("File not added.")
	}

	if err := newCommit("add wug file"); err != nil {
		t.Fatal(err)
	}
	objects, err := getFilenames(objectsDir)
	if err != nil {
		t.Fatal(err)
	}
	// expected blobs: initial commit, wug file, wug commit
	if len(objects) != 3 {
		t.Fatalf("Commit and/or file blobs not found. Found %v", objects)
	}
	// check index after commit
	idx, err = readIndex()
	if err != nil {
		t.Fatal(err)
	}
	if len(idx) != 0 {
		t.Fatal("Index not cleared after commit.")
	}
}

func TestRemove(t *testing.T) {}

func TestLog(t *testing.T) {}

func TestGlobalLog(t *testing.T) {}

func TestFind(t *testing.T) {}

func TestStatus(t *testing.T) {}

func TestCheckout(t *testing.T) {}

func TestBranch(t *testing.T) {}

func TestRemoveBranch(t *testing.T) {}

func TestReset(t *testing.T) {}

func TestMerge(t *testing.T) {}
