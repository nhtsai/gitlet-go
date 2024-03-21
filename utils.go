package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
)

// getHash generates a 40-character SHA1 hash given an array of bytes and strings.
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

// restrictedDelete removes a file if the working directory contains a .gitlet directory.
// This function is used to safely delete user files within a Gitlet repository and
// should be called from the root directory of the Gitlet repository.
// Does nothing if file does not exist.
func restrictedDelete(file string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("restrictedDelete: %w", err)
	}
	_, err = os.Stat(filepath.Join(wd, gitletDir))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			log.Fatalf("Not in an initialized Gitlet repository.")
		}
		return fmt.Errorf("restrictedDelete: %w", err)
	}
	fileInfo, err := os.Stat(file)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return fmt.Errorf("restrictedDelete: %w", err)
	}
	if fileInfo.IsDir() {
		return fmt.Errorf("restrictedDelete: cannot delete directory '%v'", file)
	}
	if err := os.Remove(file); err != nil {
		return fmt.Errorf("restrictedDelete: %w", err)
	}
	return nil
}

// readContents returns the contents of a file as bytes.
func readContents(file string) ([]byte, error) {
	fileBytes, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("readContents: %w", err)
	}
	return bytes.TrimRight(fileBytes, "\n"), nil
}

// readContentsAsString returns the contents of a file as a string.
func readContentsAsString(file string) (string, error) {
	fileBytes, err := readContents(file)
	if err != nil {
		return "", fmt.Errorf("readContentsToString: %w", err)
	}
	return string(fileBytes), nil
}

// writeContents writes all contents of an array of strings or byte arrays to a file.
// If the file does not exist, it is created. If the file does exist, it is overwritten.
// Returns an error if the file is a directory.
func writeContents[T any](file string, arr []T) error {
	fileInfo, err := os.Stat(file)
	if (err != nil) && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("writeContents: %w", err)
	}
	if (err == nil) && fileInfo.IsDir() {
		return fmt.Errorf("writeContents: cannot overwrite directory '%v'", file)
	}
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("writeContents: cannot open file '%v': %w", file, err)
	}
	defer f.Close()
	for _, a := range arr {
		switch t := any(a).(type) {
		case string:
			if _, err := f.WriteString(t); err != nil {
				return fmt.Errorf("writeContents: cannot write string '%v': %w", t, err)
			}
		case []byte:
			if _, err := f.Write(t); err != nil {
				return fmt.Errorf("writeContents: cannot write bytes '%v': %w", string(t), err)
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

// getFilenames returns a sorted list of filenames in the directory.
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

// serialize encodes an object as bytes.
func serialize[T any](obj T) ([]byte, error) {
	b, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("serialize: %w", err)
	}
	return b, nil
}

// deserialize decodes bytes as an object.
func deserialize[T any](b []byte) (T, error) {
	var obj T
	if err := json.Unmarshal(b, &obj); err != nil {
		return obj, fmt.Errorf("deserialize: %w", err)
	}
	return obj, nil
}
