package main

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"
	"strings"

	// Uncomment this block to pass the first stage!
	"os"

	"github.com/codecrafters-io/git-starter-go/cmd/mygit/date"
	"github.com/codecrafters-io/git-starter-go/cmd/mygit/util"
	myzlib "github.com/codecrafters-io/git-starter-go/cmd/mygit/zlib"
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
		dirname := os.Args[len(os.Args)-1]
		if len(os.Args) < 3 {
			dirname = "."
		}
		objectname, err := writeTree(dirname, true)
		if err != nil {
			fmt.Fprintf(os.Stderr, "write-tree: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(objectname)
		// writeTree()
	case "commit-tree":
		treeSha := os.Args[2]
		commitMsg := os.Args[len(os.Args)-1]

		parentCommitSha := ""

		for i, v := range os.Args {
			if v == "-p" {
				parentCommitSha = os.Args[i+1]
			}
		}

		commitTree(treeSha, parentCommitSha, commitMsg)
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

	blobData, err := util.GetBlobDataByFileName(fileName)
	if err != nil && !os.IsExist(err) {
		fmt.Fprintf(os.Stderr, "Failed get blobdata: %s\n", err)
		return
	}

	// SHA-1ハッシュの計算
	sha1Hex, err := util.GetHashByFileName(fileName)
	if err != nil && !os.IsExist(err) {
		fmt.Fprintf(os.Stderr, "Failed get hash: %s\n", err)
		return
	}
	blobDire := string(sha1Hex[:2])
	blobFile := string(sha1Hex[2:])
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

	fmt.Printf(sha1Hex)
}

func HashObject(filepath string, write bool) (objectname string, err error) {
	// read file once to both compute sha-1 sum and compress using zlib
	// with larger files, using os.Open might be better
	contents, err := ioutil.ReadFile(filepath)
	if err != nil {
		return "", err
	}
	// https://alblue.bandlem.com/2011/08/git-tip-of-week-objects.html
	prefix := fmt.Sprintf("blob %d\x00", len(contents))
	data := append([]byte(prefix), contents...)
	hasher := sha1.New()
	hasher.Write(data)
	objectname = hex.EncodeToString(hasher.Sum(nil))
	compressed := bytes.NewBuffer(make([]byte, 0))
	zlibWriter := zlib.NewWriter(compressed)
	if _, err := zlibWriter.Write(data); err != nil {
		return "", err
	}
	if err := zlibWriter.Close(); err != nil {
		return "", err
	}

	if write {
		err := WriteObject(objectname, compressed.Bytes())
		if err != nil {
			return objectname, nil
		}
	}
	return objectname, nil
}

func lsTree(treeSha string) {
	treeInfos := getTreeInfos(treeSha)
	for _, v := range treeInfos {
		blobInfo := strings.Split(v, " ")
		// fmt.Println(blobInfo)
		if len(blobInfo) == 2 && blobInfo[0] != "tree" {
			dirName := blobInfo[1]
			fmt.Println(dirName)
		}
	}
}

// func writeTree() {
// 	modeFile := 100644
// 	modeDir := 40000
// 	treeDatasBlob := []byte{}
// 	treeSize := 0
// 	stagedFilePaths, stagedDirePaths, err := util.GetDireFilePathsWalk()
// 	if err != nil {
// 		fmt.Fprintf(os.Stderr, "Error file paths walk: %s\n", err)
// 		os.Exit(1)
// 	}
// 	for _, v := range stagedFilePaths {
// 		// fmt.Println(v)
// 		filePathsSplit := strings.Split(v, "/")
// 		fileName := filePathsSplit[len(filePathsSplit)-2]
// 		if strings.Contains(v, ".git") {
// 			continue
// 		}
// 		fileHash, err := util.GetHashByFileName(v)
// 		if err != nil {
// 			fmt.Fprintf(os.Stderr, "Error get file hash: %s\n", err)
// 			os.Exit(1)
// 		}
// 		treeDataStr := fmt.Sprintf("%d %s\u0000 %s", modeFile, fileName, fileHash)
// 		// fmt.Println(treeDataStr)
// 		treeDatasBlob = append(treeDatasBlob, []byte(treeDataStr)...)
// 		treeDatasBlob = append(treeDatasBlob, '\n')
// 		size, err := util.CalculateHeaderSize(treeDataStr)
// 		if err != nil {
// 			fmt.Fprintf(os.Stderr, "Error get header size: %s\n", err)
// 			os.Exit(1)
// 		}
// 		treeSize += size
// 	}
// 	for _, v := range stagedDirePaths {
// 		dirPathsSplit := strings.Split(v, "/")
// 		dirName := dirPathsSplit[len(dirPathsSplit)-1]
// 		if strings.Contains(v, ".git") {
// 			continue
// 		}
// 		dirHash, err := util.CalculateDirectoryHash(v)
// 		if err != nil {
// 			fmt.Fprintf(os.Stderr, "Error get dire hash: %s\n", err)
// 			os.Exit(1)
// 		}
// 		treeDataStr := fmt.Sprintf("%d %s\u0000 %s", modeDir, dirName, dirHash)
// 		// fmt.Println(treeDataStr)
// 		treeDatasBlob = append(treeDatasBlob, []byte(treeDataStr)...)
// 		treeDatasBlob = append(treeDatasBlob, '\n')
// 		size, err := util.CalculateHeaderSize(treeDataStr)
// 		if err != nil {
// 			fmt.Fprintf(os.Stderr, "Error get header size: %s\n", err)
// 			os.Exit(1)
// 		}
// 		treeSize += size
// 	}

// 	treeHeader := fmt.Sprintf("tree %d\\0\n", treeSize)
// 	treeHeaderByte := []byte(treeHeader)
// 	treeDatasBlob = append(treeHeaderByte, treeDatasBlob...)

// 	fmt.Println(util.GetHashByBlob(treeDatasBlob))

// 	// direZlibBlob, err := myzlib.CompressDire("/tmp")
// 	// if err != nil {
// 	// 	fmt.Fprintf(os.Stderr, "Error compress dir: %s\n", err)
// 	// 	os.Exit(1)
// 	// }
// 	// fmt.Println(util.GetHashByBlob(direZlibBlob))
// 	hashObject()
// }

func WriteObject(objectname string, contents []byte) error {
	dirname, filename := objectname[:2], objectname[2:]
	err := os.MkdirAll(fmt.Sprintf(".git/objects/%s", dirname), 0755)
	if err != nil {
		return err
	}
	err = os.WriteFile(fmt.Sprintf(".git/objects/%s/%s", dirname, filename), contents, 0644)
	if err != nil {
		return err
	}
	return nil
}

func writeTree(dir string, write bool) (objectname string, err error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return "", err
	}
	tree := bytes.NewBuffer(nil)
	for _, file := range files {
		path := filepath.Join(dir, file.Name())
		if file.IsDir() {
			if file.Name()[0] == '.' {
				continue
			}
			objectname, err := writeTree(path, write)
			if err != nil {
				return "", nil
			}
			hexName, err := hex.DecodeString(objectname)
			if err != nil {
				return "", nil
			}
			fmt.Fprintf(tree, "40000 %s\x00%s", file.Name(), hexName)
			continue
		}
		objectname, err := HashObject(path, false)
		if err != nil {
			return "", err
		}
		hexName, err := hex.DecodeString(objectname)
		if err != nil {
			return "", nil
		}
		fmt.Fprintf(tree, "100644 %s\x00%s", file.Name(), hexName)
	}
	prefix := fmt.Sprintf("tree %d\x00", tree.Len())
	data := append([]byte(prefix), tree.Bytes()...)
	hasher := sha1.New()
	hasher.Write(data)
	objectname = hex.EncodeToString(hasher.Sum(nil))
	compressed := bytes.NewBuffer(make([]byte, 0))
	zlibWriter := zlib.NewWriter(compressed)
	if _, err := zlibWriter.Write(data); err != nil {
		return "", err
	}
	if err := zlibWriter.Close(); err != nil {
		return "", err
	}
	if write {
		err := WriteObject(objectname, compressed.Bytes())
		if err != nil {
			return objectname, err
		}
	}
	return objectname, nil
}

