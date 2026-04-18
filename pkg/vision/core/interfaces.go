// Package core provides foundational interfaces for the HelixQA vision system.
//
// This package defines the contracts that all vision components must implement,
// enabling pluggable OCR engines, element detectors, and layout analyzers.
//
//go:build vision
// +build vision

package core

import (
	"context"
	"image"
	"time"
)

// Frame represents a processed video/screenshot frame.
type Frame struct {
	// Data contains the raw image data (BGR format for OpenCV)
	Data []byte

	// Bounds defines the image dimensions
	Bounds image.Rectangle

	// Timestamp when the frame was captured
	Timestamp time.Time

	// Source identifies where this frame came from
	Source FrameSource

	// Metadata contains additional frame information
	Metadata FrameMetadata
}

// FrameSource identifies the origin of a frame.
type FrameSource int

const (
	SourceUnknown FrameSource = iota
	SourceScreenshot
	SourceVideoStream
	SourceScrcpy
	SourceDesktopCapture
	SourceCamera
)

// FrameMetadata contains additional information about a frame.
type FrameMetadata struct {
	// Platform this frame was captured from
	Platform string

	// DeviceID identifies the specific device
	DeviceID string

	// Orientation of the device when captured
	Orientation Orientation

	// ScaleFactor for high-DPI displays
	ScaleFactor float64

	// Custom fields for extensibility
	Extra map[string]interface{}
}

// Orientation represents device screen orientation.
type Orientation int

const (
	OrientationUnknown Orientation = iota
	OrientationPortrait
	OrientationLandscape
	OrientationPortraitUpsideDown
	OrientationLandscapeLeft
)

// Point represents a 2D coordinate.
type Point struct {
	X, Y int
}

// Rectangle represents a bounding box with confidence.
type Rectangle struct {
	image.Rectangle
	Confidence float64
}

// Center returns the center point of the rectangle.
func (r Rectangle) Center() Point {
	return Point{
		X: r.Min.X + r.Dx()/2,
		Y: r.Min.Y + r.Dy()/2,
	}
}

// UIElement represents a detected UI element.
type UIElement struct {
	// ID uniquely identifies this element
	ID string

	// Type of UI element (button, text, icon, etc.)
	Type ElementType

	// Bounds is the bounding box
	Bounds Rectangle

	// Text content if applicable
	Text string

	// Description of the element's appearance/function
	Description string

	// Confidence score for this detection
	Confidence float64

	// Source indicates which detector found this element
	Source string

	// Attributes contains additional element properties
	Attributes map[string]string
}

// ElementType categorizes UI elements.
type ElementType string

const (
	ElementUnknown    ElementType = "unknown"
	ElementButton     ElementType = "button"
	ElementText       ElementType = "text"
	ElementTextField  ElementType = "textfield"
	ElementIcon       ElementType = "icon"
	ElementImage      ElementType = "image"
	ElementCheckbox   ElementType = "checkbox"
	ElementRadio      ElementType = "radio"
	ElementDropdown   ElementType = "dropdown"
	ElementList       ElementType = "list"
	ElementListItem   ElementType = "listitem"
	ElementNavigation ElementType = "navigation"
	ElementTab        ElementType = "tab"
	ElementDialog     ElementType = "dialog"
	ElementToast      ElementType = "toast"
	ElementLoading    ElementType = "loading"
	ElementVideo      ElementType = "video"
)

// TextRegion represents a region containing text.
type TextRegion struct {
	// Bounds of the text region
	Bounds Rectangle

	// Text content
	Text string

	// Confidence of OCR
	Confidence float64

	// Language detected
	Language string

	// IsVertical indicates vertical text (e.g., Chinese/Japanese)
	IsVertical bool
}

// UILayout represents the structural analysis of a UI.
type UILayout struct {
	// Timestamp of analysis
	Timestamp time.Time

	// ScreenBounds defines the full screen dimensions
	ScreenBounds image.Rectangle

	// NavigationBar region if detected
	NavigationBar *UIRegion

	// ContentArea is the main content region
	ContentArea *UIRegion

	// InputFields detected
	InputFields []UIRegion

	// Buttons detected
	Buttons []UIRegion

	// TextBlocks detected
	TextBlocks []TextBlock

	// Elements contains all detected UI elements
	Elements []UIElement

	// Hierarchy represents the tree structure
	Hierarchy *LayoutTree
}

