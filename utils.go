package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
)

func getHash[T any](arr []T) (string, error) {
	h := sha1.New()
	for _, a := range arr {
		switch t := any(a).(type) {
		case []byte:
			_, err := h.Write(t)
			if err != nil {
				return "", fmt.Errorf("getHash[[]byte]]: %w", err)
			}
		case string:
			_, err := io.WriteString(h, t)
			if err != nil {
				return "", fmt.Errorf("getHash[string]: %w", err)
			}
		default:
			return "", fmt.Errorf("could not hash input: %v", t)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func restrictedDelete(file string) error {
	// check if file in dir that contains .gitlet
	_, err := os.Stat(filepath.Join(filepath.Dir(file), ".gitlet"))
	inGitletSubDir := slices.Contains(filepath.SplitList(file), ".gitlet")
	if errors.Is(err, os.ErrNotExist) && !inGitletSubDir {
		log.Fatal("Not in an initialized Gitlet directory: ", filepath.Dir(file))
	}
	fileInfo, err := os.Stat(file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("restrictedDelete: %w", err)
		} else {
			return err
		}
	}
	if fileInfo.IsDir() {
		return fmt.Errorf("restrictedDelete: cannot delete directory '%v'", file)
	}
	return os.Remove(file)
}

func readContentsToBytes(file string) ([]byte, error) {
	fileBytes, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("readContentsToBytes: %w", err)
	}
	return bytes.TrimRight(fileBytes, "\n"), nil
}

func readContentsToString(file string) (string, error) {
	fileBytes, err := readContentsToBytes(file)
	if err != nil {
		return "", fmt.Errorf("readContentsToString: %w", err)
	}
	return string(fileBytes), nil
}

// Write all contents of an array of strings or byte arrays to a file.
// If the file does not exist, it is created.
// Returns an error if the file is a directory.
func writeContents[T any](file string, arr []T) error {
	fileInfo, err := os.Stat(file)
	if (err != nil) && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("writeContents: %w", err)
	}
	if (err == nil) && fileInfo.IsDir() {
		return fmt.Errorf("writeContents: cannot overwrite directory '%v'", file)
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("writeContents: cannot open file '%v': %w", file, err)
	}
	for _, a := range arr {
		switch t := any(a).(type) {
		case string:
			if _, err := f.WriteString(t); err != nil {
				return err
			}
		case []byte:
			if _, err := f.Write(t); err != nil {
				return err
			}
		default:
			return fmt.Errorf("writeContents: %v is not an array of strings or byte arrays", t)
		}
	}
	_, err = f.WriteString("\n")
	if err != nil {
		return fmt.Errorf("writeContents: cannot write newline: %w", err)
	}
	return f.Close()
}

// Return a sorted list of filenames in the directory.
func getFilenames(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("getFilenames: %w", err)
	}
	var filenames []string
	for _, f := range files {
		if !f.IsDir() && f.Type().IsRegular() {
			filenames = append(filenames, f.Name())
		}
	}
	slices.Sort(filenames)
	return filenames, nil
}

// serialize object and return as byte array
func serialize[T any](obj T) ([]byte, error) {
	stream := bytes.Buffer{}
	enc := gob.NewEncoder(&stream)
	err := enc.Encode(obj)
	if err != nil {
		return nil, fmt.Errorf("serialize: %w", err)
	}
	return stream.Bytes(), nil
}

func deserialize[T any](b []byte) (T, error) {
	var output T
	stream := bytes.Buffer{}
	_, err := stream.Write(b)
	if err != nil {
		return output, fmt.Errorf("deserialize: write byte stream: %w", err)
	}
	dec := gob.NewDecoder(&stream)
	err = dec.Decode(&output)
	if err != nil {
		return output, fmt.Errorf("deserialize: decode byte stream: %w", err)
	}
	return output, nil
}

func createBlobFromFile(file string, prefix string) error {
	// read file contents
	contents, err := readContentsToBytes(file)
	if err != nil {
		return fmt.Errorf("createBlobFromFile: %w", err)
	}
	// get hash
	hash, err := getHash[any]([]any{prefix, contents})
	if err != nil {
		return fmt.Errorf("createBlobFromFile: %w", err)
	}
	// write to .gitlet/blob/
	blobPath := filepath.Join(filepath.Dir(file), ".gitlet", "blob", hash)
	return writeContents[[]byte](blobPath, [][]byte{contents})
}
