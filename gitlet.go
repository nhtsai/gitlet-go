package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	gitletDir      string = ".gitlet"
	objectsDir     string = filepath.Join(gitletDir, "objects")
	branchHeadsDir string = filepath.Join(gitletDir, "refs", "heads")
	remotesDir     string = filepath.Join(gitletDir, "remotes")
	headFile       string = filepath.Join(gitletDir, "HEAD")
	indexFile      string = filepath.Join(gitletDir, "INDEX")
)

func newRepository() error {
	fi, err := os.Stat(gitletDir)
	if (err == nil) && fi.IsDir() {
		log.Fatal("A Gitlet version-control system already exists in the current directory.")
	}

	os.MkdirAll(objectsDir, 0755)
	os.MkdirAll(branchHeadsDir, 0755)
	os.MkdirAll(remotesDir, 0755)

	// create initial commit
	var initialCommit commit
	initialCommit.Message = "initial commit"
	initialCommit.Timestamp = time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC).Unix()

	b, err := serialize[commit](initialCommit)
	if err != nil {
		return err
	}
	blobData := []any{"commit", []byte{blobHeaderDelim}, b}
	initialCommitHash, err := getHash(blobData)
	if err != nil {
		return err
	}
	err = writeContents[any](filepath.Join(objectsDir, initialCommitHash), blobData)
	if err != nil {
		return fmt.Errorf("initRepository: cannot write initial commit blob: %w", err)
	}

	// create main branch
	mainBranchFile := filepath.Join(branchHeadsDir, "main")
	if err = writeContents[string](mainBranchFile, []string{initialCommitHash}); err != nil {
		return fmt.Errorf("initRepository: cannot create main branch: %w", err)
	}

	// set current branch to main branch
	if err = writeContents(headFile, []string{mainBranchFile}); err != nil {
		return fmt.Errorf("initRepository: cannot set HEAD file: %w", err)
	}

	// set up index file
	if err = newIndex(); err != nil {
		return fmt.Errorf("initRepository: cannot create index: %w", err)
	}
	return nil
}

// Add a file to the staging area and index file.
// If the file is not yet staged, stage it.
// If the file is already staged and the working directory and index are identical,
// skip the staging operation.
// If the file is already staged and changed, overwrite the staged version.
func stageFile(file string) error {
	index, err := readIndex()
	if err != nil {
		return err
	}

	stagedInfo, isStaged := index[file]
	wdInfo, err := os.Stat(file)
	if err != nil {
		return err
	}

	// working directory is identical to staged
	if isStaged &&
		(wdInfo.Size() == stagedInfo.FileSize) &&
		(wdInfo.ModTime().Unix() == stagedInfo.ModTime) {
		log.Printf("File '%v' is already staged.\n", file)
		return nil
	}

	// compare hashes
	wdContents, err := readContents(file)
	if err != nil {
		return err
	}
	wdHash, err := getHash[any]([]any{"string", wdContents})
	if err != nil {
		return err
	}

	// working directory is identical to staged
	if isStaged && (wdHash == stagedInfo.Hash) {
		log.Printf("File '%v' is already staged.\n", file)
		return nil
	}

	// stage file in working directory
	fileBlobPath := filepath.Join(".gitlet", "objects", wdHash)
	if err := writeContents(fileBlobPath, [][]byte{wdContents}); err != nil {
		return err
	}

	// update file index
	stagedInfo.Hash = wdHash
	index[file] = stagedInfo
	writeIndex(index)
	return nil
}

func newCommit(message string) (commit, error) {
	var c commit
	index, err := readIndex()
	if err != nil {
		return c, err
	}
	// create commit
	c.Message = message
	c.Timestamp = time.Now().UTC().Unix()
	c.FileToBlob = make(map[string]string)
	for k, v := range index {
		c.FileToBlob[k] = v.Hash
	}

	// get current head commit hash
	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return c, err
	}
	headCommitHash, err := readContentsAsString(currentBranchFile)
	if err != nil {
		return c, err
	}
	c.ParentUIDs = [2]string{headCommitHash}

	return c, nil
}

// Removes a file from the staging area if it is currently staged.
// If file is tracked in current commit, stage for removal (deletion), remove file from working directory
// Returns an error if the file is not staged or tracked by head commit.
func unstageFile(file string) error {
	index, err := readIndex()
	if err != nil {
		return err
	}
	_, isStaged := index[file]

	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return err
	}
	headCommitHash, err := readContentsAsString(currentBranchFile)
	if err != nil {
		return err
	}
	headCommitBytes, err := readContents(filepath.Join(objectsDir, headCommitHash))
	if err != nil {
		return err
	}
	headCommit, err := deserialize[commit](headCommitBytes)
	if err != nil {
		return err
	}

	_, isTracked := headCommit.FileToBlob[file]

	if !isStaged && !isTracked {
		log.Fatal("No reason to remove the file.")
	}
	return nil
}

// Print commit log from head of current branch to initial commit.
func printBranchLog() error {
	headBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return err
	}
	headCommitHash, err := readContentsAsString(headBranchFile)
	if err != nil {
		return err
	}
	headCommitData, err := readContents(filepath.Join(objectsDir, headCommitHash))
	if err != nil {
		return err
	}
	headCommit, err := deserialize[commit](headCommitData)
	if err != nil {
		return err
	}
	var curr = headCommit
	var currHash = headCommitHash
	for {
		fmt.Printf("===\n%v\n", curr.String(currHash))
		if curr.ParentUIDs[0] == "" {
			break
		}
		currHash = curr.ParentUIDs[0]
		curr, err = getCommit(currHash)
		if err != nil {
			return err
		}
	}
	return nil
}

// Print all commits in any order.
func printAllCommits() error {
	return filepath.WalkDir(
		objectsDir,
		func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() {
				return nil
			}
			c, c_err := getCommit(d.Name())
			if c_err != nil {
				return c_err
			}
			fmt.Printf("===\n%v\n", c.String(d.Name()))
			return err
		},
	)
}

// Print all UIDs of commits with messages that contain a given substring query.
func printAllCommitIDsByMessage(query string) error {
	found := false
	err := filepath.WalkDir(
		objectsDir,
		func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() {
				return nil
			}
			c, c_err := getCommit(d.Name())
			if c_err != nil {
				return c_err
			}
			if strings.Contains(c.Message, query) {
				found = true
				fmt.Printf("commit %v\n", d.Name())
			}
			return err
		},
	)
	if err != nil {
		return err
	}
	if !found {
		fmt.Println("Found no commit with that message.")
	}
	return nil
}

// TODO: status

// TODO: checkout

// Add a new branch pointing to the head commit of the current branch.
// Does not automatically switch to the new branch.
func addBranch(branchName string) error {
	branchFile := filepath.Join(branchHeadsDir, branchName)
	if _, err := os.Stat(branchFile); err == nil {
		log.Fatal("A branch with that name already exists.")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return err
	}
	headCommitHash, err := readContents(currentBranchFile)
	if err != nil {
		return err
	}
	if err := writeContents(branchFile, [][]byte{headCommitHash}); err != nil {
		return err
	}
	log.Printf("Branch '%v' was created on commit (%v).\n", branchName, string(headCommitHash[:6]))
	return nil
}

// rm-branch
func removeBranch(branchName string) error {
	headBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return err
	}
	if filepath.Base(headBranchFile) == branchName {
		log.Fatal("Cannot remove the current branch.")
	}

	err = os.Remove(filepath.Join(branchHeadsDir, branchName))
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

// reset
func resetFile(file string) error {
	return nil
}

// merge
func mergeBranch(branchName string) error {
	return nil
}
