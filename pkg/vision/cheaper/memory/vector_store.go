// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

// Package memory provides image hashing utilities for the HelixQA L1 exact
// image cache. It produces deterministic, content-addressable hashes over raw
// pixel data so that visually identical screenshots always map to the same
// cache key regardless of how the image.Image was obtained.
//
// This file adds a vector memory store backed by chromem-go for semantic
// retrieval of past vision-model interactions (few-shot examples, replay).
package memory

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	chromem "github.com/philippgille/chromem-go"

	"github.com/google/uuid"
)

const collectionName = "vision_memories"

// VisionMemory is a single recorded vision-model interaction that can be
// stored in and retrieved from the VectorMemoryStore.
type VisionMemory struct {
	// ID uniquely identifies this memory entry. A UUID is generated
	// automatically by Store if the field is empty.
	ID string

	// ImageHash is the SHA-256 hex hash of the screenshot (see ComputeImageHash).
	ImageHash string

	// Prompt is the text prompt that was sent to the vision model.
	Prompt string

	// Response is the raw text response returned by the vision model.
	Response string

	// ProviderModel identifies the provider/model string (e.g. "gemini-2.0-flash").
	ProviderModel string

	// Success indicates whether the interaction was considered successful.
	Success bool

	// Latency is the round-trip time of the vision-model call.
	Latency time.Duration

	// UIElementType is the type of UI element this interaction concerned
	// (e.g. "button", "text_field", "list_item").
	UIElementType string

	// ConfidenceScore is the model's self-reported confidence in [0, 1].
	ConfidenceScore float64

	// SimilarityScore is populated by Search / GetFewShotExamples and holds
	// the cosine similarity between the query embedding and this document's
	// embedding. It is not stored in the collection.
	SimilarityScore float64

	// Metadata holds arbitrary key-value pairs for this memory entry.
	Metadata map[string]interface{}

	// Timestamp is when the interaction was recorded.
	Timestamp time.Time

	// AccessCount is incremented each time this memory is retrieved.
	AccessCount int

	// LastAccessed is the time this memory was most recently retrieved.
	LastAccessed time.Time
}

// VectorMemoryStore persists VisionMemory entries in a chromem-go collection
// and supports semantic similarity search via embedding functions.
type VectorMemoryStore struct {
	db            *chromem.DB
	collection    *chromem.Collection
	embeddingFunc chromem.EmbeddingFunc
	mu            sync.RWMutex
	persistPath   string
}

// NewVectorMemoryStore creates a new VectorMemoryStore.
//
// persistPath — directory path for on-disk persistence. Pass an empty string
// for a purely in-memory store.
//
// embeddingFunc — the function used to embed document and query text.
// It must return normalized (unit-length) vectors.
func NewVectorMemoryStore(persistPath string, embeddingFunc chromem.EmbeddingFunc) (*VectorMemoryStore, error) {
	var (
		db  *chromem.DB
		err error
	)

	if persistPath != "" {
		db, err = chromem.NewPersistentDB(persistPath, false)
		if err != nil {
			return nil, fmt.Errorf("memory: create persistent DB at %q: %w", persistPath, err)
		}
	} else {
		db = chromem.NewDB()
	}

	col, err := db.GetOrCreateCollection(collectionName, nil, embeddingFunc)
	if err != nil {
		return nil, fmt.Errorf("memory: get or create collection %q: %w", collectionName, err)
	}

	return &VectorMemoryStore{
		db:            db,
		collection:    col,
		embeddingFunc: embeddingFunc,
		persistPath:   persistPath,
	}, nil
}

// embeddingText builds the text that will be embedded for a VisionMemory.
// Combining prompt, response, and UIElementType gives the embedding model
// enough context for meaningful semantic retrieval.
func embeddingText(mem *VisionMemory) string {
	return mem.Prompt + " " + mem.Response + " " + mem.UIElementType
}

// metadataFromMemory converts the storable fields of a VisionMemory into a
// flat map[string]string suitable for chromem-go document metadata.
func metadataFromMemory(mem *VisionMemory) map[string]string {
	m := map[string]string{
		"image_hash":       mem.ImageHash,
		"prompt":           mem.Prompt,
		"response":         mem.Response,
		"provider_model":   mem.ProviderModel,
		"success":          strconv.FormatBool(mem.Success),
		"latency_ns":       strconv.FormatInt(int64(mem.Latency), 10),
		"ui_element_type":  mem.UIElementType,
		"confidence_score": strconv.FormatFloat(mem.ConfidenceScore, 'f', -1, 64),
		"timestamp":        strconv.FormatInt(mem.Timestamp.UnixNano(), 10),
		"access_count":     strconv.Itoa(mem.AccessCount),
		"last_accessed":    strconv.FormatInt(mem.LastAccessed.UnixNano(), 10),
	}
	return m
}