// UIRegion represents a region of the UI with a specific purpose.
type UIRegion struct {
	// Bounds of the region
	Bounds Rectangle

	// Type of region
	Type RegionType

	// Confidence of detection
	Confidence float64
}

// RegionType categorizes UI regions.
type RegionType string

const (
	RegionUnknown    RegionType = "unknown"
	RegionNavigation RegionType = "navigation"
	RegionContent    RegionType = "content"
	RegionSidebar    RegionType = "sidebar"
	RegionHeader     RegionType = "header"
	RegionFooter     RegionType = "footer"
	RegionModal      RegionType = "modal"
	RegionToast      RegionType = "toast"
	RegionKeyboard   RegionType = "keyboard"
)

// TextBlock represents a block of text with layout information.
type TextBlock struct {
	TextRegion

	// BlockType categorizes the text (heading, paragraph, caption)
	BlockType TextBlockType

	// Level for headings (1-6)
	Level int

	// ReadingOrder position in document flow
	ReadingOrder int
}

// TextBlockType categorizes text blocks.
type TextBlockType string

const (
	TextBlockUnknown   TextBlockType = "unknown"
	TextBlockHeading   TextBlockType = "heading"
	TextBlockParagraph TextBlockType = "paragraph"
	TextBlockCaption   TextBlockType = "caption"
	TextBlockLabel     TextBlockType = "label"
	TextBlockButton    TextBlockType = "button"
	TextBlockLink      TextBlockType = "link"
	TextBlockCode      TextBlockType = "code"
	TextBlockQuote     TextBlockType = "quote"
)

// LayoutTree represents hierarchical UI structure.
type LayoutTree struct {
	// Root node
	Root *LayoutNode

	// Flattened list of all nodes
	AllNodes []*LayoutNode
}

// LayoutNode represents a node in the layout tree.
type LayoutNode struct {
	// ID unique identifier
	ID string

	// Element type
	Type ElementType

	// Bounds
	Bounds Rectangle

	// Text content
	Text string

	// Parent node (nil for root)
	Parent *LayoutNode

	// Children nodes
	Children []*LayoutNode

	// Depth in tree
	Depth int

	// Attributes
	Attributes map[string]string
}

// ElementDetector interface for detecting UI elements.
type ElementDetector interface {
	// Detect finds all UI elements in a frame
	Detect(ctx context.Context, frame *Frame) ([]UIElement, error)

	// DetectType finds elements of a specific type
	DetectType(ctx context.Context, frame *Frame, elemType ElementType) ([]UIElement, error)

	// FindByTemplate locates an element matching a template image
	FindByTemplate(ctx context.Context, frame *Frame, template []byte, confidence float64) (*UIElement, error)

	// FindByText locates elements containing specific text
	FindByText(ctx context.Context, frame *Frame, text string) ([]UIElement, error)
}

// TextExtractor interface for extracting text from frames.
type TextExtractor interface {
	// Extract extracts all text from a frame
	Extract(ctx context.Context, frame *Frame) ([]TextRegion, error)

	// ExtractRegion extracts text from a specific region
	ExtractRegion(ctx context.Context, frame *Frame, region image.Rectangle) (*TextRegion, error)

	// DetectLanguage identifies the language of text in the frame
	DetectLanguage(ctx context.Context, frame *Frame) (string, error)
}

// LayoutAnalyzer interface for analyzing UI structure.
type LayoutAnalyzer interface {
	// Analyze performs complete layout analysis
	Analyze(ctx context.Context, frame *Frame) (*UILayout, error)

	// DetectRegions identifies distinct UI regions
	DetectRegions(ctx context.Context, frame *Frame) ([]UIRegion, error)

	// BuildHierarchy constructs the layout tree
	BuildHierarchy(ctx context.Context, elements []UIElement) (*LayoutTree, error)
}

// VisualNavigator interface for CV-based navigation.
type VisualNavigator interface {
	// FindAndClick finds an element and clicks it
	FindAndClick(ctx context.Context, description string) error

	// FindAndInput finds a text field and enters text
	FindAndInput(ctx context.Context, fieldDescription string, text string) error

	// NavigateTo navigates to a specific screen/page
	NavigateTo(ctx context.Context, destination string) error

	// WaitForElement waits for an element to appear
	WaitForElement(ctx context.Context, description string, timeout time.Duration) (*UIElement, error)

	// VerifyState checks if the UI is in an expected state
	VerifyState(ctx context.Context, expected string) (bool, error)
}

