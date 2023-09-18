package cmd

import (
	"bufio"
	"bytes"
	"compress/zlib"
	"crypto/sha1"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
)

const (
	// ref: https://github.com/git/git/blob/830b4a04c45bf0a6db26defe02ed1f490acd18ee/Documentation/gitformat-pack.txt#L70-L79
	objCommit   = 1
	objTree     = 2
	objBlob     = 3
	objTag      = 4
	objOfsDelta = 6
	objRefDelta = 7

	msbMask      = uint8(0b10000000)
	remMask      = uint8(0b01111111)
	objMask      = uint8(0b01110000)
	firstRemMask = uint8(0b00001111)
)

var (
	// Fetched object. Map from sha1 to the object.
	shaToObj map[string]Object = make(map[string]Object)
)

type GitObjectReader struct {
	objectFileReader *bufio.Reader
	ContentSize      int64
	Type             string // "tree", "commit", "blob"
	Sha              string
}
type Object struct {
	Type byte // object type.
	Buf  []byte
}

func Clone(repoUrl string, cloneDir string) {
	// repoUrl := os.Args[2]
	// directory := os.Args[3]
	repoPath := path.Join(".", cloneDir)
	if err := os.MkdirAll(repoPath, 0750); err != nil {
		fmt.Errorf("error creating cloneDir: %s\n", err)
	}

	// log.Printf("[Debug] git url: %s, dir: %s\n", repoUrl, cloneDir)
	// status := initCmd(repoPath)
	// if status.err != nil {
	// 	return &Status{
	// 		exitCode: ExitCodeError,
	// 		err:      fmt.Errorf("error initializing git repository: %s\n", status.err),
	// 	}
	// }
	// latest commit: 7b8eb72b9dfa14a28ed22d7618b3cdecaa5d5be0
	commitSha, err := fetchLatestCommitHash(repoUrl)
	if err != nil {
		fmt.Errorf("error fetch latest commit hash: %s\n", err)
	}
	if err := writeBranchRefFile(repoPath, "master", commitSha); err != nil {
		fmt.Errorf("error write branch ref file: %s\n", err)
	}
	// Fetch objects.
	if err := fetchObjects(repoUrl, commitSha); err != nil {
		fmt.Errorf("error fetching objects: %s\n", err)
	}
	if err := writeFetchedObjects(repoPath); err != nil {
		fmt.Errorf("error writing fetched objects: %s\n", err)
	}
	// Restore files committed at the commit sha.
	if err := restoreRepository(repoPath, commitSha); err != nil {
		fmt.Errorf("error restoring repository: %s\n", err)
	}
}


func fetchLatestCommitHash(repositoryURL string) (string, error) {
	// $ curl 'https://github.com/taxintt/codecrafters-git-go/info/refs?service=git-upload-pack' --output -
	// 2023/06/27 23:40:54 SHA: 4b825dc642cb6eb9a060e54bf8d69288fbee4904
	// 001e# service=git-upload-pack
	// 0000
	// 0155 39065120688df73291eb9ec890bd5fd72e2bc9f1 HEADmulti_ack thin-pack side-band side-band-64k ofs-delta shallow deepen-since deepen-not deepen-relative no-progress include-tag multi_ack_detailed allow-tip-sha1-in-want allow-reachable-sha1-in-want no-done symref=HEAD:refs/heads/master filter object-format=sha1 agent=git/github-3b381533b78b
	// 003f 39065120688df73291eb9ec890bd5fd72e2bc9f1 refs/heads/master
	// 0000%
	resp, err := http.Get(fmt.Sprintf("%s/info/refs?service=git-upload-pack", repositoryURL))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	buf := bytes.NewBuffer([]byte{})
	if _, err := io.Copy(buf, resp.Body); err != nil {
		return "", err
	}
	reader := bufio.NewReader(buf)
	// read "001e# service=git-upload-pack\n"
	if _, err := readPacketLine(reader); err != nil {
		return "", err
	}
	// read "0000"
	if _, err := readPacketLine(reader); err != nil {
		return "", err
	}
	// read "0155 <commit sha> HEADmulti_ack..."
	// 0155 39065120688df73291eb9ec890bd5fd72e2bc9f1 HEADmulti_ack
	head, err := readPacketLine(reader)
	if err != nil {
		return "", err
	}
	// extract commit sha from head
	commitHash := strings.Split(string(head), " ")[0]
	return commitHash, nil
}

