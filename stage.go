package main

import (
	"fmt"
)

// Metadata for staged files.
type stageMetadata struct {
	Hash     string
	ModTime  int64
	FileSize int64
}

// Map between filename and staging metadata.
type stagedFileMap map[string]stageMetadata

// Read the index file and return the index map object.
func readIndex() (stagedFileMap, error) {
	indexData, err := readContents(indexFile)
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
	if err = writeContents(indexFile, [][]byte{indexData}); err != nil {
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
