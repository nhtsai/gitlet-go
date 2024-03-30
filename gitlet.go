package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	gitletDir              string = ".gitlet"
	stagedForRemovalMarker string = "DELETED"
)

var (
	objectsDir  string = filepath.Join(gitletDir, "objects")
	refsDir     string = filepath.Join(gitletDir, "refs")
	branchesDir string = filepath.Join(refsDir, "heads")
	remotesDir  string = filepath.Join(refsDir, "remotes")
	headFile    string = filepath.Join(gitletDir, "HEAD")
	indexFile   string = filepath.Join(gitletDir, "INDEX")
	remoteFile  string = filepath.Join(gitletDir, "REMOTE")
)

// newRepository creates a new Gitlet repository with an initial commit and a main branch.
// The repository stored in .gitlet contains the necessary directories and files for Gitlet.
func newRepository() error {
	if dirInfo, err := os.Stat(gitletDir); err == nil {
		if dirInfo.IsDir() {
			log.Fatal("A Gitlet version-control system already exists in the current directory.")
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("newRepository: %w", err)
	}

	if err := errors.Join(
		os.Mkdir(gitletDir, 0755),
		os.Mkdir(objectsDir, 0755),
		os.Mkdir(refsDir, 0755),
		os.Mkdir(branchesDir, 0755),
		os.Mkdir(remotesDir, 0755),
	); err != nil {
		return fmt.Errorf("newRepository: %w", err)
	}

	initialCommit := commit{
		Message:    "initial commit",
		Timestamp:  time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC).Unix(),
		FileToBlob: make(map[string]string),
		ParentUIDs: [2]string{"", ""},
	}

	contents, err := serialize(initialCommit)
	if err != nil {
		return fmt.Errorf("initRepository: cannot serialize initial commit: %w", err)
	}
	payload := []any{"commit", []byte{blobHeaderDelim}, contents}
	initialCommitHash, err := getHash(payload)
	if err != nil {
		return fmt.Errorf("initRepository: cannot get initial commit hash: %w", err)
	}
	err = writeContents(filepath.Join(objectsDir, initialCommitHash), payload)
	if err != nil {
		return fmt.Errorf("initRepository: cannot write initial commit blob: %w", err)
	}

	// create main branch
	mainBranchFile := filepath.Join(branchesDir, "main")
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
//
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
				// path: not in WD (modified), is staged (for deletion), is tracked
				if isStaged && stagedMetadata.Hash == stagedForRemovalMarker {
					log.Printf("File '%v' is already staged.\n", file)
					return nil
				}
				// path: not in WD (modified), not staged (for deletion), is tracked
				// stage file for deletion
				index[file] = indexMetadata{stagedForRemovalMarker, time.Now().Unix(), 0}
				if err := writeIndex(index); err != nil {
					return fmt.Errorf("stageFile: could not stage file for deletion: %w", err)
				}
				return nil
			} else {
				if isStaged {
					// path: not in WD
					// remove staged blob
					if err := restrictedDelete(filepath.Join(objectsDir, stagedMetadata.Hash)); err != nil {
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
	wdBlobPayload := []any{"file", []byte{blobHeaderDelim}, wdContents}
	wdHash, err := getHash(wdBlobPayload)
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

	// path: file exists in WD and is modified

	// remove previously staged file blob that is now outdated
	if isStaged {
		if err := restrictedDelete(filepath.Join(objectsDir, stagedMetadata.Hash)); err != nil {
			return fmt.Errorf("stageFile: cannot delete old file blob: %w", err)
		}
	}

	// file is not already staged or should be re-staged
	wdBlobFile := filepath.Join(objectsDir, wdHash)
	if err = writeContents(wdBlobFile, wdBlobPayload); err != nil {
		return fmt.Errorf("stageFile: could not write staged file blob: %w", err)
	}

	// update file index
	index[file] = indexMetadata{wdHash, time.Now().Unix(), int64(len(wdContents))}
	if err = writeIndex(index); err != nil {
		return fmt.Errorf("stageFile: could not update file index: %w", err)
	}
	return nil
}

func writeCommit(c commit) (string, error) {
	index, err := readIndex()
	if err != nil {
		return "", fmt.Errorf("writeCommit: %w", err)
	}
	if len(index) == 0 {
		log.Fatal("No changes added to commit.")
	}

	contents, err := serialize(c)
	if err != nil {
		return "", fmt.Errorf("writeCommit: could not serialize commit: %w", err)
	}
	payload := []any{"commit", []byte{blobHeaderDelim}, contents}
	commitHash, err := getHash(payload)
	if err != nil {
		return "", fmt.Errorf("writeCommit: could not create commit hash: %w", err)
	}
	if err := writeContents(filepath.Join(objectsDir, commitHash), payload); err != nil {
		return "", fmt.Errorf("writeCommit: cannot write commit blob: %w", err)
	}

	// set current branch head commit to new commit
	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return "", fmt.Errorf("writeCommit: %w", err)
	}
	if err := writeContents(currentBranchFile, []string{commitHash}); err != nil {
		return "", fmt.Errorf("writeCommit: cannot update current branch file: %w", err)
	}

	// clear index
	if err := newIndex(); err != nil {
		return "", fmt.Errorf("newCommit: cannot clear index: %w", err)
	}
	return commitHash, nil
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
		if metadata.Hash == stagedForRemovalMarker {
			// remove file from commit if it is staged for deletion
			delete(c.FileToBlob, file)
		} else {
			c.FileToBlob[file] = metadata.Hash
		}
	}

	if _, err := writeCommit(c); err != nil {
		return fmt.Errorf("newCommit: %w", err)
	}
	return nil
}

