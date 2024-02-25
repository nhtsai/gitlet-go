package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInit(t *testing.T) {
	setupTempDir(t)
	if err := newRepository(); err != nil {
		t.Fatal(err)
	}
	// filepath.WalkDir(".gitlet", func(path string, d fs.DirEntry, err error) error {
	// 	t.Log(d.IsDir(), d.Name(), path)
	// 	return nil
	// })
	if _, err := os.Stat(".gitlet"); err != nil {
		t.Fatal(err)
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

func TestCommit(t *testing.T) {}

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
