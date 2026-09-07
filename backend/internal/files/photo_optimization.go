package files

import (
	"bytes"
	"encoding/binary"
	"image"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/gen2brain/webp"
)

const maxPhotoPixels = 16_000_000
const maxPhotoSide = 2048

type photoPolicy struct {
	maxSide int
	quality int
}

func photoPolicyFor(ownerType string) (photoPolicy, bool) {
	switch strings.ToLower(strings.TrimSpace(ownerType)) {
	case "product_image", "quotation_photo", "directory_listing":
		return photoPolicy{maxSide: maxPhotoSide, quality: 82}, true
	case "message":
		return photoPolicy{maxSide: 1600, quality: 78}, true
	case "user_profile":
		return photoPolicy{maxSide: 768, quality: 80}, true
	default:
		return photoPolicy{}, false
	}
}

// Do not queue decoded images in memory during bursts. Saturation keeps originals.
var imageProcessingSlots = make(chan struct{}, 2)

func boundedImage(raw []byte) bool {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	return err == nil && cfg.Width > 0 && cfg.Height > 0 &&
		cfg.Width <= maxPhotoPixels/cfg.Height
}

func optimizePhoto(raw []byte, filename, mimeType, ownerType string) ([]byte, string, string) {
	policy, eligible := photoPolicyFor(ownerType)
	if !eligible {
		return raw, filename, mimeType
	}
	if (mimeType != "image/jpeg" && mimeType != "image/png") || !boundedImage(raw) || animatedPNG(raw) {
		return raw, filename, mimeType
	}
	select {
	case imageProcessingSlots <- struct{}{}:
		defer func() { <-imageProcessingSlots }()
	default:
		return raw, filename, mimeType
	}
	img, err := imaging.Decode(bytes.NewReader(raw), imaging.AutoOrientation(true))
	if err != nil {
		return raw, filename, mimeType
	}
	if img.Bounds().Dx() > policy.maxSide || img.Bounds().Dy() > policy.maxSide {
		img = imaging.Fit(img, policy.maxSide, policy.maxSide, imaging.Lanczos)
	}
	var encoded bytes.Buffer
	if err := webp.Encode(&encoded, img, webp.Options{Quality: policy.quality, Method: 3}); err != nil || encoded.Len() >= len(raw) {
		return raw, filename, mimeType
	}
	name := strings.TrimSuffix(filename, filepath.Ext(filename)) + ".webp"
	return encoded.Bytes(), name, "image/webp"
}

func storageThumbnail(raw []byte, mimeType string, sourceBytes int64) ([]byte, bool) {
	thumb, ok := thumbnail(raw, mimeType)
	if !ok || (int64(len(raw)) < sourceBytes && int64(len(raw)+len(thumb)) >= sourceBytes) {
		return nil, false
	}
	return thumb, true
}

type PhotoStorageEstimate struct {
	OwnerType    string `json:"ownerType"`
	SourceBytes  int64  `json:"sourceBytes"`
	StoredBytes  int64  `json:"storedBytes"`
	PreviewBytes int64  `json:"previewBytes"`
	SavedBytes   int64  `json:"savedBytes"`
	SourceMIME   string `json:"sourceMime"`
	StoredMIME   string `json:"storedMime"`
	Converted    bool   `json:"converted"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
}

// EstimatePhotoStorage runs the upload transforms in memory without persisting files.
func EstimatePhotoStorage(raw []byte, ownerType string) (PhotoStorageEstimate, error) {
	if len(raw) == 0 || len(raw) > maxUploadBytes {
		return PhotoStorageEstimate{}, errInvalidFile
	}
	sourceMIME := http.DetectContentType(raw)
	stored, _, mimeType := optimizePhoto(raw, "sample", sourceMIME, ownerType)
	thumb, _ := storageThumbnail(stored, mimeType, int64(len(raw)))
	width, height := imageSize(stored)
	return PhotoStorageEstimate{
		OwnerType: ownerType, SourceBytes: int64(len(raw)), StoredBytes: int64(len(stored)),
		PreviewBytes: int64(len(thumb)), SavedBytes: int64(len(raw) - len(stored) - len(thumb)),
		SourceMIME: sourceMIME, StoredMIME: mimeType, Converted: mimeType != sourceMIME,
		Width: width, Height: height,
	}, nil
}

// PNG's animation control chunk precedes image data. Never flatten animated uploads.
func animatedPNG(raw []byte) bool {
	if !bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")) {
		return false
	}
	for offset := 8; offset+12 <= len(raw); {
		size := uint64(binary.BigEndian.Uint32(raw[offset:]))
		end := uint64(offset) + size + 12
		if end > uint64(len(raw)) {
			return true
		}
		kind := string(raw[offset+4 : offset+8])
		if kind == "acTL" {
			return true
		}
		if kind == "IDAT" {
			return false
		}
		offset = int(end)
	}
	return false
}
