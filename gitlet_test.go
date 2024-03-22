package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

const initialCommitHash = "f14a7dfac63092f78fb5d209312a84315dd9ef73"

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
	expectedHash := initialCommitHash
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
	actualHash := string(bytes.TrimRight(hashBytes, "\n"))
	if actualHash != expectedHash {
		t.Fatalf("Incorrect main branch head commit hash, want %v, got %v\n", expectedHash, actualHash)
	}
}

func TestAddFile(t *testing.T) {
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
	beforeMetadata, ok := index[testFile]
	if !ok {
		t.Fatalf("Staged file not in index: %v\n", index)
	}
	// check objects for staged file blob
	if _, err = os.Stat(filepath.Join(objectsDir, beforeMetadata.Hash)); err != nil {
		t.Fatal("Staged file blob not found.")
	}

	// modify file and restage
	f, err := os.OpenFile(testFile, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("!"); err != nil {
		f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := stageFile(testFile); err != nil {
		t.Fatal(err)
	}

	// after restaging, previously staged blob should not exist
	if _, err := os.Stat(filepath.Join(objectsDir, beforeMetadata.Hash)); err == nil || !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}

	// restaged file should be in the index
	index, err = readIndex()
	if err != nil {
		t.Fatal(err)
	}
	afterMetadata, ok := index[testFile]
	if !ok {
		t.Fatalf("Restaged file not in index: %v\n", index)
	}
	if beforeMetadata.Hash == afterMetadata.Hash {
		t.Fatal("Hashes are identical before and after staging changes.")
	}
	if _, err = os.Stat(filepath.Join(objectsDir, afterMetadata.Hash)); err != nil {
		t.Fatal("Restaged file blob not found.")
	}

	// restaging a file after deletion
	if err := restrictedDelete(testFile); err != nil {
		t.Fatal(err)
	}
	if err := stageFile(testFile); err != nil {
		t.Fatal(err)
	}

	// after staging, previously staged blob should not exist
	if _, err := os.Stat(filepath.Join(objectsDir, afterMetadata.Hash)); err == nil || !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}

	index, err = readIndex()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := index[testFile]; ok {
		t.Fatal("Restaging after deletion did not remove file from index.")
	}
}

func TestNewCommit(t *testing.T) {
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

func TestRemoveStaged(t *testing.T) {
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
	if len(index) != 1 {
		t.Fatal("Test file was not staged.")
	}
	if err := unstageFile(testFile); err != nil {
		t.Fatal(err)
	}
	index, err = readIndex()
	if err != nil {
		t.Fatal(err)
	}
	if len(index) != 0 {
		t.Fatal("Test file was not unstaged.")
	}
}

func TestRemoveTracked(t *testing.T) {

}

func TestLog(t *testing.T) {}

func TestGlobalLog(t *testing.T) {}

func TestFind(t *testing.T) {}

func TestStatus(t *testing.T) {}

func TestCheckout(t *testing.T) {}

func TestBranch(t *testing.T) {
	setupTestRepo(t)
	testBranch := "foo"
	if err := addBranch(testBranch); err != nil {
		t.Fatal(err)
	}
	testBranchHeadCommitHash, err := readContentsAsString(filepath.Join(branchHeadsDir, testBranch))
	if err != nil {
		t.Fatal(err)
	}
	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		t.Fatal(err)
	}
	currentBranchHeadCommitHash, err := readContentsAsString(currentBranchFile)
	if err != nil {
		t.Fatal(err)
	}
	if testBranchHeadCommitHash != currentBranchHeadCommitHash {
		t.Fatalf(
			"New branch head does not match current branch head, want %v, got %v",
			currentBranchHeadCommitHash, testBranchHeadCommitHash,
		)
	}
}

func TestRemoveBranch(t *testing.T) {
	setupTestRepo(t)
	// TODO: test remove current branch
	// TODO: test remove non-existent branch
	testBranch := "foo"
	if err := addBranch(testBranch); err != nil {
		t.Fatal(err)
	}
	if err := removeBranch(testBranch); err != nil {
		t.Fatal(err)
	}
	// check if branch was deleted
	if _, err := os.Stat(filepath.Join(branchHeadsDir, testBranch)); err == nil {
		t.Fatalf("Branch '%v' was not removed: %v", testBranch, err)
	}
}

func TestReset(t *testing.T) {}

func TestMerge(t *testing.T) {}
