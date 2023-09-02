package util

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
)

func GetFilePathsWalk() ([]string, error) {
	// カレントディレクトリを取得
	currentDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("カレントディレクトリの取得中にエラーが発生しました: %v\n", err)
		return nil, err
	}

	// カレントディレクトリ以下のファイルパスを取得
	filePaths := []string{}
	err = filepath.Walk(currentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("ファイルパスの取得中にエラーが発生しました: %v\n", err)
			return nil
		}
		if !info.IsDir() {
			filePaths = append(filePaths, path)
		}
		return nil
	})
	if err != nil {
		fmt.Printf("ファイルパスの取得中にエラーが発生しました: %v\n", err)
		return []string{}, err
	}
	return filePaths, nil
}

func GetHashByBlob(blob []byte) [20]byte {
	sha := sha1.Sum(blob)
	return sha
}
