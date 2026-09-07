package files

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash/crc32"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"chakuchuri/backend/internal/auth"
)

func photoFixture(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x*3 + y), uint8(x + y*7), uint8(x*11 + y*5), 255})
		}
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.NoCompression}
	if err := enc.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestPhotoUploadStoredAndServedAsSmallerWebP(t *testing.T) {
	raw := photoFixture(t, 320, 160)
	service := NewService(nil, nil, t.TempDir())
	actor := auth.User{Role: "Admin", Status: "Active"}
	file, header := testMultipartFile(t, "product.png", raw)
	record, err := service.Create(file, header, UploadRequest{OwnerType: "product_image"}, actor)
	if err != nil {
		t.Fatal(err)
	}
	if record.MimeType != "image/webp" || record.OriginalName != "product.webp" || record.Width != 320 || record.Height != 160 {
		t.Fatalf("incorrect optimized metadata: %+v", record)
	}
	stored, err := os.ReadFile(filepath.Join(service.storageDir, "uploads", filepath.FromSlash(record.StorageKey)))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(stored)
	if record.ByteSize != int64(len(stored)) || record.SourceBytes != int64(len(raw)) || record.Checksum != hex.EncodeToString(sum[:]) {
		t.Fatal("stored bytes and metadata disagree")
	}
	var thumb []byte
	if record.ThumbnailKey != "" {
		thumb, err = os.ReadFile(filepath.Join(service.storageDir, "uploads", filepath.FromSlash(record.ThumbnailKey)))
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(stored)+len(thumb) >= len(raw) {
		t.Fatal("fixture did not save storage including thumbnail")
	}
	if record.PreviewBytes != int64(len(thumb)) {
		t.Fatal("thumbnail storage accounting is incorrect")
	}
	t.Logf("original=%d stored=%d thumbnail=%d", len(raw), len(stored), len(thumb))
	response := httptest.NewRecorder()
	service.Serve(response, httptest.NewRequest("GET", record.URL, nil), record.ID, false)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "image/webp" || !bytes.Equal(response.Body.Bytes(), stored) {
		t.Fatal("WebP download did not match stored file")
	}
}

func TestPhotoOptimizationPreservesTransparency(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 128, 128))
	for y := 0; y < 128; y++ {
		for x := 32; x < 128; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 255, A: 128})
		}
	}
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.NoCompression}
	if err := enc.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	got, _, mime := optimizePhoto(buf.Bytes(), "transparent.png", "image/png", "product_image")
	if mime != "image/webp" {
		t.Fatal("fixture not converted")
	}
	decoded, _, err := image.Decode(bytes.NewReader(got))
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range []int{0, 64} {
		_, _, _, expected := img.At(x, 64).RGBA()
		_, _, _, actual := decoded.At(x, 64).RGBA()
		if actual != expected {
			t.Fatal("alpha channel changed")
		}
	}
}

func TestPhotoOptimizationPreservesAnimatedPNG(t *testing.T) {
	raw := photoFixture(t, 32, 32)
	chunk := make([]byte, 20)
	binary.BigEndian.PutUint32(chunk, 8)
	copy(chunk[4:], "acTL")
	binary.BigEndian.PutUint32(chunk[8:], 1)
	binary.BigEndian.PutUint32(chunk[16:], crc32.ChecksumIEEE(chunk[4:16]))
	animated := append(append(append([]byte{}, raw[:33]...), chunk...), raw[33:]...)
	got, name, mime := optimizePhoto(animated, "animated.png", "image/png", "message")
	if !bytes.Equal(got, animated) || name != "animated.png" || mime != "image/png" {
		t.Fatal("animation was flattened")
	}
}

func TestPhotoOptimizationNeverIncreasesFileSize(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	raw := buf.Bytes()
	got, _, _ := optimizePhoto(raw, "tiny.png", "image/png", "message")
	if len(got) > len(raw) {
		t.Fatal("conversion increased storage")
	}
	if _, ok := thumbnail(got, http.DetectContentType(got)); ok {
		t.Fatal("redundant thumbnail created for tiny image")
	}
}

func TestPhotoOptimizationPreservesExcludedUploads(t *testing.T) {
	raw := photoFixture(t, 100, 80)
	for _, owner := range []string{"customer_document", "payment_proof", "quotation_drawing", "quotation_reference", "general", "rate_sheet", "shipping_rate_book", "unknown"} {
		got, name, mime := optimizePhoto(raw, "original.png", "image/png", owner)
		if !bytes.Equal(got, raw) || name != "original.png" || mime != "image/png" {
			t.Fatalf("changed protected category %s", owner)
		}
	}
	for _, mime := range []string{"application/pdf", "image/webp", "text/plain"} {
		got, _, gotMIME := optimizePhoto(raw, "original", mime, "message")
		if !bytes.Equal(got, raw) || gotMIME != mime {
			t.Fatalf("changed excluded MIME %s", mime)
		}
	}
}

func TestPhotoOptimizationResizesWithoutUpscaling(t *testing.T) {
	for _, width := range []int{64, 2200} {
		got, _, mime := optimizePhoto(photoFixture(t, width, 80), "photo.png", "image/png", "message")
		if mime != "image/webp" {
			t.Fatal("fixture was not converted")
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(got))
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Width != min(width, 1600) || cfg.Height > 80 {
			t.Fatalf("unexpected dimensions %dx%d", cfg.Width, cfg.Height)
		}
	}
}