func commitTree(treeSha string, parentCommitSha string, commitMsg string) {
	committer := "committer"
	mail := "hoge@gmail.com"
	commitDate := date.FormatNowTimezoneOffset()

	treeInfo := fmt.Sprintf("tree %s\n", treeSha)
	parentInfo := fmt.Sprintf("parent %s\n", parentCommitSha)
	authorInfo := fmt.Sprintf("author %s <%s> %s\n", committer, mail, commitDate)
	committerInfo := fmt.Sprintf("committer %s <%s> %s\n\n", committer, mail, commitDate)
	commitMsgInfo := fmt.Sprintf("%s\n", commitMsg)

	commitObjContent := []byte{}
	commitObjContent = append(commitObjContent, []byte(treeInfo)...)
	if parentCommitSha != "" {
		commitObjContent = append(commitObjContent, []byte(parentInfo)...)
	}
	commitObjContent = append(commitObjContent, []byte(authorInfo)...)
	commitObjContent = append(commitObjContent, []byte(committerInfo)...)
	commitObjContent = append(commitObjContent, []byte(commitMsgInfo)...)

	commitHeader := fmt.Sprintf("commit %d\x00", len(commitObjContent))
	data := append([]byte(commitHeader), commitObjContent...)
	zlibContent, err := myzlib.CompressData(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error compress content", err)
	}

	sha1Hex := util.GetHashByBlob(zlibContent)

	WriteObject(sha1Hex, zlibContent)

	fmt.Println(sha1Hex)
}
