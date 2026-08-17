package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewDesktopAboutInfoProjectsBuildMetadata(t *testing.T) {
	config := []byte(`{
		"info": {
			"productName": "  Skill Manager  ",
			"productVersion": "  0.5.0  ",
			"comments": "  Local manager for Claude Code and Codex skills  "
		}
	}`)
	icon := []byte("png data")

	about, err := newDesktopAboutInfo(config, icon)
	if err != nil {
		t.Fatalf("newDesktopAboutInfo() error = %v", err)
	}
	if about.Title != "Skill Manager" {
		t.Fatalf("Title = %q, want %q", about.Title, "Skill Manager")
	}
	wantMessage := "Version 0.5.0\n\nLocal manager for Claude Code and Codex skills"
	if about.Message != wantMessage {
		t.Fatalf("Message = %q, want %q", about.Message, wantMessage)
	}
	if !bytes.Equal(about.Icon, icon) {
		t.Fatalf("Icon = %q, want %q", about.Icon, icon)
	}
}

func TestNewDesktopAboutInfoRejectsInvalidMetadata(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		icon    []byte
		wantErr string
	}{
		{
			name:    "malformed config",
			config:  `{`,
			icon:    []byte("png data"),
			wantErr: "parse desktop build metadata",
		},
		{
			name:    "missing product name",
			config:  `{"info":{"productVersion":"0.5.0","comments":"Description"}}`,
			icon:    []byte("png data"),
			wantErr: "productName",
		},
		{
			name:    "missing product version",
			config:  `{"info":{"productName":"Skill Manager","comments":"Description"}}`,
			icon:    []byte("png data"),
			wantErr: "productVersion",
		},
		{
			name:    "missing product description",
			config:  `{"info":{"productName":"Skill Manager","productVersion":"0.5.0"}}`,
			icon:    []byte("png data"),
			wantErr: "comments",
		},
		{
			name:    "missing icon",
			config:  `{"info":{"productName":"Skill Manager","productVersion":"0.5.0","comments":"Description"}}`,
			wantErr: "application icon",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := newDesktopAboutInfo([]byte(test.config), test.icon)
			if err == nil || !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("newDesktopAboutInfo() error = %v, want error containing %q", err, test.wantErr)
			}
		})
	}
}

func TestEmbeddedDesktopAboutInfoIsComplete(t *testing.T) {
	about, err := newDesktopAboutInfo(desktopBuildMetadata, desktopApplicationIcon)
	if err != nil {
		t.Fatalf("newDesktopAboutInfo() embedded metadata error = %v", err)
	}
	if about.Title != "Skill Manager" {
		t.Fatalf("Title = %q, want %q", about.Title, "Skill Manager")
	}
	if !strings.HasPrefix(about.Message, "Version ") {
		t.Fatalf("Message = %q, want version prefix", about.Message)
	}
	if !strings.Contains(about.Message, "\n\nLocal manager for Claude Code and Codex skills") {
		t.Fatalf("Message = %q, want current product description", about.Message)
	}
}
