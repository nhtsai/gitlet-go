package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

func initRepository() error {
	fi, err := os.Stat(".gitlet")
	if (err == nil) && fi.IsDir() {
		log.Fatal("A Gitlet version-control system already exists in the current directory.")
	}

	os.MkdirAll(".gitlet/objects", 0755)
	os.MkdirAll(".gitlet/refs/heads", 0755)
	os.MkdirAll(".gitlet/remotes", 0755)

	// create initial commit
	initialCommitUID, err := getHash[any](nil)
	if err != nil {
		return err
	}
	initialCommit := &commit{
		UID:        initialCommitUID,
		Message:    "initial commit",
		Timestamp:  time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC),
		FileToBlob: make(map[string]string),
		ParentUIDs: [2]string{},
	}

	// write initial commit blob
	err = initialCommit.writeBlob()
	if err != nil {
		return fmt.Errorf("initRepository: cannot write initial commit blob: %w", err)
	}

	// create main branch
	mainBranchFile := filepath.Join(".gitlet", "refs", "heads", "main")
	err = writeContents(mainBranchFile, []string{initialCommitUID})
	if err != nil {
		return fmt.Errorf("initRepository: cannot create main branch: %w", err)
	}

	// set current branch to main branch
	err = writeContents(
		filepath.Join(".gitlet", "HEAD"),
		[]string{mainBranchFile},
	)
	if err != nil {
		return fmt.Errorf("initRepository: cannot set HEAD file: %w", err)
	}

	// set up index file
	err = newIndex()
	if err != nil {
		return fmt.Errorf("initRepository: cannot create index: %w", err)
	}
	return nil
}
