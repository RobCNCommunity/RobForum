package store

import (
	"strings"
	"testing"
)

func TestValidatePostMedia(t *testing.T) {
	valid := PostMediaInput{
		StoredName: "0123456789abcdef.jpg",
		MIMEType:   "image/jpeg",
		Width:      1280,
		Height:     720,
		SizeBytes:  1024,
	}
	if err := validatePostMedia(valid); err != nil {
		t.Fatalf("valid media rejected: %v", err)
	}

	tests := []struct {
		name  string
		alter func(*PostMediaInput)
	}{
		{name: "path traversal", alter: func(item *PostMediaInput) { item.StoredName = "../image.jpg" }},
		{name: "unsupported mime", alter: func(item *PostMediaInput) { item.MIMEType = "image/svg+xml" }},
		{name: "zero width", alter: func(item *PostMediaInput) { item.Width = 0 }},
		{name: "oversized dimensions", alter: func(item *PostMediaInput) { item.Height = 9000 }},
		{name: "empty file", alter: func(item *PostMediaInput) { item.SizeBytes = 0 }},
		{name: "oversized file", alter: func(item *PostMediaInput) { item.SizeBytes = (5 << 20) + 1 }},
		{name: "long filename", alter: func(item *PostMediaInput) { item.StoredName = strings.Repeat("a", 252) + ".jpg" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			item := valid
			test.alter(&item)
			if err := validatePostMedia(item); err == nil {
				t.Fatal("invalid media accepted")
			}
		})
	}
}
