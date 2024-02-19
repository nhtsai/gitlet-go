package main

import (
	"bytes"
	"errors"
	"os"
	"slices"
	"testing"
)

func mkTempDir(dir string) error {
	err1 := os.Mkdir(dir, 0755)
	err2 := os.Chmod(dir, 0755)
	if err := errors.Join(err1, err2); err != nil {
		return err
	}
	return nil
}

func setupTempDir(t *testing.T) {
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
}

func setupTestRepo(t *testing.T) {
	setupTempDir(t)
	if err := initRepository(); err != nil {
		t.Fatal(err)
	}
}

func TestGetFilenames(t *testing.T) {
	files, err := getFilenames(".")
	if err != nil {
		t.Errorf("Could not read directory %v: %v", ".", err)
	}
	for _, f := range files {
		t.Log(f)
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
	os.Mkdir(testDir, 0755)
	err := restrictedDelete(testDir)
	if err == nil {
		t.Fatalf(`restrictedDelete("%v") occurred, want fail.`, testDir)
	}
}

func TestRestrictedDeleteFileNotExist(t *testing.T) {
	setupTestRepo(t)
	testFile := "baz.go"
	err := restrictedDelete(testFile)
	if err == nil {
		t.Fatalf(`restrictedDelete("%v") occurred, want fail.`, testFile)
	}
}

func TestRestrictedDeleteFile(t *testing.T) {
	setupTestRepo(t)
	testFile := "baz.go"
	f, err := os.Create(testFile)
	if err != nil {
		t.Errorf("Could not create test file: %v", err)
	}
	f.Close()
	err = restrictedDelete(testFile)
	if err != nil {
		t.Fatalf(`restrictedDelete("%v") did not occur as expected.`, testFile)
	}
}

func TestReadContentsToBytes(t *testing.T) {
	setupTempDir(t)
	testFile := "foo.txt"
	expected := []byte("Hello, world!")
	os.WriteFile(testFile, expected, 0644)

	actual, err := readContentsToBytes(testFile)
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

	actual, err := readContentsToString(testFile)
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

func TestCreateBlobFromFile(t *testing.T) {
	tempDir := setupTempDir(t)

	testFile := filepath.Join(tempDir, "foo.txt")
	expected := []byte("This is a wug.")
	err := os.WriteFile(testFile, expected, 0644)
	if err != nil {
		t.Error(err)
	}

	err = os.MkdirAll(filepath.Join(tempDir, ".gitlet", "blob"), 0755)
	if err != nil {
		t.Error(err)
	}
	// needs execute permission
	err = os.Chmod(filepath.Join(tempDir, ".gitlet", "blob"), 0755)
	if err != nil {
		t.Errorf("Could not set .gitlet directory perms: %v", err)
	}

	err = createBlobFromFile(testFile)
	if err != nil {
		t.Error(err)
	}
	files, err := os.ReadDir(filepath.Join(tempDir, ".gitlet", "blob"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if file.Name() == "b0438c11aca0470310517c59f2cbd763d1e5cbb4" {
			return
		}
	}
	t.Fail()
}