// read packet line sequentially from reader
func readPacketLine(reader io.Reader) ([]byte, error) {
	// e.g.) string(hex)=001e → size=30
	hex := make([]byte, 4)
	if _, err := reader.Read(hex); err != nil {
		return []byte{}, err
	}
	size, err := strconv.ParseInt(string(hex), 16, 64)
	if err != nil {
		return []byte{}, err
	}
	// Return immediately for "0000".
	if size == 0 {
		return []byte{}, nil
	}
	// read content and write to buf
	buf := make([]byte, size-4)
	if _, err := reader.Read(buf); err != nil {
		return []byte{}, err
	}
	return buf, nil
}

// write $repo/.git/refs/heads/<branch>
func writeBranchRefFile(repoPath string, branch string, commitSha string) error {
	refFilePath := path.Join(repoPath, ".git", "refs", "heads", branch)
	if err := os.MkdirAll(path.Dir(refFilePath), 0750); err != nil && !os.IsExist(err) {
		return err
	}
	refFileContent := []byte(commitSha)
	if err := ioutil.WriteFile(refFilePath, refFileContent, 0644); err != nil {
		return err
	}
	return nil
}


func fetchObjects(gitRepositoryURL, commitSha string) error {
	// do Reference discovery
	packfileBuf := fetchPackfile(gitRepositoryURL, commitSha)
	// parse packfile for debugging
	sign := packfileBuf[:4]
	version := binary.BigEndian.Uint32(packfileBuf[4:8])
	numObjects := binary.BigEndian.Uint32(packfileBuf[8:12])
	log.Printf("[Debug] packfile sign: %s\n", string(sign))
	log.Printf("[Debug] version: %d\n", version)
	log.Printf("[Debug] num objects: %d\n", numObjects)
	// verify checksum
	checksumLen := 20
	calculatedChecksum := packfileBuf[len(packfileBuf)-checksumLen:]
	storedChecksum := sha1.Sum(packfileBuf[:len(packfileBuf)-checksumLen])
	if !bytes.Equal(storedChecksum[:], calculatedChecksum) {
		log.Printf("[Error] expected checksum: %v, but got: %v", storedChecksum, calculatedChecksum)
	}
	// read objects from packfile except for header
	headerLen := 12
	bufReader := bytes.NewReader(packfileBuf[headerLen:])
	for {
		err := readObject(bufReader)
		if err != nil {
			return err
		}
		if bufReader.Len() <= checksumLen {
			log.Printf("[Debug] remaining buf len: %d\n", bufReader.Len())
			break
		}
	}
	return nil
}

func fetchPackfile(gitUrl, commitSha string) []byte {
	buf := bytes.NewBuffer([]byte{})
	// write no-progress for Packfile negotiation
	buf.WriteString(packetLine(fmt.Sprintf("want %s no-progress\n", commitSha)))
	buf.WriteString("0000")
	buf.WriteString(packetLine("done\n"))
	// do Packfile negotiation
	uploadPackUrl := fmt.Sprintf("%s/git-upload-pack", gitUrl)
	resp, err := http.Post(uploadPackUrl, "", buf)
	if err != nil {
		log.Fatalf("[Error] Error in git-upload-pack request: %v\n", err)
	}
	result := bytes.NewBuffer([]byte{})
	if _, err := io.Copy(result, resp.Body); err != nil {
		log.Fatal(err)
	}
	// log.Printf("[Debug] resp body: %v\n", result)
	packfileBuf := result.Bytes()[8:] // skip like "0031ACK\n"
	return packfileBuf
}

func packetLine(rawLine string) string {
	size := len(rawLine) + 4
	return fmt.Sprintf("%04x%s", size, rawLine)
}

