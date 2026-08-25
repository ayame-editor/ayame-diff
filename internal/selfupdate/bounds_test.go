package selfupdate

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
)

func TestReadBoundedRefusesOneByteTooMany(t *testing.T) {
	t.Parallel()

	body, err := readBounded(strings.NewReader("12345"), 5, "thing")
	if err != nil || string(body) != "12345" {
		t.Fatalf("body=%q err=%v", body, err)
	}
	if _, err := readBounded(strings.NewReader("123456"), 5, "thing"); err == nil {
		t.Fatal("a body over the limit was accepted")
	}
}

func TestDownloadRefusesAnOversizedBody(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		// No Content-Length: the limit must hold on the stream itself, which is
		// what a hostile server would exploit.
		w.Header().Set("Transfer-Encoding", "chunked")
		for written := 0; written < 4096; written += 64 {
			_, _ = w.Write(bytes.Repeat([]byte("x"), 64))
		}
	}))
	defer server.Close()

	if _, err := download(context.Background(), server.URL, 1024); err == nil {
		t.Fatal("an oversized download was accepted")
	}
	if _, err := download(context.Background(), server.URL, 1<<20); err != nil {
		t.Fatalf("a download inside the limit failed: %v", err)
	}
}

// A release archive that expands far beyond its download size is the shape of
// a decompression bomb. Extraction must refuse it on the declared size rather
// than allocate its way through it.
func TestExtractBinaryRefusesABomb(t *testing.T) {
	t.Parallel()

	name := binaryInArchive("v1.0.0")
	archive := bombArchive(t, name, maxBinaryBytes+1)
	_, err := extractBinary(archive, "v1.0.0")
	if err == nil {
		t.Fatal("an oversized entry was extracted")
	}
	if !strings.Contains(err.Error(), "byte limit") {
		t.Fatalf("the refusal does not name the limit: %v", err)
	}
}

func TestExtractBinaryStillReturnsARealBinary(t *testing.T) {
	t.Parallel()

	name := binaryInArchive("v1.0.0")
	want := []byte("#!/bin/sh\necho ayame\n")
	got, err := extractBinary(archiveWith(t, name, want), "v1.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("extracted %q, want %q", got, want)
	}
}

// archiveWith builds the archive shape this platform's release uses.
func archiveWith(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	if runtime.GOOS == "windows" {
		buffer := &bytes.Buffer{}
		zw := zip.NewWriter(buffer)
		entry, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write(content); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return buffer.Bytes()
	}
	buffer := &bytes.Buffer{}
	gz := gzip.NewWriter(buffer)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

// bombArchive declares an entry far larger than the limit while staying small
// on disk, so the test does not have to materialize the claimed size.
func bombArchive(t *testing.T, name string, declared int64) []byte {
	t.Helper()
	if runtime.GOOS == "windows" {
		// A zip's central directory carries the size, so patching a real entry
		// is unnecessary: write the header by hand with an inflated size.
		buffer := &bytes.Buffer{}
		zw := zip.NewWriter(buffer)
		header := &zip.FileHeader{Name: name, Method: zip.Store}
		header.UncompressedSize64 = uint64(declared)
		entry, err := zw.CreateRaw(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte("small")); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		return buffer.Bytes()
	}
	buffer := &bytes.Buffer{}
	gz := gzip.NewWriter(buffer)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: declared}); err != nil {
		t.Fatal(err)
	}
	// The declared size is checked before any of it is read, so the body can
	// stay empty; closing an incomplete entry is expected here.
	_ = tw.Flush()
	_ = gz.Close()
	return buffer.Bytes()
}

func TestExtractBinaryStopsAtTheEntryLimit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the zip path checks the entry count up front")
	}
	t.Parallel()

	buffer := &bytes.Buffer{}
	gz := gzip.NewWriter(buffer)
	tw := tar.NewWriter(gz)
	for i := 0; i < maxArchiveEntries+10; i++ {
		content := []byte("x")
		if err := tw.WriteHeader(&tar.Header{Name: fmt.Sprintf("padding-%d", i), Mode: 0o644, Size: int64(len(content))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(content); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	if _, err := extractBinary(buffer.Bytes(), "v1.0.0"); err == nil {
		t.Fatal("an archive of padding was scanned to the end without complaint")
	}
}
