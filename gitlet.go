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

var (
	gitletDir      string = ".gitlet"
	objectsDir     string = filepath.Join(gitletDir, "objects")
	branchHeadsDir string = filepath.Join(gitletDir, "refs", "heads")
	remotesDir     string = filepath.Join(gitletDir, "remotes")
	headFile       string = filepath.Join(gitletDir, "HEAD")
	indexFile      string = filepath.Join(gitletDir, "INDEX")
)

const stagedForRemovalMarker string = "DELETED"

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
					index[file] = indexMetadata{stagedForRemovalMarker, time.Now().Unix(), 0}
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
		if metadata.Hash == stagedForRemovalMarker {
			// remove file from commit if it is staged for deletion
			delete(c.FileToBlob, file)
		} else {
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
		if err := os.Remove(filepath.Join(objectsDir, stagedMetadata.Hash)); err != nil {
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
		if err := os.Remove(file); err != nil && !errors.Is(err, fs.ErrNotExist) {
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
	headBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return fmt.Errorf("printBranchLog: %w", err)
	}
	headCommitHash, err := readContentsAsString(headBranchFile)
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
		fmt.Printf("===\n%v\n", curr.String(currHash))
		if curr.ParentUIDs[0] == "" {
			break
		}
		currHash = curr.ParentUIDs[0] // traverse up first parent
		curr, err = getCommit(currHash)
		if err != nil {
			return fmt.Errorf("printBranchLog: %w", err)
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

func printStatus() error {
	fmt.Println("=== Branches ===")
	currentBranchFile, err := readContentsAsString(headFile)
	if err != nil {
		return fmt.Errorf("printStatus: %w", err)
	}
	currentBranch := filepath.Base(currentBranchFile)
	branches, err := getFilenames(branchHeadsDir)
	if err != nil {
		return fmt.Errorf("printStatus: %w", err)
	}
	slices.Sort(branches)
	for _, branch := range branches {
		if branch == currentBranch {
			fmt.Print("*")
		}
		fmt.Println(branch)
	}

	index, err := readIndex()
	if err != nil {
		return fmt.Errorf("printStatus: %w", err)
	}
	var staged, removed []string

	for k, v := range index {
		if v.Hash == stagedForRemovalMarker {
			removed = append(removed, k)
		} else {
			staged = append(staged, k)
		}
	}

	fmt.Println("\n=== Staged Files ===")
	slices.Sort(staged)
	for _, file := range staged {
		fmt.Println(file)
	}

	fmt.Println("\n=== Removed Files ===")
	slices.Sort(removed)
	for _, file := range removed {
		fmt.Println(file)
	}

	fmt.Println("\n=== Modifications Not Staged For Commit ===")
	/*
		- tracked in current commit, changed in WD, but not staged
			- iterate through tracked, check index if not staged, check WD if modified
		- not staged for removal, but tracked in the current commit and deleted from the working directory.
			- iterate through tracked, check index if not staged, check WD if deleted
		- staged for addition, different contents (hash) than WD
			- iterate through index (non deletion), check WD if modified
		- staged for addition, deleted in WD
			- iterate through index (non deletion), check WD if deleted

	*/

	// files in head commit that are not in wd and not staged
	headCommit, err := getHeadCommit()
	if err != nil {
		return fmt.Errorf("printStatus: %w", err)
	}
	var unstagedChanges []string
	// check tracked but unstaged files
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
		} else {
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
	}

	// check staged files
	for stagedFile, stagedMetadata := range index {
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
		fmt.Println(file)
	}

	fmt.Println("\n=== Untracked Files ===")
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
		fmt.Println(file)

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
			log.Fatalf("No commit with that id exists.")
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
	targetBranchFile := filepath.Join(branchHeadsDir, targetBranch)
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

	// put all files from target branch head commit into the working directory,
	// creating or overwriting as needed
	for file, targetBlobHash := range targetBranchHeadCommit.FileToBlob {
		_, contents, err := readBlob(targetBlobHash)
		if err != nil {
			return fmt.Errorf("checkoutBranch: %w", err)
		}
		if err := writeContents(file, contents); err != nil {
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
	return nil
}

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