// Read objects from packfile.
func readObject(reader *bytes.Reader) error {
	objType, objLen, err := readObjectTypeAndLen(reader)
	if err != nil {
		return err
	}
	if objType == objRefDelta {
		baseObjSha, err := readSha(reader)
		if err != nil {
			return err
		}
		baseObj, ok := shaToObj[baseObjSha]
		if !ok {
			return errors.New(fmt.Sprintf("Unknown obj sha: %s", baseObjSha))
		}
		decompressed, err := decompressObject(reader)
		if err != nil {
			return err
		}
		deltified, err := readDeltified(decompressed, &baseObj)
		if err != nil {
			return err
		}
		obj := Object{
			Type: baseObj.Type,
			Buf:  deltified.Bytes(),
		}
		if err := saveObj(&obj); err != nil {
			return err
		}
	} else if objType == objOfsDelta {
		// TODO : Implement this section
		return errors.New("Unsupported")
	} else {
		decompressed, err := decompressObject(reader)
		if err != nil {
			return err
		}
		if objLen != decompressed.Len() {
			return errors.New(fmt.Sprintf("Expected obj len: %d, but got: %d", objLen, decompressed.Len()))
		}
		obj := Object{
			Type: objType,
			Buf:  decompressed.Bytes(),
		}
		if err := saveObj(&obj); err != nil {
			return err
		}
	}
	return nil
}

func readSha(reader io.Reader) (string, error) {
	sha := make([]byte, 20)
	if _, err := reader.Read(sha); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha), nil
}

// Read objects. Update data.
func readObjectTypeAndLen(reader *bytes.Reader) (byte, int, error) {
	num := 0
	b, err := reader.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	objType := (b & objMask) >> 4
	num += int(b & firstRemMask)
	if (b & msbMask) == 0 {
		return objType, num, nil
	}
	i := 0
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return 0, 0, err
		}
		num += int(b) << (4 + 7*i)
		if (b & msbMask) == 0 {
			break
		}
		i++
	}
	// log.Printf("[Debug] varint num: %d\n", num)
	// log.Printf("[Debug] read data: %b\n", data[:i+1])
	return objType, num, nil
}

func decompressObject(reader *bytes.Reader) (*bytes.Buffer, error) {
	decompressedReader, err := zlib.NewReader(reader)
	if err != nil {
		return nil, err
	}
	decompressed := bytes.NewBuffer([]byte{})
	if _, err := io.Copy(decompressed, decompressedReader); err != nil {
		return nil, err
	}
	return decompressed, nil
}

// ref: https://git-scm.com/docs/pack-format#_deltified_representation
func readDeltified(reader *bytes.Buffer, baseObj *Object) (*bytes.Buffer, error) {
	// srcObjLen, err := binary.ReadUvarint(reader)
	_, err := binary.ReadUvarint(reader)
	if err != nil {
		return nil, err
	}
	// log.Printf("[Debug] base len: %d\n", srcObjLen)
	dstObjLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return nil, err
	}
	// log.Printf("[Debug] deltified len: %d\n", dstObjLen)
	result := bytes.NewBuffer([]byte{})
	for reader.Len() > 0 {
		firstByte, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		// log.Printf("[Debug] first byte: %b\n", firstByte)
		if (firstByte & msbMask) == 0 {
			// Add new data.
			n := int64(firstByte & remMask)
			if _, err := io.CopyN(result, reader, n); err != nil {
				return nil, err
			}
		} else { // msb == 1
			// Copy data.
			offset := 0
			size := 0
			// Check offset byte.
			for i := 0; i < 4; i++ {
				if (firstByte>>i)&1 > 0 { // i-bit is present.
					b, err := reader.ReadByte()
					if err != nil {
						return nil, err
					}
					offset += int(b) << (i * 8)
				}
			}
			// Check size byte.
			for i := 4; i < 7; i++ {
				if (firstByte>>i)&1 > 0 { // i-bit is present.
					b, err := reader.ReadByte()
					if err != nil {
						return nil, err
					}
					size += int(b) << ((i - 4) * 8)
				}
			}
			// log.Printf("[Debug] offset: %d\n", offset)
			// log.Printf("[Debug] size: %d\n", size)
			// log.Printf("[Debug] size: %b\n", size)
			if _, err := result.Write(baseObj.Buf[offset : offset+size]); err != nil {
				return nil, err
			}
		}
	}
	if result.Len() != int(dstObjLen) {
		return nil, errors.New(fmt.Sprintf("Invalid deltified buf: expected: %d, but got: %d", dstObjLen, result.Len()))
	}
	return result, nil
}
func saveObj(o *Object) error {
	objSha, err := o.sha()
	if err != nil {
		return err
	}
	shaToObj[objSha] = *o
	// log.Printf("[Debug] obj sha: %s\n", objSha)
	// log.Printf("[Debug] actual obj len: %d\n", len(o.Buf))
	return nil
}