// unstageFile removes a file from the staging area if it is currently staged.
// If the file is also tracked in the current head commit, it will be staged for
// deletion and removed from the working directory if not already removed.
// Returns an error if the file is not staged or tracked by head commit.
func unstageFile(file string) error {
	index, err := readIndex()
	if err != nil {
		return fmt.Errorf("unstageFile: %w", err)
	}
	stagedMetadata, isStaged := index[file]

	// Unstage the file if it is currently staged for addition.
	if isStaged {
		if err := restrictedDelete(filepath.Join(objectsDir, stagedMetadata.Hash)); err != nil {
			return fmt.Errorf("unstageFile: %w", err)
		}
		delete(index, file)
		if err := writeIndex(index); err != nil {
			return fmt.Errorf("unstageFile: %w", err)
		}
	}

	headCommit, err := getHeadCommit()
	if err != nil {
		return fmt.Errorf("unstageFile: %w", err)
	}
	_, isTracked := headCommit.FileToBlob[file]
	if !isStaged && !isTracked {
		log.Fatal("No reason to remove the file.")
	}

	// Stage for deletion if the file is tracked in the head commit.
	if isTracked {
		// remove file from WD if present, do nothing if file does not exist
		if err := restrictedDelete(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("unstageFile: %w", err)
		}
		// stage for deletion (stage a deleted file)
		if err := stageFile(file); err != nil {
			return fmt.Errorf("unstageFile: %w", err)
		}
	}
	return nil
}

// printBranchLog prints the commit log from head of current branch to initial commit.
func printBranchLog() error {
	headCommitHash, err := getHeadCommitHash()
	if err != nil {
		return fmt.Errorf("printBranchLog: %w", err)
	}
	headCommit, err := getCommit(headCommitHash)
	if err != nil {
		return fmt.Errorf("printBranchLog: %w", err)
	}
	var curr = headCommit
	var currHash = headCommitHash
	for {
		log.Printf("===\n%v\n", curr.String(currHash))
		if curr.ParentUIDs[0] == "" {
			break
		}
		currHash = curr.ParentUIDs[0] // traverse up first parent
		if curr, err = getCommit(currHash); err != nil {
			return fmt.Errorf("printBranchLog: %w", err)
		}
	}
	return nil
}

