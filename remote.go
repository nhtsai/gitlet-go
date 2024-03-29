package main

import "fmt"

type remoteMetadata struct {
	URL string
}

type remoteIndex map[string]remoteMetadata

// Read the index file and return the index map object.
func readRemoteIndex() (remoteIndex, error) {
	remoteIndexData, err := readContents(remoteFile)
	if err != nil {
		return nil, fmt.Errorf("readRemoteIndex: cannot read index file: %w", err)
	}
	index, err := deserialize[remoteIndex](remoteIndexData)
	if err != nil {
		return nil, fmt.Errorf("readRemoteIndex: %w", err)
	}
	return index, nil
}

func writeRemoteIndex(r remoteIndex) error {
	remoteIndexData, err := serialize(r)
	if err != nil {
		return fmt.Errorf("writeRemoteIndex: %w", err)
	}
	if err = writeContents(indexFile, [][]byte{remoteIndexData}); err != nil {
		return fmt.Errorf("writeRemoteIndex: %w", err)
	}
	return nil
}
