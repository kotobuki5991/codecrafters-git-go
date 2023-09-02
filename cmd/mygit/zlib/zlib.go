package zlib

import (
	"bytes"
	"compress/zlib"
	"io/ioutil"
	"os"
	"path/filepath"
)

func CompressDire(dirPath string) ([]byte, error) {
	// ディレクトリ内のファイルを再帰的に収集します。
	var fileList []string
	err := filepath.Walk(dirPath, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
					return err
			}
			if !info.IsDir() {
					fileList = append(fileList, filePath)
			}
			return nil
	})
	if err != nil {
			return nil, err
	}

	// Zlibでディレクトリを圧縮します。
	var compressedBlobData bytes.Buffer
	zlibWriter := zlib.NewWriter(&compressedBlobData)
	for _, filePath := range fileList {
			fileData, err := ioutil.ReadFile(filePath)
			if err != nil {
					return nil, err
			}

			_, err = zlibWriter.Write(fileData)
			if err != nil {
					return nil, err
			}
	}
	zlibWriter.Close()

	return compressedBlobData.Bytes(), nil
}