func (o *Object) sha() (string, error) {
	b, err := o.wrappedBuf()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha1.Sum(b)), nil
}
// Write objects in shaToObj to .git/objects.
func writeFetchedObjects(repoPath string) error {
	for _, object := range shaToObj {
		b, err := object.wrappedBuf()
		if err != nil {
			return err
		}
		if _, err := writeGitObject(repoPath, b); err != nil {
			return err
		}
	}
	return nil
}

func (o *Object) wrappedBuf() ([]byte, error) {
	t, err := o.typeString()
	if err != nil {
		return []byte{}, err
	}
	wrappedBuf, err := wrapContent(o.Buf, t)
	if err != nil {
		return []byte{}, err
	}
	return wrappedBuf.Bytes(), nil
}

func (o *Object) typeString() (string, error) {
	switch o.Type {
	case objCommit:
		return "commit", nil
	case objTree:
		return "tree", nil
	case objBlob:
		return "blob", nil
	default:
		return "", errors.New(fmt.Sprintf("Invalid type: %d", o.Type))
	}
}

// Wrap content and returns a git object.
func wrapContent(contents []byte, objectType string) (*bytes.Buffer, error) {
	outerContents := bytes.NewBuffer([]byte{})
	outerContents.WriteString(fmt.Sprintf("%s %d\x00", objectType, len(contents)))
	if _, err := io.Copy(outerContents, bytes.NewReader(contents)); err != nil {
		return nil, err
	}
	return outerContents, nil
}

// Write the git object and return the sha1.
func writeGitObject(repoPath string, object []byte) (string, error) {
	blobSha := fmt.Sprintf("%x", sha1.Sum(object))
	// log.Printf("[Debug] object sha: %s\n", blobSha)
	objectFilePath := path.Join(repoPath, ".git", "objects", blobSha[:2], blobSha[2:])
	// log.Printf("[Debug] object file path: %s\n", objectFilePath)
	if err := os.MkdirAll(path.Dir(objectFilePath), 0755); err != nil {
		return "", err
	}
	objectFile, err := os.Create(objectFilePath)
	if err != nil {
		return "", err
	}
	compresssedFileWriter := zlib.NewWriter(objectFile)
	if _, err = compresssedFileWriter.Write(object); err != nil {
		return "", err
	}
	if err := compresssedFileWriter.Close(); err != nil {
		return "", err
	}
	return blobSha, nil
}

func restoreRepository(repoPath, commitSha string) error {
	// Parse commit and get tree sha.
	commitBuf, err := readObjectContent(repoPath, commitSha)
	if err != nil {
		return err
	}
	log.Printf("[Debug] latest commit sha: %s\n", commitSha)
	log.Printf("[Debug] latest commit buf: %s\n", string(commitBuf))
	commitReader := bufio.NewReader(bytes.NewReader(commitBuf))
	treePrefix, err := commitReader.ReadString(' ')
	if err != nil {
		return err
	}
	if treePrefix != "tree " {
		return errors.New(fmt.Sprintf("Invalid commit blob: %s", string(commitBuf)))
	}
	treeSha, err := commitReader.ReadString('\n')
	if err != nil {
		return err
	}
	treeSha = treeSha[:len(treeSha)-1] // Strip newline.
	// Traverse tree objects.
	if err := traverseTree(repoPath, "", treeSha); err != nil {
		return err
	}
	return nil
}

func readObjectContent(repoPath, objSha string) ([]byte, error) {
	objReader, err := NewGitObjectReader(repoPath, objSha)
	if err != nil {
		return []byte{}, err
	}
	contents, err := objReader.ReadContents()
	if err != nil {
		return []byte{}, err
	}
	return contents, nil
}

func (g *GitObjectReader) ReadContents() ([]byte, error) {
	contents := make([]byte, g.ContentSize)
	if _, err := io.ReadFull(g.objectFileReader, contents); err != nil {
		return []byte{}, err
	}
	return contents, nil
}

