// Package validators provides asset validation for QA session results
package validators

import (
	"fmt"
	"path/filepath"
	"strings"
)

// AssetType represents the type of QA session asset
type AssetType string

const (
	AssetTypeText   AssetType = "text"
	AssetTypeVideo  AssetType = "video"
	AssetTypeImage  AssetType = "image"
	AssetTypeJSON   AssetType = "json"
	AssetTypeYAML   AssetType = "yaml"
	AssetTypeBinary AssetType = "binary"
	AssetTypeUnknown AssetType = "unknown"
)

// TextSubtype represents subtypes of text files
type TextSubtype string

const (
	TextSubtypeLog      TextSubtype = "log"
	TextSubtypeReport   TextSubtype = "report"
	TextSubtypeMarkdown TextSubtype = "markdown"
	TextSubtypePlain    TextSubtype = "plain"
	TextSubtypeCSV      TextSubtype = "csv"
	TextSubtypeXML      TextSubtype = "xml"
)

// ValidationResult contains the result of asset validation
type ValidationResult struct {
	AssetPath   string      `json:"asset_path"`
	AssetType   AssetType   `json:"asset_type"`
	TextSubtype TextSubtype `json:"text_subtype,omitempty"`
	IsValid     bool        `json:"is_valid"`
	Errors      []string    `json:"errors,omitempty"`
	Warnings    []string    `json:"warnings,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Validator is the interface for asset validators
type Validator interface {
	Validate(path string) (*ValidationResult, error)
	Supports(path string) bool
}

// DetectAssetType determines the asset type from file extension and content
tunc DetectAssetType(path string) AssetType {
	ext := strings.ToLower(filepath.Ext(path))
	
	switch ext {
	case ".mp4", ".avi", ".mov", ".mkv", ".webm", ".flv", ".wmv", ".m4v":
		return AssetTypeVideo
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".tiff", ".ico":
		return AssetTypeImage
	case ".json":
		return AssetTypeJSON
	case ".yaml", ".yml":
		return AssetTypeYAML
	case ".log":
		return AssetTypeText
	case ".txt":
		return AssetTypeText
	case ".md", ".markdown":
		return AssetTypeText
	case ".csv":
		return AssetTypeText
	case ".xml":
		return AssetTypeText
	case ".html", ".htm":
		return AssetTypeText
	default:
		return AssetTypeUnknown
	}
}

// DetectTextSubtype determines the text file subtype
tunc DetectTextSubtype(path string) TextSubtype {
	ext := strings.ToLower(filepath.Ext(path))
	base := strings.ToLower(filepath.Base(path))
	
	switch {
	case ext == ".log" || strings.Contains(base, ".log"):
		return TextSubtypeLog
	case ext == ".md" || ext == ".markdown":
		return TextSubtypeMarkdown
	case ext == ".csv":
		return TextSubtypeCSV
	case ext == ".xml":
		return TextSubtypeXML
	case strings.Contains(base, "report") || strings.Contains(base, "summary"):
		return TextSubtypeReport
	default:
		return TextSubtypePlain
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("[%s] %s: %s", e.Code, e.Path, e.Message)
}
