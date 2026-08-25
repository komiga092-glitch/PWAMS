package handlers

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestValidateUploadContent(t *testing.T) {
	var pngContent bytes.Buffer
	if err := png.Encode(&pngContent, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		contentType string
		content     []byte
		wantErr     bool
	}{
		{name: "valid png", contentType: "image/png", content: pngContent.Bytes()},
		{name: "invalid png", contentType: "image/png", content: []byte("not an image"), wantErr: true},
		{name: "valid pdf signature", contentType: "application/pdf", content: []byte("%PDF-1.7")},
		{name: "invalid pdf", contentType: "application/pdf", content: []byte("plain text"), wantErr: true},
		{name: "valid webp signature", contentType: "image/webp", content: []byte("RIFF0000WEBP")},
		{name: "invalid webp", contentType: "image/webp", content: []byte("not webp"), wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateUploadContent(test.contentType, test.content)
			if (err != nil) != test.wantErr {
				t.Fatalf("validateUploadContent() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}

func TestSHA256Hex(t *testing.T) {
	if got := sha256Hex([]byte("PWAMS")); got != "4548941fc2b3b9fd0563ef0c029c4317276952d5b71130715f192c0a1d603cbd" {
		t.Fatalf("sha256Hex() = %s", got)
	}
}