// RegressionComparator interface for visual regression testing.
type RegressionComparator interface {
	// Compare compares a current frame against a baseline
	Compare(ctx context.Context, baseline, current *Frame) (*DiffReport, error)

	// UpdateBaseline sets a new baseline
	UpdateBaseline(ctx context.Context, name string, frame *Frame) error

	// LoadBaseline retrieves a stored baseline
	LoadBaseline(ctx context.Context, name string) (*Frame, error)
}

// DiffReport contains visual regression comparison results.
type DiffReport struct {
	// Similarity score (0-1, where 1 is identical)
	Similarity float64

	// DiffPixels count of different pixels
	DiffPixels int

	// DiffPercentage percentage of changed pixels
	DiffPercentage float64

	// DiffImage visualization of differences
	DiffImage []byte

	// ChangedRegions list of regions that changed
	ChangedRegions []Rectangle

	// Passed whether the comparison passed threshold
	Passed bool

	// Threshold used for comparison
	Threshold float64
}

// FrameProcessor processes frames through the vision pipeline.
type FrameProcessor interface {
	// Process runs the complete vision pipeline on a frame
	Process(ctx context.Context, frame *Frame) (*VisionResult, error)

	// ProcessWithOptions runs pipeline with specific options
	ProcessWithOptions(ctx context.Context, frame *Frame, opts ProcessOptions) (*VisionResult, error)
}

// VisionResult contains complete vision analysis results.
type VisionResult struct {
	// Timestamp of analysis
	Timestamp time.Time

	// Duration of processing
	Duration time.Duration

	// Elements detected
	Elements []UIElement

	// Text regions found
	TextRegions []TextRegion

	// Layout analysis
	Layout *UILayout

	// Raw results from individual components
	RawResults map[string]interface{}
}

// ProcessOptions configures the vision pipeline.
type ProcessOptions struct {
	// DetectElements enables element detection
	DetectElements bool

	// ExtractText enables text extraction
	ExtractText bool

	// AnalyzeLayout enables layout analysis
	AnalyzeLayout bool

	// ElementTypes to detect (empty = all)
	ElementTypes []ElementType

	// ROIs limits processing to specific regions
	ROIs []image.Rectangle

	// Timeout for processing
	Timeout time.Duration
}

// Cache provides caching for vision operations.
type Cache interface {
	// Get retrieves cached data
	Get(key string) (interface{}, bool)

	// Set stores data in cache
	Set(key string, value interface{}, ttl time.Duration)

	// Invalidate removes cached data
	Invalidate(key string)

	// Clear removes all cached data
	Clear()
}

// GPUAccelerator provides GPU-accelerated operations.
type GPUAccelerator interface {
	// IsAvailable returns true if GPU acceleration is available
	IsAvailable() bool

	// Upload uploads data to GPU
	Upload(data []byte) (GPUMemory, error)

	// Download downloads data from GPU
	Download(mem GPUMemory) ([]byte, error)

	// Process runs a GPU-accelerated operation
	Process(op GPUOperation, inputs []GPUMemory) (GPUMemory, error)
}

// GPUMemory represents GPU memory allocation.
type GPUMemory interface {
	// ID returns the memory identifier
	ID() string

	// Size returns the allocated size
	Size() int

	// Free releases the GPU memory
	Free() error
}

// GPUOperation represents a GPU-accelerated operation.
type GPUOperation interface {
	// Name identifies the operation
	Name() string

	// Run executes the operation on GPU
	Run(ctx context.Context, inputs []GPUMemory) (GPUMemory, error)
}

// MetricsCollector collects performance metrics.
type MetricsCollector interface {
	// RecordTiming records operation duration
	RecordTiming(operation string, duration time.Duration)

	// RecordCount records an operation count
	RecordCount(operation string, count int)

	// RecordError records an error occurrence
	RecordError(operation string, err error)

	// GetMetrics returns collected metrics
	GetMetrics() Metrics
}

// Metrics contains collected performance data.
type Metrics struct {
	Timings map[string][]time.Duration
	Counts  map[string]int
	Errors  map[string]int
}