// printAllCommits prints the log of all commits in any order.
func printAllCommits() error {
	if err := filepath.WalkDir(
		objectsDir,
		func(path string, d fs.DirEntry, err error) error {
			if d.IsDir() {
				return nil
			}
			c, c_err := getCommit(d.Name())
			if c_err != nil {
				return c_err
			}
			log.Printf("===\n%v\n", c.String(d.Name()))
			return err
		},
	); err != nil {
		return fmt.Errorf("printAllCommits: %w", err)
	}
	return nil
}

// printMatchingCommits prints all UIDs of commits with messages that contain a given substring query.
func printMatchingCommits(query string) error {
	hasMatch := false
	if err := filepath.WalkDir(
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
				hasMatch = true
				log.Printf("commit %v\n", d.Name())
			}
			return err
		},
	); err != nil {
		return fmt.Errorf("printMatchingCommits: %w", err)
	}
	if !hasMatch {
		log.Fatal("Found no commit with that message.")
	}
	return nil
}

// printStatus prints the current state of the repository.
func printStatus() error {
	log.Println("=== Branches ===")
	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return fmt.Errorf("printStatus: %w", err)
	}
	currentBranch := filepath.Base(currentBranchFile)
	branches, err := getFilenames(branchesDir)
	if err != nil {
		return fmt.Errorf("printStatus: %w", err)
	}
	slices.Sort(branches)
	for _, branch := range branches {
		if branch == currentBranch {
			log.Printf("*%v\n", branch)
		} else {
			log.Println(branch)
		}
	}

	index, err := readIndex()
	if err != nil {
		return fmt.Errorf("printStatus: %w", err)
	}
	var staged, removed []string
	for file, stagedMetadata := range index {
		if stagedMetadata.Hash == stagedForRemovalMarker {
			removed = append(removed, file)
		} else {
			staged = append(staged, file)
		}
	}

	log.Println("\n=== Staged Files ===")
	slices.Sort(staged)
	for _, file := range staged {
		log.Println(file)
	}

	log.Println("\n=== Removed Files ===")
	slices.Sort(removed)
	for _, file := range removed {
		log.Println(file)
	}

	log.Println("\n=== Modifications Not Staged For Commit ===")
	headCommit, err := getHeadCommit()
	if err != nil {
		return fmt.Errorf("printStatus: %w", err)
	}
	var unstagedChanges []string
	// check tracked files (deleted in WD, modified and unstaged in WD)
	for trackedFile, trackedHash := range headCommit.FileToBlob {
		_, isStaged := index[trackedFile]
		if isStaged {
			continue
		}
		contents, err := readContents(trackedFile)

		// check if deleted
		if errors.Is(err, fs.ErrNotExist) {
			unstagedChanges = append(unstagedChanges, fmt.Sprintf("%v (deleted)", trackedFile))
		} else if err != nil {
			return fmt.Errorf("printStatus: %w", err)
		}

		// check if modified
		payload := []any{"file", []byte{blobHeaderDelim}, contents}
		wdHash, err := getHash(payload)
		if err != nil {
			return fmt.Errorf("printStatus: %w", err)
		}
		if wdHash != trackedHash {
			unstagedChanges = append(unstagedChanges, fmt.Sprintf("%v (modified)", trackedFile))
		}
	}

	// check staged files (deleted in WD, modified in WD)
	// TODO: combine iteration with Staged and Removed sections
	for stagedFile, stagedMetadata := range index {
		// skip files staged for removal
		if stagedMetadata.Hash == stagedForRemovalMarker {
			continue
		}

		contents, err := readContents(stagedFile)
		// check if deleted
		if errors.Is(err, fs.ErrNotExist) {
			unstagedChanges = append(unstagedChanges, fmt.Sprintf("%v (deleted)", stagedFile))
		} else if err != nil {
			return fmt.Errorf("printStatus: %w", err)
		} else {
			// check if modified
			payload := []any{"file", []byte{blobHeaderDelim}, contents}
			wdHash, err := getHash(payload)
			if err != nil {
				return fmt.Errorf("printStatus: %w", err)
			}
			if wdHash != stagedMetadata.Hash {
				unstagedChanges = append(unstagedChanges, fmt.Sprintf("%v (modified)", stagedFile))
			}
		}
	}
	slices.Sort(unstagedChanges)
	for _, file := range unstagedChanges {
		log.Println(file)
	}

	log.Println("\n=== Untracked Files ===")
	var untracked []string
	// files in wd that are not tracked or staged
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("printStatus: %w", err)
	}
	wdFiles, err := getFilenames(cwd)
	if err != nil {
		return fmt.Errorf("printStatus: %w", err)
	}
	for _, file := range wdFiles {
		_, isStaged := index[file]
		_, isTracked := headCommit.FileToBlob[file]
		if !isStaged && !isTracked {
			untracked = append(untracked, file)
		}
	}
	slices.Sort(untracked)
	for _, file := range untracked {
		log.Println(file)
	}
	return nil
}

