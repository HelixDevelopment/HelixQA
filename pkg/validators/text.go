// Package validators provides asset validation for QA session results
package validators

import (
	"bufio"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

// TextValidator validates text-based files
type TextValidator struct{}

// NewTextValidator creates a new text validator
func NewTextValidator() *TextValidator {
	return &TextValidator{}
}

// Validate validates a text file
func (v *TextValidator) Validate(path string) (*ValidationResult, error) {
	result := &ValidationResult{
		AssetPath: path,
		AssetType: AssetTypeText,
		IsValid:   true,
		Metadata:  make(map[string]interface{}),
	}

	// Detect subtype
	subtype := DetectTextSubtype(path)
	result.TextSubtype = subtype

	// Open file
	file, err := os.Open(path)
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to open file: %v", err))
		return result, nil
	}
	defer file.Close()

	// Get file info
	stat, err := file.Stat()
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to stat file: %v", err))
		return result, nil
	}

	// Empty file check
	if stat.Size() == 0 {
		result.Warnings = append(result.Warnings, "file is empty (0 bytes)")
	}

	result.Metadata["size_bytes"] = stat.Size()
	result.Metadata["size_human"] = formatBytes(stat.Size())

	// Validate based on subtype
	switch subtype {
	case TextSubtypeLog:
		v.validateLog(file, result)
	case TextSubtypeCSV:
		v.validateCSV(file, result)
	case TextSubtypeXML:
		v.validateXML(file, result)
	case TextSubtypeMarkdown, TextSubtypeReport:
		v.validateMarkdown(file, result)
	default:
		v.validatePlainText(file, result)
	}

	return result, nil
}

// Supports checks if this validator supports the given file
func (v *TextValidator) Supports(path string) bool {
	type_ := DetectAssetType(path)
	return type_ == AssetTypeText
}

func (v *TextValidator) validateLog(file *os.File, result *ValidationResult) {
	scanner := bufio.NewScanner(file)
	lineCount := 0
	errorCount := 0
	warningCount := 0

	for scanner.Scan() {
		lineCount++
		line := scanner.Text()
		
		// Check for error/warning patterns
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "fatal") || strings.Contains(lower, "panic") {
			errorCount++
		}
		if strings.Contains(lower, "warning") || strings.Contains(lower, "warn") {
			warningCount++
		}
		
		// Limit scan for large files
		if lineCount >= 10000 {
			result.Warnings = append(result.Warnings, "log file truncated (only checked first 10000 lines)")
			break
		}
	}

	result.Metadata["line_count"] = lineCount
	result.Metadata["error_count"] = errorCount
	result.Metadata["warning_count"] = warningCount

	if errorCount > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("found %d error/panic entries", errorCount))
	}
}

func (v *TextValidator) validateCSV(file *os.File, result *ValidationResult) {
	reader := csv.NewReader(file)
	
	// Read header
	header, err := reader.Read()
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to read CSV header: %v", err))
		return
	}

	result.Metadata["column_count"] = len(header)
	result.Metadata["columns"] = header

	// Count rows
	rowCount := 0
	for {
		_, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("CSV parse error at row %d: %v", rowCount+2, err))
		}
		rowCount++
	}

	result.Metadata["row_count"] = rowCount
}

func (v *TextValidator) validateXML(file *os.File, result *ValidationResult) {
	data, err := io.ReadAll(file)
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to read XML: %v", err))
		return
	}

	// Check basic XML structure
	var dummy interface{}
	if err := xml.Unmarshal(data, &dummy); err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("XML parse error: %v", err))
		return
	}

	result.Metadata["valid_xml"] = true
}

func (v *TextValidator) validateMarkdown(file *os.File, result *ValidationResult) {
	data, err := io.ReadAll(file)
	if err != nil {
		result.IsValid = false
		result.Errors = append(result.Errors, fmt.Sprintf("failed to read file: %v", err))
		return
	}

	content := string(data)
	
	// Count markdown elements
	headingCount := strings.Count(content, "# ")
	linkCount := strings.Count(content, "](")
	codeBlockCount := strings.Count(content, "```")

	result.Metadata["heading_count"] = headingCount
	result.Metadata["link_count"] = linkCount
	result.Metadata["code_block_count"] = codeBlockCount/2 // Divide by 2 for opening/closing
	result.Metadata["char_count"] = len(content)
	result.Metadata["word_count"] = len(strings.Fields(content))
}

func (v *TextValidator) validatePlainText(file *os.File, result *ValidationResult) {
	// Check UTF-8 validity
	buf := make([]byte, 4096)
	totalBytes := 0
	invalidUTF8 := 0

	for {
		n, err := file.Read(buf)
		if n > 0 {
			totalBytes += n
			if !utf8.Valid(buf[:n]) {
				invalidUTF8++
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("read error: %v", err))
			break
		}
	}

	result.Metadata["valid_utf8"] = invalidUTF8 == 0
	if invalidUTF8 > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("found %d invalid UTF-8 sequences", invalidUTF8))
	}
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
