package main

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	// Uncomment this block to pass the first stage!
	"os"

	"github.com/codecrafters-io/git-starter-go/cmd/mygit/util"
)

const (
	GIT_DIRE = ".git"
	GIT_OBJECT_DIRE = ".git/objects"
	GIT_REFS_DIRE = ".git/refs"
)

// Usage: your_git.sh <command> <arg1> <arg2> ...
func main() {
	switch command := os.Args[1]; command {
	case "init":
		for _, dir := range []string{GIT_DIRE, GIT_OBJECT_DIRE, GIT_REFS_DIRE} {
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
		catFile()
	case "hash-object":
		hashObject()
	case "ls-tree":
		option := os.Args[2]
		if option == "--name-only" {
			treeHash := os.Args[3]
			lsTree(treeHash)
		}
	case "write-tree":
		writeTree()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}

func getTreeInfos(sha string) []string {
	blobPath := filepath.Join(GIT_OBJECT_DIRE, sha[:2], sha[2:])

	files, err := os.Open(blobPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error open file: %s\n", err)
	}
	defer files.Close()

	// io.ByteReaderを実装したReaderを生成
	br := bufio.NewReader(files)

	zr, err := zlib.NewReader(br)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error read file: %s\n", err)
	}
	defer zr.Close()


	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, zr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error copy buf file: %s\n", err)
	}

	treeInfo := strings.Split(buf.String(), "\000")
	return treeInfo
}

func catFile()  {
	sha := os.Args[len(os.Args)-1]
	blobBody := getTreeInfos(sha)[1]
	fmt.Print(blobBody)
}

func hashObject() {
	fileName := os.Args[len(os.Args)-1]

	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error read file: %s\n", err)
	}
	header := fmt.Sprintf("blob %d\x00", len(data))
	blobData := append([]byte(header), data...)

	// SHA-1ハッシュの計算
	sha := sha1.Sum(blobData)
	objectId := fmt.Sprintf("%x", sha)
	blobDire := string(objectId[:2])
	blobFile := string(objectId[2:])
	blobDirePath := filepath.Join(GIT_OBJECT_DIRE, blobDire)
	blobFilePath := filepath.Join(blobDirePath, blobFile)

	err = os.Mkdir(blobDirePath, 0755)
	if err != nil && !os.IsExist(err) {
		fmt.Fprintf(os.Stderr, "Failed mkdire: %s\n", err)
		return
	}

	outFile, err := os.Create(blobFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error create file: %s\n", err)
	}
	defer outFile.Close()

	zw := zlib.NewWriter(outFile)
	_, err = zw.Write(blobData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error write blob to file: %s\n", err)
	}
	zw.Close()

	fmt.Printf("%x", sha)
}

func lsTree(treeSha string) {
	treeInfos := getTreeInfos(treeSha)
	for _, v := range treeInfos {
		blobInfo := strings.Split(v, " ")
		if len(blobInfo) == 2 && blobInfo[0] != "tree" {
			dirName := blobInfo[1]
			fmt.Println(dirName)
		}
	}
}

func writeTree() {
	filePaths, err := util.GetFilePathsWalk()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error get filePaths: %s\n", err)
		os.Exit(1)
	}
	for _, v := range filePaths {
		fmt.Println(v)
	}
	// hashObject()
}
