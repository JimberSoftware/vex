package commands

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"hash"
	"os"
	"path/filepath"

	"github.com/jimbersoftware/vex/internal/vmp"
)

const uploadChunkSize = 1024 * 1024

type fileUpload struct {
	file        *os.File
	temporary   string
	destination string
	expected    uint64
	written     uint64
	checksum    []byte
	hash        hash.Hash
	mode        os.FileMode
}

func (u *fileUpload) start(req *vmp.UploadStartRequest) *vmp.Response {
	if u.file != nil {
		return uploadError("an upload is already in progress")
	}
	if !filepath.IsAbs(req.GetPath()) {
		return uploadError("upload path must be absolute")
	}
	if len(req.GetSha256()) != sha256.Size {
		return uploadError("sha256 checksum must be 32 bytes")
	}
	if req.GetMode()&^uint32(os.ModePerm) != 0 {
		return uploadError("file mode contains unsupported bits")
	}

	destination := filepath.Clean(req.GetPath())
	file, err := os.CreateTemp(filepath.Dir(destination), "."+filepath.Base(destination)+".vex-*")
	if err != nil {
		return uploadError(fmt.Sprintf("create temporary file: %v", err))
	}

	u.file = file
	u.temporary = file.Name()
	u.destination = destination
	u.expected = req.GetSize()
	u.checksum = append([]byte(nil), req.GetSha256()...)
	u.hash = sha256.New()
	u.mode = os.FileMode(req.GetMode())
	return uploadResponse(0)
}

func (u *fileUpload) write(req *vmp.UploadChunkRequest) *vmp.Response {
	if u.file == nil {
		return uploadError("no upload is in progress")
	}
	if len(req.GetData()) > uploadChunkSize {
		return uploadError("upload chunk exceeds 1 MiB")
	}
	if uint64(len(req.GetData())) > u.expected-u.written {
		return uploadError("upload exceeds declared size")
	}

	bytesWritten, err := u.file.Write(req.GetData())
	if err != nil {
		u.close()
		return uploadError(fmt.Sprintf("write upload: %v", err))
	}
	if bytesWritten != len(req.GetData()) {
		u.close()
		return uploadError("short write during upload")
	}
	_, _ = u.hash.Write(req.GetData())
	u.written += uint64(bytesWritten) //nolint:gosec // os.File.Write cannot return a negative byte count.
	return uploadResponse(u.written)
}

func (u *fileUpload) finish() *vmp.Response {
	if u.file == nil {
		return uploadError("no upload is in progress")
	}
	if u.written != u.expected {
		u.close()
		return uploadError(fmt.Sprintf("upload size mismatch: got %d, want %d", u.written, u.expected))
	}
	if !bytes.Equal(u.hash.Sum(nil), u.checksum) {
		u.close()
		return uploadError("upload checksum mismatch")
	}
	if err := u.file.Sync(); err != nil {
		u.close()
		return uploadError(fmt.Sprintf("sync upload: %v", err))
	}
	if err := u.file.Chmod(u.mode); err != nil {
		u.close()
		return uploadError(fmt.Sprintf("set upload mode: %v", err))
	}
	if err := u.file.Close(); err != nil {
		u.file = nil
		_ = os.Remove(u.temporary)
		return uploadError(fmt.Sprintf("close upload: %v", err))
	}
	u.file = nil
	if err := os.Rename(u.temporary, u.destination); err != nil {
		_ = os.Remove(u.temporary)
		return uploadError(fmt.Sprintf("publish upload: %v", err))
	}

	written := u.written
	u.temporary = ""
	u.destination = ""
	u.checksum = nil
	u.hash = nil
	u.expected = 0
	u.written = 0
	return uploadResponse(written)
}

func (u *fileUpload) close() {
	if u.file != nil {
		_ = u.file.Close()
		u.file = nil
	}
	if u.temporary != "" {
		_ = os.Remove(u.temporary)
		u.temporary = ""
	}
}

func uploadResponse(bytesWritten uint64) *vmp.Response {
	return &vmp.Response{Result: &vmp.Response_Upload{Upload: &vmp.UploadResponse{BytesWritten: bytesWritten}}}
}

func uploadError(message string) *vmp.Response {
	return &vmp.Response{Error: message}
}