func NewGitObjectReader(repoPath, objectSha string) (GitObjectReader, error) {
	objectFilePath := path.Join(repoPath, ".git", "objects", objectSha[:2], objectSha[2:])
	objectFile, err := os.Open(objectFilePath)
	if err != nil {
		return GitObjectReader{}, err
	}
	objectFileDecompressed, err := zlib.NewReader(objectFile)
	if err != nil {
		return GitObjectReader{}, err
	}
	objectFileReader := bufio.NewReader(objectFileDecompressed)
	// Read the object type (includes the space character after).
	// e.g. tree for tree object.
	objectType, err := objectFileReader.ReadString(' ')
	if err != nil {
		return GitObjectReader{}, err
	}
	objectType = objectType[:len(objectType)-1] // Remove the trailing space character
	// Read the object size (includes the null byte after)
	// e.g. 100 as the ascii string.
	objectSizeStr, err := objectFileReader.ReadString(0)
	if err != nil {
		return GitObjectReader{}, err
	}
	objectSizeStr = objectSizeStr[:len(objectSizeStr)-1] // Remove the trailing null byte
	size, err := strconv.ParseInt(objectSizeStr, 10, 64)
	if err != nil {
		return GitObjectReader{}, err
	}
	return GitObjectReader{
		objectFileReader: objectFileReader,
		Type:             objectType,
		Sha:              objectSha,
		ContentSize:      size,
	}, nil
}

func traverseTree(repoPath, curDir, treeSha string) error {
	treeBuf, err := readObjectContent(repoPath, treeSha)
	if err != nil {
		return err
	}
	tree, err := parseTree(treeBuf)
	if err != nil {
		return err
	}
	log.Printf("[Debug] tree: %+v\n", tree)
	for _, child := range tree.children {
		if isBlob(child.mode) {
			// Create a file
			blobBuf, err := readObjectContent(repoPath, child.sha)
			if err != nil {
				return err
			}
			filePath := path.Join(repoPath, curDir, child.name)
			log.Printf("[Debug] write file: %s\n", filePath)
			if err := os.MkdirAll(path.Dir(filePath), 0750); err != nil && !os.IsExist(err) {
				return err
			}
			perm, err := getPerm(child.mode)
			if err != nil {
				return err
			}
			if err := ioutil.WriteFile(filePath, blobBuf, perm); err != nil {
				return err
			}
		} else {
			// traverse recursively.
			childDir := path.Join(curDir, child.name)
			if err := traverseTree(repoPath, childDir, child.sha); err != nil {
				return err
			}
		}
	}
	return nil
}

type TreeChild struct {
	mode string // 100XXX for blob, 40000 for tree.
	name string
	sha  string
}
type Tree struct {
	children []TreeChild
}

func parseTree(treeBuf []byte) (*Tree, error) {
	children := make([]TreeChild, 0)
	contentsReader := bufio.NewReader(bytes.NewReader(treeBuf))
	for {
		// Read the mode of the entry (including the space character after)
		mode, err := contentsReader.ReadString(' ')
		if err == io.EOF {
			break // We've reached the end of the file
		} else if err != nil {
			return nil, err
		}
		mode = mode[:len(mode)-1] // Trim the space suffix.
		// Read the name of the entry (including the null-byte character after)
		entryName, err := contentsReader.ReadString(0)
		if err != nil {
			return nil, err
		}
		entryName = entryName[:len(entryName)-1] // Trim the null-byte character suffix.
		sha := make([]byte, 20)
		_, err = contentsReader.Read(sha)
		if err != nil {
			return nil, err
		}
		children = append(children, TreeChild{
			name: entryName,
			mode: mode,
			sha:  fmt.Sprintf("%x", sha),
		})
	}
	tree := Tree{
		children: children,
	}
	return &tree, nil
}
func isBlob(mode string) bool {
	return strings.HasPrefix(mode, "100")
}
func getPerm(mode string) (os.FileMode, error) {
	if !isBlob(mode) {
		return 0, errors.New(fmt.Sprintf("Invalid mode: %s", mode))
	}
	perm, err := strconv.ParseInt(mode[3:], 8, 64)
	if err != nil {
		return 0, err
	}
	return os.FileMode(perm), nil
}