// memoryFromResult reconstructs a VisionMemory from a chromem-go Result.
func memoryFromResult(r chromem.Result) *VisionMemory {
	mem := &VisionMemory{
		ID:              r.ID,
		SimilarityScore: float64(r.Similarity),
	}

	if v, ok := r.Metadata["image_hash"]; ok {
		mem.ImageHash = v
	}
	if v, ok := r.Metadata["prompt"]; ok {
		mem.Prompt = v
	}
	if v, ok := r.Metadata["response"]; ok {
		mem.Response = v
	}
	if v, ok := r.Metadata["provider_model"]; ok {
		mem.ProviderModel = v
	}
	if v, ok := r.Metadata["success"]; ok {
		mem.Success, _ = strconv.ParseBool(v)
	}
	if v, ok := r.Metadata["latency_ns"]; ok {
		ns, _ := strconv.ParseInt(v, 10, 64)
		mem.Latency = time.Duration(ns)
	}
	if v, ok := r.Metadata["ui_element_type"]; ok {
		mem.UIElementType = v
	}
	if v, ok := r.Metadata["confidence_score"]; ok {
		mem.ConfidenceScore, _ = strconv.ParseFloat(v, 64)
	}
	if v, ok := r.Metadata["timestamp"]; ok {
		ns, _ := strconv.ParseInt(v, 10, 64)
		mem.Timestamp = time.Unix(0, ns)
	}
	if v, ok := r.Metadata["access_count"]; ok {
		mem.AccessCount, _ = strconv.Atoi(v)
	}
	if v, ok := r.Metadata["last_accessed"]; ok {
		ns, _ := strconv.ParseInt(v, 10, 64)
		mem.LastAccessed = time.Unix(0, ns)
	}

	return mem
}

// Store adds a VisionMemory to the collection. If mem.ID is empty a new UUID
// is generated. The embedding is derived from the concatenation of Prompt,
// Response, and UIElementType.
func (s *VectorMemoryStore) Store(ctx context.Context, mem *VisionMemory) error {
	if mem == nil {
		return fmt.Errorf("memory: cannot store nil VisionMemory")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if mem.ID == "" {
		mem.ID = uuid.New().String()
	}

	doc := chromem.Document{
		ID:       mem.ID,
		Content:  embeddingText(mem),
		Metadata: metadataFromMemory(mem),
	}

	if err := s.collection.AddDocument(ctx, doc); err != nil {
		return fmt.Errorf("memory: store document %q: %w", mem.ID, err)
	}

	return nil
}

// Search performs a semantic similarity search against the collection and
// returns up to limit results. Returns an empty slice (no error) when the
// collection is empty or limit is zero.
func (s *VectorMemoryStore) Search(ctx context.Context, query string, limit int) ([]*VisionMemory, error) {
	if limit <= 0 {
		return []*VisionMemory{}, nil
	}

	s.mu.RLock()
	count := s.collection.Count()
	if count == 0 {
		s.mu.RUnlock()
		return []*VisionMemory{}, nil
	}
	if limit > count {
		limit = count
	}
	results, err := s.collection.Query(ctx, query, limit, nil, nil)
	s.mu.RUnlock()

	if err != nil {
		return nil, fmt.Errorf("memory: search query %q: %w", query, err)
	}

	memories := make([]*VisionMemory, 0, len(results))
	for _, r := range results {
		memories = append(memories, memoryFromResult(r))
	}

	return memories, nil
}

// GetFewShotExamples returns up to count successful VisionMemory entries that
// are semantically similar to query. Only memories with Success == true are
// considered.
func (s *VectorMemoryStore) GetFewShotExamples(ctx context.Context, query string, count int) ([]*VisionMemory, error) {
	if count <= 0 {
		return []*VisionMemory{}, nil
	}

	s.mu.RLock()
	total := s.collection.Count()
	if total == 0 {
		s.mu.RUnlock()
		return []*VisionMemory{}, nil
	}
	limit := count
	if limit > total {
		limit = total
	}
	where := map[string]string{"success": "true"}
	results, err := s.collection.Query(ctx, query, limit, where, nil)
	s.mu.RUnlock()

	if err != nil {
		return nil, fmt.Errorf("memory: few-shot query %q: %w", query, err)
	}

	memories := make([]*VisionMemory, 0, len(results))
	for _, r := range results {
		memories = append(memories, memoryFromResult(r))
	}

	return memories, nil
}

// Size returns the number of documents currently held in the collection.
func (s *VectorMemoryStore) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.collection.Count()
}

// Clear deletes the existing collection and recreates it so the store can be
// reused after clearing.
func (s *VectorMemoryStore) Clear(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Delete the old collection; chromem.DB.DeleteCollection handles persistence.
	if err := s.db.DeleteCollection(collectionName); err != nil {
		return fmt.Errorf("memory: delete collection: %w", err)
	}

	// Recreate with the same embedding function that was supplied at construction.
	col, err := s.db.GetOrCreateCollection(collectionName, nil, s.embeddingFunc)
	if err != nil {
		return fmt.Errorf("memory: recreate collection: %w", err)
	}

	s.collection = col
	return nil
}

// Close persists the DB if a persistPath was provided, then releases resources.
// For in-memory stores this is a no-op.
func (s *VectorMemoryStore) Close() error {
	if s.persistPath == "" {
		return nil
	}

	// chromem-go's persistent DB writes on every AddDocument; Export provides a
	// single-file snapshot as an additional durability guarantee.
	snapshotPath := s.persistPath + "/vision_memories.gob"
	if err := s.db.ExportToFile(snapshotPath, false, ""); err != nil {
		return fmt.Errorf("memory: export snapshot to %q: %w", snapshotPath, err)
	}

	return nil
}