func TestPhotoPoliciesAndStorageEstimate(t *testing.T) {
	raw := photoFixture(t, 2200, 80)
	for _, tc := range []struct {
		owner string
		width int
	}{
		{"product_image", 2048}, {"quotation_photo", 2048}, {"directory_listing", 2048},
		{"message", 1600}, {"user_profile", 768},
	} {
		t.Run(tc.owner, func(t *testing.T) {
			estimate, err := EstimatePhotoStorage(raw, tc.owner)
			if err != nil || !estimate.Converted || estimate.Width != tc.width || estimate.SavedBytes <= 0 {
				t.Fatalf("unexpected estimate: %+v %v", estimate, err)
			}
			service := NewService(nil, nil, t.TempDir())
			service.EnablePersistence(t.TempDir())
			record, err := service.storeUpload(raw, "photo.png", "image/png", UploadRequest{OwnerType: tc.owner}, "tenant", "test")
			if err != nil {
				t.Fatal(err)
			}
			if record.SourceBytes != estimate.SourceBytes || record.ByteSize != estimate.StoredBytes || record.PreviewBytes != estimate.PreviewBytes {
				t.Fatalf("estimate differs from actual upload: %+v / %+v", estimate, record)
			}
			persisted, _, err := service.repository.LoadFiles()
			if err != nil || len(persisted) != 1 || persisted[0] != record {
				t.Fatalf("storage metrics lost on persistence: %+v %v", persisted, err)
			}
		})
	}
}

func TestStorageThumbnailCannotErasePhotoSavings(t *testing.T) {
	raw := photoFixture(t, 400, 300)
	if _, ok := storageThumbnail(raw, "image/png", int64(len(raw))); !ok {
		t.Fatal("fixture should have a preview")
	}
	if _, ok := storageThumbnail(raw, "image/png", int64(len(raw)+1)); ok {
		t.Fatal("preview erased the storage saving")
	}
	for _, raw := range [][]byte{nil, make([]byte, maxUploadBytes+1)} {
		if _, err := EstimatePhotoStorage(raw, "message"); err == nil {
			t.Fatal("invalid sample size accepted")
		}
	}
}

func TestPhotoOptimizationHandlesPhoneOrientation(t *testing.T) {
	img, _, err := image.Decode(bytes.NewReader(photoFixture(t, 120, 60)))
	if err != nil {
		t.Fatal(err)
	}
	var jpg bytes.Buffer
	if err := jpeg.Encode(&jpg, img, &jpeg.Options{Quality: 100}); err != nil {
		t.Fatal(err)
	}
	// TIFF orientation 6: rotate 90 degrees clockwise.
	exif := []byte{'E', 'x', 'i', 'f', 0, 0, 'I', 'I', 42, 0, 8, 0, 0, 0, 1, 0, 0x12, 1, 3, 0, 1, 0, 0, 0, 6, 0, 0, 0, 0, 0, 0, 0}
	segment := []byte{0xff, 0xe1, 0, byte(len(exif) + 2)}
	raw := append([]byte{0xff, 0xd8}, segment...)
	raw = append(raw, exif...)
	raw = append(raw, jpg.Bytes()[2:]...)
	got, _, mime := optimizePhoto(raw, "phone.jpg", "image/jpeg", "message")
	if mime != "image/webp" {
		t.Fatal("JPEG fixture was not converted")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(got))
	if err != nil || cfg.Width != 60 || cfg.Height != 120 {
		t.Fatalf("orientation lost: %+v %v", cfg, err)
	}
}

func TestPhotoOptimizationBoundsAndSaturation(t *testing.T) {
	raw := photoFixture(t, 32, 32)
	for i := 0; i < cap(imageProcessingSlots); i++ {
		imageProcessingSlots <- struct{}{}
	}
	defer func() {
		for i := 0; i < cap(imageProcessingSlots); i++ {
			<-imageProcessingSlots
		}
	}()
	got, _, _ := optimizePhoto(raw, "photo.png", "image/png", "message")
	if !bytes.Equal(got, raw) {
		t.Fatal("conversion ran despite saturation")
	}
	if _, ok := thumbnail(raw, "image/png"); ok {
		t.Fatal("thumbnail ran despite saturation")
	}
}

func TestPhotoOptimizationSkipsOversizedOrCorruptImages(t *testing.T) {
	raw := photoFixture(t, 1, 1)
	binary.BigEndian.PutUint32(raw[16:20], 100000)
	binary.BigEndian.PutUint32(raw[20:24], 100000)
	binary.BigEndian.PutUint32(raw[29:33], crc32.ChecksumIEEE(raw[12:29]))
	for _, input := range [][]byte{raw, []byte("broken image")} {
		got, _, _ := optimizePhoto(input, "photo.png", "image/png", "message")
		if !bytes.Equal(got, input) {
			t.Fatal("oversized/corrupt image changed")
		}
		if _, ok := thumbnail(input, "image/png"); ok {
			t.Fatal("unsafe image decoded for thumbnail")
		}
	}
}
