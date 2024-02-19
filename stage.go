package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// Metadata for staged files.
type stageMetadata struct {
	Hash     string
	ModTime  int64
	FileSize int64
}

// Map between filename and staging metadata.
type stagedFileMap map[string]stageMetadata

// Path to index file.
var indexFilePath string = filepath.Join(".gitlet", "INDEX")

// Read the index file and return the index map object.
func readIndex() (stagedFileMap, error) {
	indexData, err := readContentsToBytes(indexFilePath)
	if err != nil {
		return nil, fmt.Errorf("readIndex: cannot read index file: %w", err)
	}
	index, err := deserialize[stagedFileMap](indexData)
	if err != nil {
		return nil, fmt.Errorf("readIndex: %w", err)
	}
	return index, nil
}

// Write the index map object to the index file.
func writeIndex(i stagedFileMap) error {
	indexData, err := serialize[stagedFileMap](i)
	if err != nil {
		return fmt.Errorf("writeIndex: %w", err)
	}
	if err = writeContents(indexFilePath, [][]byte{indexData}); err != nil {
		return fmt.Errorf("writeIndex: %w", err)
	}
	return nil
}

// Clear the index file.
func newIndex() error {
	if err := writeIndex(make(stagedFileMap)); err != nil {
		return fmt.Errorf("newIndex: %w", err)
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
	if isStaged && (wdInfo.Size() == stagedInfo.FileSize) && (wdInfo.ModTime().Unix() == stagedInfo.ModTime) {
		log.Printf("File '%v' is already staged.\n", file)
		return nil
	}

	// compare hashes
	wdContents, err := readContentsToBytes(file)
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

// Removes a file from the staging area if it is currently staged.
// If file is tracked in current commit, stage for removal (deletion), remove file from working directory
// Returns an error if the file is not staged or tracked by head commit.
func unstageFile(file string) error {
	index, err := readIndex()
	if err != nil {
		return err
	}
	_, isStaged := index[file]

	currentBranchFile, err := readContentsToString(filepath.Join(".gitlet", "HEAD"))
	if err != nil {
		return err
	}
	headCommitHash, err := readContentsToString(currentBranchFile)
	if err != nil {
		return err
	}
	headCommitBytes, err := readContentsToBytes(filepath.Join(".gitlet", "objects", headCommitHash))
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
