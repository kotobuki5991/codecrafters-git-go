package main

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"

	// Uncomment this block to pass the first stage!
	"os"

	"github.com/joho/godotenv"
	"github.com/labstack/gommon/log"
)

const (
	GIT_DIRE = ".git"
	GIT_OBJECT_DIRE = ".git/objects"
	GIT_REFS_DIRE = ".git/refs"
	GIT_API_PROTOCOL_SCHEME = "https://api.github.com"
	GIT_REPO_NAME = "codecrafters-io/git-starter-go"
)

// Usage: your_git.sh <command> <arg1> <arg2> ...
func main() {
	// .envファイルを読み込みます
	if err := godotenv.Load(); err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}
	GIT_OWNER_NAME := os.Getenv("GIT_OWNER_NAME")
	fmt.Println(GIT_OWNER_NAME)
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: mygit <command> [<args>...]\n")
		os.Exit(1)
	}

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
		treeHash := os.Args[3]
		lsTree(GIT_OWNER_NAME, treeHash)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %s\n", command)
		os.Exit(1)
	}
}

func catFile()  {
	sha := os.Args[len(os.Args)-1]
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

	split := strings.Split(buf.String(), "\000")
	blobBody := split[1]
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

func lsTree(gitOwnerName string, treeSha string) {
	// GET APIのurl
	url := fmt.Sprintf("%s/repos/%s/%s/git/trees/%s", GIT_API_PROTOCOL_SCHEME, gitOwnerName, GIT_REPO_NAME, treeSha)
	fmt.Println(url)
	// HTTPクライアントを作成
	client := &http.Client{}

	// HTTPリクエストを作成
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
			panic(err)
	}

	// Acceptヘッダーを設定
	req.Header.Set("Accept", "application/vnd.github+json")

	// リクエストを送信
	resp, err := client.Do(req)
	if err != nil {
			panic(err)
	}
	defer resp.Body.Close()

	// レスポンスを処理
	// ...

	fmt.Println(resp.Status)
}
