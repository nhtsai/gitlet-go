package main

import (
	"errors"
	"fmt"
	"io/fs"
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
	var initialCommit = new(commit)
	initialCommit.UID = initialCommitUID
	initialCommit.Message = "initial commit"
	initialCommit.Timestamp = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()

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

// Print commit log from head of current branch to initial commit.
func printBranchLog() error {
	headBranchFile, err := readContentsToString(filepath.Join(".gitlet", "HEAD"))
	if err != nil {
		return err
	}
	headCommitHash, err := readContentsToString(headBranchFile)
	if err != nil {
		return err
	}
	headCommitData, err := readContentsToBytes(filepath.Join(".gitlet", "objects", headCommitHash))
	if err != nil {
		return err
	}
	headCommit, err := deserialize[commit](headCommitData)
	if err != nil {
		return err
	}
	var curr = headCommit
	for {
		fmt.Printf("===\n%v\n", curr.String())
		if curr.ParentUIDs[0] == "" {
			break
		}
		curr, err = getCommit(curr.ParentUIDs[0])
		if err != nil {
			return err
		}
	}
	return nil
}

// Add a new branch pointing to the head commit of the current branch.
// Does not automatically switch to the new branch.
func addBranch(branchName string) error {
	branchFile := filepath.Join(".gitlet", "refs", "heads", branchName)
	if _, err := os.Stat(branchFile); err == nil {
		log.Fatal("A branch with that name already exists.")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	currentBranchFile, err := readContentsToString(filepath.Join(".gitlet", "HEAD"))
	if err != nil {
		return err
	}
	headCommitHash, err := readContentsToBytes(currentBranchFile)
	if err != nil {
		return err
	}
	if err := writeContents(branchFile, [][]byte{headCommitHash}); err != nil {
		return err
	}
	log.Printf("Branch '%v' was created on commit (%v).\n", branchName, string(headCommitHash[:6]))
	return nil
}

func removeBranch(branchName string) error {
	headBranchFile, err := readContentsToString(filepath.Join(".gitlet", "HEAD"))
	if err != nil {
		return err
	}
	if filepath.Base(headBranchFile) == branchName {
		log.Fatal("Cannot remove the current branch.")
	}

	err = os.Remove(filepath.Join(".gitlet", "refs", "heads", branchName))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Fatal("A branch with that name does not exist.")
		} else {
			return err
		}
	}
	log.Printf("Branch '%v' has been deleted.\n", branchName)
	return nil
}
