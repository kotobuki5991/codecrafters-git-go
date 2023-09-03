package util

import (
	"crypto/sha1"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func GetDireFilePathsWalk() ([]string, []string, error) {
	// カレントディレクトリを取得
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("カレントディレクトリの取得中にエラーが発生しました: %v\n", err)
		return nil, nil, err
	}

	// カレントディレクトリ以下のファイルパスを取得
	filePaths := []string{}
	direPath := []string{}
	err = filepath.Walk(currentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("ファイルパスの取得中にエラーが発生しました: %v\n", err)
			return nil
		}
		if !info.IsDir() {
			filePaths = append(filePaths, path)
		}
		direPath = append(direPath, path)
		return nil
	})
	if err != nil {
		fmt.Printf("ファイルパスの取得中にエラーが発生しました: %v\n", err)
		return nil, nil, err
	}
	return filePaths, direPath, nil
}

func GetHashByBlob(blob []byte) string {
	sha := sha1.Sum(blob)
	sha1Hex := fmt.Sprintf("%x", sha)
	return sha1Hex
}

func GetHashByFileName(fileName string) (string, error) {
	blobData, err := GetBlobDataByFileName(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error get blob: %s\n", err)
	}

	// SHA-1ハッシュの計算
	sha1Hex := GetHashByBlob(blobData)
	return sha1Hex, nil
}

func GetSha1ByFileName(fileName string) ([20]byte, error) {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error read file: %s\n", err)
	}
	header := fmt.Sprintf("blob %d\x00", len(data))
	blobData := append([]byte(header), data...)
	sha := sha1.Sum(blobData)
	// SHA-1ハッシュの計算
	return sha, nil
}

func GetBlobDataByFileName(fileName string)([]byte, error) {
	data, err := ioutil.ReadFile(fileName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error read file: %s\n", err)
		return nil, err
	}
	header := fmt.Sprintf("blob %d\x00", len(data))
	blobData := append([]byte(header), data...)
	return blobData, nil;
}


// ディレクトリのSHA-1ハッシュを計算する関数
func CalculateDirectoryHash(path string) (string, error) {
	// ディレクトリ内のすべてのファイルとディレクトリのパスを再帰的に取得
	var filePaths []string
	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error walk path: %s\n", err)
			return err
		}
		filePaths = append(filePaths, filePath)
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error walk path2: %s\n", err)
		return "", err
	}

	// ファイルパスをソート
	sort.Strings(filePaths)

	// ハッシュを計算
	h := sha1.New()
	for _, filePath := range filePaths {
		// ファイルの内容を読み込む
		fileData, err := ioutil.ReadFile(filePath)
		if err != nil {
			// fmt.Fprintf(os.Stderr, "Error walk path3: %s\n", err)
			continue
			// return "", err
		}
		// ファイルの内容をハッシュに追加
		h.Write(fileData)
	}
	// ハッシュを16進数文字列に変換して返す
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// ヘッダー情報のバイト数を計算する関数
func CalculateHeaderSize(entry string) (int, error) {
	// エントリを空白で分割
	parts := strings.Split(entry, " ")

	// モード、エントリ名、null文字を合計
	mode := parts[0]
	entryName := parts[1]
	nullCharacter := "\x00"
	headerInfo := mode + " " + entryName + nullCharacter

	// ヘッダー情報のバイト数を返す
	return len(headerInfo), nil
}
