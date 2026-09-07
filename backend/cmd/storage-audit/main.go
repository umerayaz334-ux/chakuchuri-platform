package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"chakuchuri/backend/internal/files"
)

type extensionTotal struct {
	Extension string `json:"extension"`
	Files     int64  `json:"files"`
	Bytes     int64  `json:"bytes"`
}

type storageReport struct {
	Files          int64                       `json:"files"`
	TotalBytes     int64                       `json:"totalBytes"`
	UniqueContents int64                       `json:"uniqueContents"`
	UniqueBytes    int64                       `json:"uniqueBytes"`
	DuplicateFiles int64                       `json:"duplicateFiles"`
	DuplicateBytes int64                       `json:"duplicateBytes"`
	SkippedEntries int64                       `json:"skippedEntries"`
	ByExtension    []extensionTotal            `json:"byExtension"`
	Sample         *files.PhotoStorageEstimate `json:"sample,omitempty"`
}

func main() {
	root := flag.String("uploads-dir", "storage/uploads", "Directory to measure; never modified")
	sample := flag.String("sample-photo", "", "Optional photo to simulate a new upload in memory (max 10 MiB)")
	owner := flag.String("owner-type", "product_image", "Upload category for the sample simulation")
	flag.Parse()
	if err := run(*root, *sample, *owner, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "storage audit:", err)
		os.Exit(1)
	}
}

func run(root, sample, owner string, out io.Writer) error {
	report, err := auditStorage(root)
	if err != nil {
		return err
	}
	if sample != "" {
		estimate, err := estimateSample(sample, owner)
		if err != nil {
			return err
		}
		report.Sample = &estimate
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}

func auditStorage(root string) (storageReport, error) {
	report := storageReport{ByExtension: []extensionTotal{}}
	info, err := os.Lstat(root)
	if err != nil {
		return report, err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return report, fmt.Errorf("uploads directory must be a real directory")
	}
	type contentKey struct {
		hash [sha256.Size]byte
		size int64
	}
	seen := map[contentKey]bool{}
	extensions := map[string]extensionTotal{}
	buffer := make([]byte, 32*1024)
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if !entry.Type().IsRegular() {
			report.SkippedEntries++
			return nil
		}
		sum, size, err := hashFile(path, buffer)
		if err != nil {
			return fmt.Errorf("measure %q: %w", path, err)
		}
		report.Files++
		report.TotalBytes += size
		key := contentKey{sum, size}
		if seen[key] {
			report.DuplicateFiles++
			report.DuplicateBytes += size
		} else {
			seen[key] = true
			report.UniqueContents++
			report.UniqueBytes += size
		}
		ext := strings.ToLower(filepath.Ext(path))
		total := extensions[ext]
		total.Extension = ext
		total.Files++
		total.Bytes += size
		extensions[ext] = total
		return nil
	})
	if err != nil {
		return storageReport{}, err
	}
	for _, total := range extensions {
		report.ByExtension = append(report.ByExtension, total)
	}
	sort.Slice(report.ByExtension, func(i, j int) bool {
		return report.ByExtension[i].Extension < report.ByExtension[j].Extension
	})
	return report, nil
}

func hashFile(path string, buffer []byte) ([sha256.Size]byte, int64, error) {
	var sum [sha256.Size]byte
	f, err := os.Open(path)
	if err != nil {
		return sum, 0, err
	}
	defer f.Close()
	before, err := f.Stat()
	if err != nil {
		return sum, 0, err
	}
	if !before.Mode().IsRegular() {
		return sum, 0, fmt.Errorf("not a regular file")
	}
	h := sha256.New()
	size, err := io.CopyBuffer(h, f, buffer)
	if err != nil {
		return sum, 0, err
	}
	after, err := f.Stat()
	if err != nil {
		return sum, 0, err
	}
	if size != before.Size() || size != after.Size() || !before.ModTime().Equal(after.ModTime()) {
		return sum, 0, fmt.Errorf("file changed while reading; retry when uploads are idle")
	}
	copy(sum[:], h.Sum(nil))
	return sum, size, nil
}

func estimateSample(path, owner string) (files.PhotoStorageEstimate, error) {
	f, err := os.Open(path)
	if err != nil {
		return files.PhotoStorageEstimate{}, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return files.PhotoStorageEstimate{}, err
	}
	if !info.Mode().IsRegular() {
		return files.PhotoStorageEstimate{}, fmt.Errorf("sample must be a regular file")
	}
	raw, err := io.ReadAll(io.LimitReader(f, (10<<20)+1))
	if err != nil {
		return files.PhotoStorageEstimate{}, err
	}
	return files.EstimatePhotoStorage(raw, owner)
}