/*
checkoutHeadCommit pulls the file as it exists in the head commit into the working directory.
This command will create the file if it does not exist and overwrites the existing file if it does exist.
The new version of the file is not staged.
*/
func checkoutHeadCommit(file string) error {
	headCommitHash, err := getHeadCommitHash()
	if err != nil {
		return fmt.Errorf("checkoutHeadCommit: %w", err)
	}
	if err := checkoutCommit(file, headCommitHash); err != nil {
		return fmt.Errorf("checkoutHeadCommit: %w", err)
	}
	return nil
}

/*
checkoutCommit pulls the file as it exists in the commit into the working directory.
This command will create the file if it does not exist and overwrites the existing file if it does exist.
The new version of the file is not staged.
*/
func checkoutCommit(file string, targetCommitUID string) error {
	targetCommit, err := getCommit(targetCommitUID)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Fatal("No commit with that id exists.")
		}
		return fmt.Errorf("checkoutCommit: %w", err)
	}
	targetBlobHash, ok := targetCommit.FileToBlob[file]
	if !ok {
		log.Fatal("File does not exist in that commit.")
	}
	// read file contents from target commit
	_, contents, err := readBlob(targetBlobHash)
	if err != nil {
		return fmt.Errorf("checkoutCommit: %w", err)
	}
	// write file contents into working directory
	if err := writeContents(file, [][]byte{contents}); err != nil {
		return fmt.Errorf("checkoutCommit: %w", err)
	}
	return nil
}

/*
checkoutBranch switches the current branch to the target branch and pulls all files in
the head commit of the target branch into the working directory.

Files existing in the working directory are overwritten, and files that don't exist in
the working directory are created. Tracked files in the current branch that are not
present in the target branch are deleted, and the staging area is cleared.

Returns an error if the current branch is the target branch, the target branch does not
exist, or there is an untracked file that would be overwritten by the checkout.
*/
func checkoutBranch(targetBranch string) error {
	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return fmt.Errorf("checkoutBranch: %w", err)
	}
	currentBranch := filepath.Base(currentBranchFile)
	if targetBranch == currentBranch {
		log.Fatal("No need to checkout the current branch.")
	}
	targetBranchFile := filepath.Join(branchesDir, targetBranch)
	targetBranchHeadCommitHash, err := readContentsAsString(targetBranchFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Fatal("No such branch exists.")
		}
		return fmt.Errorf("checkoutBranch: %w", err)
	}
	targetBranchHeadCommit, err := getCommit(targetBranchHeadCommitHash)
	if err != nil {
		return fmt.Errorf("checkoutBranch: %w", err)
	}

	// check working directory for untracked files
	currentBranchHeadCommit, err := getHeadCommit()
	if err != nil {
		return fmt.Errorf("checkoutBranch: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("checkoutBranch: %w", err)
	}
	wdFiles, err := getFilenames(cwd)
	if err != nil {
		return fmt.Errorf("checkoutBranch: %w", err)
	}
	for _, file := range wdFiles {
		_, isTracked := currentBranchHeadCommit.FileToBlob[file]
		_, wouldBeOverwritten := targetBranchHeadCommit.FileToBlob[file]
		if !isTracked && wouldBeOverwritten {
			log.Fatal("There is an untracked file in the way; delete it, or add and commit it first.")
		}
	}

	// pull all files from target branch head commit into the working directory,
	// creating or overwriting as needed
	for file, targetBlobHash := range targetBranchHeadCommit.FileToBlob {
		_, contents, err := readBlob(targetBlobHash)
		if err != nil {
			return fmt.Errorf("checkoutBranch: %w", err)
		}
		if err := writeContents(file, [][]byte{contents}); err != nil {
			return fmt.Errorf("checkoutBranch: %w", err)
		}
	}

	// delete files in WD that are not target branch head commit
	for _, file := range wdFiles {
		_, ok := targetBranchHeadCommit.FileToBlob[file]
		if !ok {
			if err := restrictedDelete(file); err != nil {
				return fmt.Errorf("checkoutBranch: %w", err)
			}
		}
	}

	// set current branch to target branch
	if err = writeContents(headFile, []string{targetBranchFile}); err != nil {
		return fmt.Errorf("checkoutBranch: cannot set HEAD file: %w", err)
	}

	// clear staging area
	if err := newIndex(); err != nil {
		return fmt.Errorf("checkoutBranch: %w", err)
	}

	log.Printf("Branch '%v' is now checked out.\n", targetBranch)
	return nil
}

