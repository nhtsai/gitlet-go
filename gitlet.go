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

// newRepository creates a new Gitlet repository with an initial commit and a main branch.
// The repository stored in .gitlet contains the necessary directories and files for Gitlet.
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

	b, err := serialize(initialCommit)
	if err != nil {
		return fmt.Errorf("initRepository: cannot serialize initial commit: %w", err)
	}
	blobData := []any{"commit", []byte{blobHeaderDelim}, b}
	initialCommitHash, err := getHash(blobData)
	if err != nil {
		return fmt.Errorf("initRepository: cannot get initial commit hash: %w", err)
	}
	err = writeContents(filepath.Join(objectsDir, initialCommitHash), blobData)
	if err != nil {
		return fmt.Errorf("initRepository: cannot write initial commit blob: %w", err)
	}

	// create main branch
	mainBranchFile := filepath.Join(branchHeadsDir, "main")
	if err = writeContents(mainBranchFile, []string{initialCommitHash}); err != nil {
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

// stageFile stages a file to be committed.
// If the file is already staged and identical to the file in the working directory, the staging operation is skipped.
// If the file is already staged but modified in the working directory, the file is re-staged, overwriting the previously staged version.
// If the file is already staged, not in the working directory, and tracked in the head commit, the file is already staged for deletion and staging is skipped.
// If the file is not staged, not in the working directory, and tracked in the head commit, then it is staged for deletion.
// If the file is not yet staged and modified, the file will be staged.
func stageFile(file string) error {
	headCommit, err := getHeadCommit()
	if err != nil {
		return fmt.Errorf("stageFile: cannot get head commit: %w", err)
	}
	trackedHash, isTracked := headCommit.FileToBlob[file]

	index, err := readIndex()
	if err != nil {
		return fmt.Errorf("stageFile: cannot read index file: %w", err)
	}
	stagedMetadata, isStaged := index[file]

	wdInfo, err := os.Stat(file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			if isTracked {
				if isStaged {
					// path: not in WD (modified), is staged, is tracked
					// skip, file is already staged for deletion
					return nil
				} else {
					// path: not in WD (modified), not staged, is tracked
					// stage file for deletion
					index[file] = indexMetadata{"DELETE", time.Now().Unix(), 0}
					if err := writeIndex(index); err != nil {
						return fmt.Errorf("stageFile: could not stage file for deletion: %w", err)
					}
					return nil
				}
			} else {
				if isStaged {
					// remove staged blob
					if err := os.Remove(filepath.Join(objectsDir, stagedMetadata.Hash)); err != nil {
						return fmt.Errorf("stageFile: cannot delete old file blob: %w", err)
					}
					// delete from index
					delete(index, file)
					if err := writeIndex(index); err != nil {
						return fmt.Errorf("stageFile: could not remove file from index: %w", err)
					}
					return nil
				} else {
					log.Fatal("File does not exist.")
				}
			}
		} else {
			return fmt.Errorf("stageFile: cannot stat file '%v': %w", file, err)
		}
	}

	// compare metadata of WD and index
	if isStaged &&
		(wdInfo.Size() == stagedMetadata.FileSize) &&
		(wdInfo.ModTime().Unix() == stagedMetadata.ModTime) {
		log.Printf("File '%v' is already staged.\n", file)
		return nil
	}

	// compare hashes of WD and index
	wdContents, err := readContents(file)
	if err != nil {
		return fmt.Errorf("stageFile: cannot read file '%v': %w", file, err)
	}
	wdBlobContents := []any{"file", []byte{blobHeaderDelim}, wdContents}
	wdHash, err := getHash(wdBlobContents)
	if err != nil {
		return fmt.Errorf("stageFile: cannot get file hash: %w", err)
	}
	if isStaged && (wdHash == stagedMetadata.Hash) {
		log.Printf("File '%v' is already staged.\n", file)
		return nil
	}
	// compare hashes of WD and head commit
	if !isStaged && isTracked && (wdHash == trackedHash) {
		log.Printf("No changes detected. Skipping staging...\n")
		return nil
	}

	// file exists in WD and is modified

	// remove previously staged file blob that is now outdated
	if isStaged {
		if err := os.Remove(filepath.Join(objectsDir, stagedMetadata.Hash)); err != nil {
			return fmt.Errorf("stageFile: cannot delete old file blob: %w", err)
		}
	}

	// file is not already staged or should be re-staged
	wdBlobFile := filepath.Join(objectsDir, wdHash)
	if err = writeContents(wdBlobFile, wdBlobContents); err != nil {
		return fmt.Errorf("stageFile: could not write staged file blob: %w", err)
	}

	// update file index
	index[file] = indexMetadata{wdHash, time.Now().Unix(), int64(len(wdContents))}
	if err = writeIndex(index); err != nil {
		return fmt.Errorf("stageFile: could not update file index: %w", err)
	}
	return nil
}

// newCommit creates a new commit.
// Returns an error if commit message is empty or if no files are staged.
func newCommit(message string) error {
	if message == "" {
		log.Fatal("Please enter a commit message.")
	}
	index, err := readIndex()
	if err != nil {
		return fmt.Errorf("newCommit: %w", err)
	}
	if len(index) == 0 {
		log.Fatal("No changes added to commit.")
	}

	c := commit{
		Message:    message,
		Timestamp:  time.Now().UTC().Unix(),
		FileToBlob: make(map[string]string),
		ParentUIDs: [2]string{},
	}

	// set current head commit as parent
	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return fmt.Errorf("newCommit: %w", err)
	}
	headCommitHash, err := readContentsAsString(currentBranchFile)
	if err != nil {
		return fmt.Errorf("newCommit: %w", err)
	}
	c.ParentUIDs[0] = headCommitHash

	headCommit, err := getCommit(headCommitHash)
	if err != nil {
		return fmt.Errorf("newCommit: %w", err)
	}
	// create file to blob mapping from the previous commit
	for file, blobUID := range headCommit.FileToBlob {
		c.FileToBlob[file] = blobUID
	}
	// overwrite mapping with staged files
	for file, metadata := range index {
		if metadata.Hash != "DELETED" {
			c.FileToBlob[file] = metadata.Hash
		}
	}

	// write commit blob
	contents, err := serialize(c)
	if err != nil {
		return fmt.Errorf("newCommit: could not serialize commit: %w", err)
	}
	payload := []any{"commit", []byte{blobHeaderDelim}, contents}
	commitHash, err := getHash(payload)
	if err != nil {
		return fmt.Errorf("newCommit: could not create commit hash: %w", err)
	}
	if err := writeContents(filepath.Join(objectsDir, commitHash), payload); err != nil {
		return fmt.Errorf("newCommit: cannot write commit blob: %w", err)
	}

	// update branch to new commit
	if err := writeContents(currentBranchFile, []string{commitHash}); err != nil {
		return fmt.Errorf("newCommit: cannot update current branch file: %w", err)
	}

	// clear index
	if err := newIndex(); err != nil {
		return fmt.Errorf("newCommit: cannot clear index: %w", err)
	}
	return nil
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
