package main

import (
	"fmt"
	"testing"
	"time"
)

func TestCommitString(t *testing.T) {
	testTime := time.Now().Unix()
	c := commit{
		Message:    "test commit",
		Timestamp:  testTime,
		FileToBlob: make(map[string]string),
		ParentUIDs: [2]string{},
	}
	testCommitHash := "A123"
	localTestTime := time.Unix(testTime, 0).Local().Format("Mon Jan 02 15:04:05 2006 -0700")
	expected := fmt.Sprintf("commit %v\nDate: %v\ntest commit\n", testCommitHash, localTestTime)
	actual := c.String(testCommitHash)
	if expected != actual {
		t.Fatalf("Commit hash does not match:\nwant %v\ngot %v", actual, expected)
	}
}

func TestParseBlobHeader(t *testing.T) {
	setupTestRepo(t)
	header, err := parseBlobHeader("7914794a7f0269202a9611b759450eb00d5dba47")
	if err != nil {
		t.Fatal(err)
	}
	if header != "commit" {
		t.Fatalf("want 'commit', got '%v'", header)
	}
}

func TestGetCommit(t *testing.T) {
	setupTestRepo(t)
	initialCommit, err := getCommit("7914794a7f0269202a9611b759450eb00d5dba47")
	if err != nil {
		t.Fatal(err)
	}
	if initialCommit.Message != "initial commit" {
		t.Fatalf("incorrect commit message: want 'initial commit', got %v", initialCommit.Message)
	}
}
