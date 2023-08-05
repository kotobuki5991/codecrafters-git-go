package main

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	// Uncomment this block to pass the first stage!
	"os"
)

// Usage: your_git.sh <command> <arg1> <arg2> ...
func main() {

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}

	switch command := os.Args[1]; command {
	case "init":
		for _, dir := range []string{".git", ".git/objects", ".git/refs"} {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintf(os.Stderr, "Error creating directory: %s\n", err)
			}
		}

		headFileContents := []byte("ref: refs/heads/master\n")
		if err := os.WriteFile(".git/HEAD", headFileContents, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
		}

		fmt.Println("Initialized git directory")

	case "cat-file":
		sha := os.Args[len(os.Args)-1]
		blobPath := filepath.Join(".git/objects", sha[:2], sha[2:])

		files, err := os.Open(blobPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
		}
		defer files.Close()

		// io.ByteReaderを実装したReaderを生成
		br := bufio.NewReader(files)

		zr, err := zlib.NewReader(br)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
		}
		defer zr.Close()


		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, zr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error writing file: %s\n", err)
		}

		split := strings.Split(buf.String(), "\000")
		blobBody := split[1]
		fmt.Print(blobBody)

	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}