// addBranch creates a new branch pointing to the head commit of the current branch.
// This function does not checkout the new branch.
func addBranch(branchName string) error {
	branchFile := filepath.Join(branchesDir, branchName)
	if _, err := os.Stat(branchFile); err == nil {
		log.Fatal("A branch with that name already exists.")
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("addBranch: %w", err)
	}
	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return fmt.Errorf("addBranch: %w", err)
	}
	headCommitHash, err := readContents(currentBranchFile)
	if err != nil {
		return fmt.Errorf("addBranch: %w", err)
	}
	if err := writeContents(branchFile, [][]byte{headCommitHash}); err != nil {
		return fmt.Errorf("addBranch: %w", err)
	}
	log.Printf("Branch '%v' was created on commit (%v).\n", branchName, string(headCommitHash[:6]))
	return nil
}

// rm-branch
func removeBranch(branchName string) error {
	headBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return fmt.Errorf("removeBranch: %w", err)
	}
	if filepath.Base(headBranchFile) == branchName {
		log.Fatal("Cannot remove the current branch.")
	}

	if err := restrictedDelete(filepath.Join(branchesDir, branchName)); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Fatal("A branch with that name does not exist.")
		}
		return fmt.Errorf("removeBranch: %w", err)
	}
	log.Printf("Branch '%v' has been deleted.\n", branchName)
	return nil
}

// resetFile checks out all files tracked by the given commit
// and removes tracked files not present in that commit.
func resetFile(targetCommitUID string) error {
	targetCommit, err := getCommit(targetCommitUID)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Fatal("No commit with that id exists.")
		}
		return fmt.Errorf("resetFile: %w", err)
	}
	headCommit, err := getHeadCommit()
	if err != nil {
		return fmt.Errorf("resetFile: %w", err)
	}
	// check working directory for untracked files that would be overwritten
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resetFile: %w", err)
	}
	wdFiles, err := getFilenames(cwd)
	if err != nil {
		return fmt.Errorf("resetFile: %w", err)
	}
	for _, file := range wdFiles {
		_, isTracked := headCommit.FileToBlob[file]
		_, wouldBeOverwritten := targetCommit.FileToBlob[file]
		if !isTracked && wouldBeOverwritten {
			log.Fatal("There is an untracked file in the way; delete it, or add and commit it first.")
		}
	}

	// checkout every file from the target commit
	for file, targetBlobHash := range targetCommit.FileToBlob {
		_, contents, err := readBlob(targetBlobHash)
		if err != nil {
			return fmt.Errorf("resetFile: %w", err)
		}
		if err := writeContents(file, contents); err != nil {
			return fmt.Errorf("resetFile: %w", err)
		}
	}

	// delete files in WD that are not target commit
	for _, file := range wdFiles {
		_, ok := targetCommit.FileToBlob[file]
		if !ok {
			if err := restrictedDelete(file); err != nil {
				return fmt.Errorf("resetFile: %w", err)
			}
		}
	}

	// set current branch head commit to target commit
	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return fmt.Errorf("resetFile: %w", err)
	}
	if err = writeContents(currentBranchFile, []string{targetCommitUID}); err != nil {
		return fmt.Errorf("resetFile: cannot set HEAD commit: %w", err)
	}

	// clear staging area
	if err := newIndex(); err != nil {
		return fmt.Errorf("resetFile: %w", err)
	}
	return nil
}

