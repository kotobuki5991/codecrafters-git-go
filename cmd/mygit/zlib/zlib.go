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

func CompressData(data []byte) ([]byte, error) {
	var compressedData bytes.Buffer
	zlibWriter := zlib.NewWriter(&compressedData)

	// データをzlibWriterに書き込み
	_, err := zlibWriter.Write(data)
	if err != nil {
		return nil, err
	}

	// zlibWriterを閉じて、バッファに圧縮データを書き込む
	zlibWriter.Close()

	// 圧縮されたデータを取得
	compressedBytes := compressedData.Bytes()
	return compressedBytes, nil
}
