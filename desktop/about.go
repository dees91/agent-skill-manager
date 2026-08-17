package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

type desktopConfig struct {
	Info struct {
		ProductName    string `json:"productName"`
		ProductVersion string `json:"productVersion"`
		Comments       string `json:"comments"`
	} `json:"info"`
}

func newDesktopAboutInfo(configJSON, icon []byte) (*mac.AboutInfo, error) {
	var config desktopConfig
	if err := json.Unmarshal(configJSON, &config); err != nil {
		return nil, fmt.Errorf("parse desktop build metadata: %w", err)
	}

	productName := strings.TrimSpace(config.Info.ProductName)
	if productName == "" {
		return nil, fmt.Errorf("desktop build metadata info.productName is required")
	}
	productVersion := strings.TrimSpace(config.Info.ProductVersion)
	if productVersion == "" {
		return nil, fmt.Errorf("desktop build metadata info.productVersion is required")
	}
	description := strings.TrimSpace(config.Info.Comments)
	if description == "" {
		return nil, fmt.Errorf("desktop build metadata info.comments is required")
	}
	if len(icon) == 0 {
		return nil, fmt.Errorf("desktop application icon is required")
	}

	return &mac.AboutInfo{
		Title:   productName,
		Message: fmt.Sprintf("Version %s\n\n%s", productVersion, description),
		Icon:    icon,
	}, nil
}