// mergeBranch merges files from the given branch into the current branch.
func mergeBranch(branchName string) error {
	// check for uncommitted changes in staging area
	idx, err := readIndex()
	if err != nil {
		return fmt.Errorf("mergeBranch: %w", err)
	}
	if len(idx) != 0 {
		log.Fatal("You have uncommitted changes.")
	}

	// check target branch exists
	targetBranchFile := filepath.Join(branchesDir, branchName)
	targetBranchHeadCommitHash, err := readContentsAsString(targetBranchFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Fatal("A branch with that name does not exist.")
		}
		return fmt.Errorf("mergeBranch: %w", err)
	}

	// check current branch is not target branch
	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return fmt.Errorf("mergeBranch: %w", err)
	}
	currentBranch := filepath.Base(currentBranchFile)
	if branchName == currentBranch {
		log.Fatal("Cannot merge a branch with itself.")
	}

	targetBranchHeadCommit, err := getCommit(targetBranchHeadCommitHash)
	if err != nil {
		return fmt.Errorf("mergeBranch: %w", err)
	}
	currentBranchHeadCommit, err := getHeadCommit()
	if err != nil {
		return fmt.Errorf("mergeBranch: %w", err)
	}

	// check working directory for untracked files
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("mergeBranch: %w", err)
	}
	wdFiles, err := getFilenames(cwd)
	if err != nil {
		return fmt.Errorf("mergeBranch: %w", err)
	}
	// TODO: check tracked but modified WD files with not yet staged changes
	for _, file := range wdFiles {
		_, isTracked := currentBranchHeadCommit.FileToBlob[file]
		_, wouldBeOverwritten := targetBranchHeadCommit.FileToBlob[file]
		if !isTracked && wouldBeOverwritten {
			log.Fatal("There is an untracked file in the way; delete it, or add and commit it first.")
		}
	}
	currentBranchHeadCommitHash, err := getHeadCommitHash()
	if err != nil {
		return fmt.Errorf("mergeBranch: %w", err)
	}

	// find split point (latest common ancestor)
	splitPointCommitHash, err := findSplitPoint(currentBranchHeadCommitHash, targetBranchHeadCommitHash)
	if err != nil {
		return fmt.Errorf("mergeBranch: %w", err)
	}

	// check if split point same commit as given branch
	// merge is complete; do nothing
	if splitPointCommitHash == targetBranchHeadCommitHash {
		log.Println("Given branch is an ancestor of the current branch.")
		return nil
	}
	// check if split point is the current branch
	// checkout the target branch
	if splitPointCommitHash == currentBranchHeadCommitHash {
		if err := checkoutBranch(branchName); err != nil {
			return fmt.Errorf("mergeBranch: %w", err)
		}
		log.Println("Current branch fast-forwarded.")
		return nil
	}

	splitPointCommit, err := getCommit(splitPointCommitHash)
	if err != nil {
		return fmt.Errorf("mergeBranch: %w", err)
	}

	// all files: splitPoint, current, target, WD??
	allFiles := make(map[string]bool)
	for file := range splitPointCommit.FileToBlob {
		allFiles[file] = true
	}
	for file := range currentBranchHeadCommit.FileToBlob {
		allFiles[file] = true
	}
	for file := range targetBranchHeadCommit.FileToBlob {
		allFiles[file] = true
	}
	for file := range allFiles {
		targetHeadFileBlob, inTargetBranchHeadCommit := targetBranchHeadCommit.FileToBlob[file]
		currentHeadFileBlob, inCurrentBranchHeadCommit := currentBranchHeadCommit.FileToBlob[file]
		splitPointFileBlob, inSplitPointCommit := splitPointCommit.FileToBlob[file]

		// modified: file has been removed, changed, added since split point
		removedInCurrentBranch := (inSplitPointCommit && !inCurrentBranchHeadCommit)
		changedInCurrentBranch := (inSplitPointCommit && inCurrentBranchHeadCommit && (splitPointFileBlob != currentHeadFileBlob))
		addedInCurrentBranch := (!inSplitPointCommit && inCurrentBranchHeadCommit)
		modifiedInCurrentBranch := removedInCurrentBranch || changedInCurrentBranch || addedInCurrentBranch

		removedInTargetBranch := (inSplitPointCommit && !inTargetBranchHeadCommit)
		changedInTargetBranch := (inSplitPointCommit && inTargetBranchHeadCommit && (splitPointFileBlob != targetHeadFileBlob))
		addedInTargetBranch := (!inSplitPointCommit && inTargetBranchHeadCommit)
		modifiedInTargetBranch := removedInTargetBranch || changedInTargetBranch || addedInTargetBranch

		// 1) modified in target branch, unmodified in current branch
		if modifiedInTargetBranch && !modifiedInCurrentBranch {
			// checkout target branch version and stage
			if err := checkoutCommit(file, targetBranchHeadCommitHash); err != nil {
				return err
			}
			if err := stageFile(file); err != nil {
				return err
			}
			continue

			// 2) modified in current branch, unmodified in target branch
		} else if modifiedInCurrentBranch && !modifiedInTargetBranch {
			// keep current branch version
			continue
			// 3) modified in both current and target
		} else if modifiedInCurrentBranch && modifiedInTargetBranch {
			// both removed
			if removedInCurrentBranch && removedInTargetBranch {
				// the removed file can exist in WD, untracked and unstaged
				continue
			}
			if !removedInCurrentBranch && !removedInTargetBranch {
				// same hash
				if currentHeadFileBlob == targetHeadFileBlob {
					continue
				}
				// same contents
				_, currentBranchFileContents, err := readBlob(currentHeadFileBlob)
				if err != nil {
					return fmt.Errorf("mergeBranch: cannot read current file blob: %w", err)
				}
				_, targetBranchFileContents, err := readBlob(targetHeadFileBlob)
				if err != nil {
					return fmt.Errorf("mergeBranch: cannot read target file blob: %w", err)
				}
				if slices.Compare(currentBranchFileContents, targetBranchFileContents) == 0 {
					continue
				}
			}
		}

		// 4) not in split point, not in target branch, in current branch
		if !inSplitPointCommit && !inTargetBranchHeadCommit && inCurrentBranchHeadCommit {
			continue
		}

		// 5) not in split point, in target branch, not in current branch
		if !inSplitPointCommit && inTargetBranchHeadCommit && !inCurrentBranchHeadCommit {
			// checkout from target branch and stage
			if err := checkoutCommit(file, targetBranchHeadCommitHash); err != nil {
				return err
			}
			if err := stageFile(file); err != nil {
				return err
			}
			continue
		}

		// 6) in split point, unmodified in current branch, not in target branch
		if inSplitPointCommit && !modifiedInCurrentBranch && !inTargetBranchHeadCommit {
			// remove and untrack
			if err := unstageFile(file); err != nil {
				return fmt.Errorf("mergeBranch: %w", err)
			}
			continue
		}

		// 7) in split point, unmodified in target branch, not in current branch
		if inSplitPointCommit && !modifiedInTargetBranch && !inCurrentBranchHeadCommit {
			continue
		}

		// 8) files are in conflict, both modified
		if modifiedInCurrentBranch && modifiedInTargetBranch {
			var currentBranchFileContents, targetBranchFileContents []byte
			// contents are changed and different
			// contents of one are changed and other is deleted
			// file absent at split point and has different contents in target and current branches
			if !removedInCurrentBranch {
				_, currentBranchFileContents, err = readBlob(currentHeadFileBlob)
				if err != nil {
					return err
				}
			}
			if !removedInTargetBranch {
				_, targetBranchFileContents, err = readBlob(targetHeadFileBlob)
				if err != nil {
					return err
				}
			}
			if err := writeContents(file,
				[]any{
					"<<<<<<< HEAD\n",
					currentBranchFileContents,
					"=======",
					targetBranchFileContents,
					">>>>>>>",
				},
			); err != nil {
				return err
			}
			if err := stageFile(file); err != nil {
				return err
			}
			continue
		}
	}

	if err := newMergeCommit(
		branchName, targetBranchHeadCommitHash,
		currentBranch, currentBranchHeadCommitHash,
	); err != nil {
		return fmt.Errorf("mergeBranch: %w", err)
	}
	log.Print("Encountered a merge conflict.")
	return nil
}

