package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func mkTestDir(t *testing.T, dir string) {
	t.Helper()
	err1 := os.Mkdir(dir, 0755)
	err2 := os.Chmod(dir, 0755)
	if err := errors.Join(err1, err2); err != nil {
		t.FailNow()
	}
}

func setupTempDir(t *testing.T) {
	t.Helper()
	if err := os.Chdir(t.TempDir()); err != nil {
		t.FailNow()
	}
}

func setupTestRepo(t *testing.T) {
	t.Helper()
	setupTempDir(t)
	if err := newRepository(); err != nil {
		t.FailNow()
	}
}

func TestGetFilenames(t *testing.T) {
	setupTestRepo(t)
	wd, err := os.Getwd()
	if err != nil {
		t.Error(err)
	}
	expected := []string{"bar.js", "foo.go", "wug.txt"}
	for _, testFile := range expected {
		if _, err := os.Create(filepath.Join(wd, testFile)); err != nil {
			t.Error(err)
		}
	}
	files, err := getFilenames(wd)
	if err != nil {
		t.Errorf("Could not read directory %v: %v", wd, err)
	}
	if slices.Compare(files, expected) != 0 {
		t.Fail()
		t.Logf("Incorrect filenames returned, want %v, got %v", expected, files)
	}
}

func TestGetHash(t *testing.T) {
	contents := []any{"This page intentionally ", []byte("left blank.")}
	actual, err := getHash(contents)
	if err != nil {
		t.Errorf("Could not get hash.")
	}
	expected := "af064923bbf2301596aac4c273ba32178ebc4a96"
	if len(actual) != 40 {
		t.Errorf("Incorrect hash length, want 40, got %d.", len(actual))
	}
	if actual != expected {
		t.Errorf("Want %v, got %v", expected, actual)
	}
}

func TestRestrictedDeleteDirectory(t *testing.T) {
	setupTestRepo(t)
	testDir := "foo"
	mkTestDir(t, testDir)
	if err := restrictedDelete(testDir); err == nil {
		t.Fatalf("restrictedDelete('%v') occurred, want fail.", testDir)
	}
}

func TestRestrictedDeleteFileNotExist(t *testing.T) {
	setupTestRepo(t)
	testFile := "baz.go"
	err := restrictedDelete(testFile)
	if err != nil {
		t.Fatalf("restrictedDelete('%v') occurred, should do nothing.", testFile)
	}
}

func TestRestrictedDeleteFile(t *testing.T) {
	setupTestRepo(t)
	mkTestDir(t, "foo")
	mkTestDir(t, filepath.Join("foo", "bar"))
	testFile := filepath.Join("foo", "bar", "baz.go")
	f, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Could not create test file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if err := restrictedDelete(testFile); err != nil {
		t.Fatalf("restrictedDelete('%v') did not occur as expected", testFile)
	}
}

func TestReadContentsToBytes(t *testing.T) {
	setupTempDir(t)
	testFile := "foo.txt"
	expected := []byte("Hello, world!")
	os.WriteFile(testFile, expected, 0644)
	actual, err := readContents(testFile)
	if err != nil {
		t.Fatalf("Could not read test file: %v", err)
	}
	if slices.Compare(actual, expected) != 0 {
		t.Fatal("Wrong contents read from test file.")
	}
}

func TestReadContentsToString(t *testing.T) {
	setupTempDir(t)
	testFile := "foo.txt"
	expected := []byte("Hello, world!")
	os.WriteFile(testFile, expected, 0644)
	actual, err := readContentsAsString(testFile)
	if err != nil {
		t.Fatalf("Could not read test file: %v", err)
	}
	if actual != string(expected) {
		t.Fatalf(`Wrong contents read from test file.`)
	}
}

func TestWriteContents(t *testing.T) {
	setupTempDir(t)
	testFile := "foo.txt"
	expected := []byte("Hello, world!")
	err := writeContents[[]byte](testFile, [][]byte{expected})
	if err != nil {
		t.Fatalf("Could not write to test file: %v", err)
	}
	actual, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Could not read test file: %v", err)
	}
	actual = bytes.TrimRight(actual, "\n")
	if slices.Compare(actual, expected) != 0 {
		t.Fatalf("Wrong contents written to test file: want %v, got %v\n", expected, actual)
	}
}

func TestSerialization(t *testing.T) {
	expected := "This is a wug."
	b, err := serialize(expected)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := deserialize[string](b)
	if err != nil {
		t.Fatal(err)
	}
	if actual != expected {
		t.Fatalf("Incorrect serialization/deserialization: want %v, got %v", expected, actual)
	}
}
