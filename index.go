package main

import (
	"fmt"
)

// Metadata for staged files.
type indexMetadata struct {
	Hash     string // Hash of the staged file blob.
	ModTime  int64  // Timestamp of staging.
	FileSize int64  // Size of file blob.
}

// Map between filename and staging metadata.
type indexMap map[string]indexMetadata

// Read the index file and return the index map object.
func readIndex() (indexMap, error) {
	indexData, err := readContents(indexFile)
	if err != nil {
		return nil, fmt.Errorf("readIndex: cannot read index file: %w", err)
	}
	index, err := deserialize[indexMap](indexData)
	if err != nil {
		return nil, fmt.Errorf("readIndex: %w", err)
	}
	return index, nil
}

// Write the index map object to the index file.
func writeIndex(i indexMap) error {
	indexData, err := serialize(i)
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
	if err := writeIndex(make(indexMap)); err != nil {
		return fmt.Errorf("newIndex: %w", err)
	}
	return nil
}
