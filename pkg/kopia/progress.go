package kopia

import (
	"sync"
	"sync/atomic"

	"github.com/kopia/kopia/snapshot/snapshotfs"
)

type kandoUploadProgress struct {
	snapshotfs.NullUploadProgress

	// all int64 must precede all int32 due to alignment requirements on ARM
	uploadedBytes int64
	cachedBytes   int64
	hashedBytes   int64

	cachedFiles       int32
	inProgressHashing int32
	hashedFiles       int32
	uploadedFiles     int32

	uploading      int32
	uploadFinished int32

	estimatedFileCount  int
	estimatedTotalBytes int64

	outputMutex sync.Mutex
}

func (p *kandoUploadProgress) UploadStarted() {
	*p = kandoUploadProgress{
		uploading: 1,
	}
}

// EstimatedDataSize implements Kopia UploadProgress
func (p *kandoUploadProgress) EstimatedDataSize(fileCount int, totalBytes int64) {
	p.outputMutex.Lock()
	defer p.outputMutex.Unlock()

	p.estimatedFileCount = fileCount
	p.estimatedTotalBytes = totalBytes
}

// UploadFinished implements UploadProgress
func (p *kandoUploadProgress) UploadFinished() {}

// HashedBytes implements UploadProgress
func (p *kandoUploadProgress) HashedBytes(numBytes int64) {
	atomic.AddInt64(&p.hashedBytes, numBytes)
	// output
}

// ExcludedFile implements Kopia UploadProgress
func (p *kandoUploadProgress) ExcludedFile(fname string, numBytes int64) {}

// ExcludedDir implements Kopia UploadProgress
func (p *kandoUploadProgress) ExcludedDir(dirname string) {}

// CachedFile implements Kopia UploadProgress
func (p *kandoUploadProgress) CachedFile(fname string, numBytes int64) {
	atomic.AddInt64(&p.cachedBytes, numBytes)
	atomic.AddInt32(&p.cachedFiles, 1)
	// output
}

// UploadedBytes implements Kopia UploadProgress
func (p *kandoUploadProgress) UploadedBytes(numBytes int64) {
	atomic.AddInt64(&p.uploadedBytes, numBytes)
	atomic.AddInt32(&p.uploadedFiles, 1)
	// output
}

// HashingFile implements Kopia UploadProgress
func (p *kandoUploadProgress) HashingFile(fname string) {
	atomic.AddInt32(&p.inProgressHashing, 1)
}

// FinishedHashingFile implements Kopia UploadProgress
func (p *kandoUploadProgress) FinishedHashingFile(fname string, numBytes int64) {
	atomic.AddInt32(&p.hashedFiles, 1)
	atomic.AddInt32(&p.inProgressHashing, -1)
	// output
}

// StartedDirectory implements Kopia UploadProgress
func (p *kandoUploadProgress) StartedDirectory(dirname string) {}

// FinishedDirectory implements Kopia UploadProgress
func (p *kandoUploadProgress) FinishedDirectory(dirname string) {}

// Error implements Kopia UploadProgress
func (p *kandoUploadProgress) Error(path string, err error, isIgnored bool) {}

var _ snapshotfs.UploadProgress = (*kandoUploadProgress)(nil)

func (p *kandoUploadProgress) GetStats() (hashed, cached, uploaded int64) {
	return p.hashedBytes, p.cachedBytes, p.uploadedBytes
}