// findSplitPoint finds the latest common ancestor given two commit UIDs.
//
// Uses BFS with map to record visited ancestors, breaking upon finding the earliest common one.
// Time: O(H), where H is the height of the DAG
// Space: O(H), recording every parent node upon visiting
func findSplitPoint(commitUID1 string, commitUID2 string) (string, error) {
	visited := make(map[string]bool)
	queue := []string{commitUID1, commitUID2}
	for len(queue) > 0 {
		commitUID := queue[0]
		if visited[commitUID] {
			return commitUID, nil
		}
		visited[commitUID] = true
		queue = queue[1:]
		c, err := getCommit(commitUID)
		if err != nil {
			return "", fmt.Errorf("findSplitPoint: %w", err)
		}
		for _, parentUID := range c.ParentUIDs {
			if parentUID != "" {
				queue = append(queue, parentUID)
			}
		}
	}
	return "", errors.New("findSplitPoint: no valid commit")
}

func newMergeCommit(
	targetBranch string,
	targetBranchHeadCommitHash string,
	currentBranch string,
	currentBranchHeadCommitHash string,
) error {
	c := commit{
		Message:    fmt.Sprintf("Merged %v into %v.", targetBranch, currentBranch),
		Timestamp:  time.Now().Unix(),
		FileToBlob: make(map[string]string),
		ParentUIDs: [2]string{currentBranchHeadCommitHash, targetBranchHeadCommitHash},
	}

	headCommit, err := getHeadCommit()
	if err != nil {
		return fmt.Errorf("newCommit: %w", err)
	}
	// create file to blob mapping from the previous commit
	for file, blobUID := range headCommit.FileToBlob {
		c.FileToBlob[file] = blobUID
	}
	// overwrite mapping with staged files
	index, err := readIndex()
	if err != nil {
		return err
	}
	for file, metadata := range index {
		if metadata.Hash == stagedForRemovalMarker {
			// remove file from commit if it is staged for deletion
			delete(c.FileToBlob, file)
		} else {
			c.FileToBlob[file] = metadata.Hash
		}
	}

	// write commit blob
	commitHash, err := writeCommit(c)
	if err != nil {
		return err
	}

	// set current branch head commit to new commit
	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return err
	}
	if err := writeContents(currentBranchFile, []string{commitHash}); err != nil {
		return fmt.Errorf("newCommit: cannot update current branch file: %w", err)
	}

	// clear index
	if err := newIndex(); err != nil {
		return fmt.Errorf("newCommit: cannot clear index: %w", err)
	}
	return nil
}

// addRemote adds a remote Gitlet repository reference.
//
// Example:
//
//	gitlet add-remote other ../testing/otherdir/.gitlet
func addRemote(remoteName string, remoteGitletDir string) error {
	remotes, err := readRemoteIndex()
	if err != nil {
		return fmt.Errorf("addRemote: %w", err)
	}
	_, ok := remotes[remoteName]
	if ok {
		log.Fatal("A remote with that name already exists.")
	}
	remotes[remoteName] = remoteMetadata{URL: filepath.FromSlash(remoteGitletDir)}
	remoteDir := filepath.Join(remotesDir, remoteName)
	if err := os.Mkdir(remoteDir, 0755); err != nil {
		return fmt.Errorf("addRemote: %w", err)
	}
	if err = writeRemoteIndex(remotes); err != nil {
		return fmt.Errorf("stageFile: could not update file index: %w", err)
	}
	return nil
}
	return nil
}
