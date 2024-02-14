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
)

func getHashFromBytes(b []byte) (string, error) {
	h := sha1.New()
	_, err := h.Write(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func getHashFromString(s string) (string, error) {
	hash, err := getHashFromBytes([]byte(s))
	if err != nil {
		return "", err
	}
	return hash, nil
}

func getHash[T any](arr []T) (string, error) {
	h := sha1.New()
	for _, a := range arr {
		switch t := any(a).(type) {
		case []byte:
			_, err := h.Write(t)
			if err != nil {
				return "", err
			}
		case string:
			_, err := io.WriteString(h, t)
			if err != nil {
				return "", err
			}
		default:
			return "", fmt.Errorf("could not hash input: %v", t)
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func restrictedDelete(file string) bool {
	// check if file in dir that contains .gitlet
	_, err := os.Stat(filepath.Join(filepath.Dir(file), ".gitlet"))
	if errors.Is(err, os.ErrNotExist) {
		log.Fatal("Not in an initialized Gitlet directory:", filepath.Dir(file))
	}
	fileInfo, err := os.Stat(file)
	if errors.Is(err, fs.ErrNotExist) || fileInfo.IsDir() {
		return false
	}
	err = os.Remove(file)
	if err != nil {
		log.Fatal("Could not delete file:", file)
	}
	return true
}

func readContentsToBytes(file string) ([]byte, error) {
	fileBytes, err := os.ReadFile(file)
	if err != nil {
		return nil, nil
	}
	return fileBytes, nil
}

func readContentsToString(file string) (string, error) {
	fileBytes, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	return string(fileBytes), nil
}

func writeContents(file string, b []byte) error {
	fileInfo, err := os.Stat(file)
	if (err != nil) && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if (err == nil) && fileInfo.IsDir() {
		return errors.New("cannot overwrite directory")
	}
	return os.WriteFile(file, b, 0666)
}

func getFiles(dir string) []string {
	files, err := os.ReadDir(".")
	if err != nil {
		log.Fatal(err)
	}
	filenames := []string{}
	for _, f := range files {
		if !f.IsDir() && f.Type().IsRegular() {
			filenames = append(filenames, f.Name())
		}
	}
	return filenames
}

// serialize object and return as byte array
func serialize[T any](obj T) ([]byte, error) {
	stream := bytes.Buffer{}
	enc := gob.NewEncoder(&stream)
	err := enc.Encode(obj)
	if err != nil {
		return nil, err
	}
	return stream.Bytes(), nil
}

func deserialize[T any](b []byte) (T, error) {
	var output T
	stream := bytes.Buffer{}
	_, err := stream.Write(b)
	if err != nil {
		return output, err
	}
	dec := gob.NewDecoder(&stream)
	err = dec.Decode(&output)
	if err != nil {
		return output, err
	}
	return output, nil
}

func createBlobFromFile(file string) error {
	// read file contents
	contents, err := readContentsToBytes(file)
	if err != nil {
		return err
	}
	// get hash
	hash, err := getHashFromBytes(contents)
	if err != nil {
		return err
	}
	// write to .gitlet/blob/
	blobPath := filepath.Join(filepath.Dir(file), ".gitlet", "blob", hash)
	return writeContents(blobPath, contents)
}
