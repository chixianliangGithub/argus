package rag

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"sync"
)

// SimpleVectorDB is a local in-memory vector database implementation.
type SimpleVectorDB struct {
	mu          sync.RWMutex
	collections map[string][]VectorEntry
}

type VectorEntry struct {
	ID       string
	Vector   []float32
	Metadata map[string]interface{}
}

func NewSimpleVectorDB() *SimpleVectorDB {
	return &SimpleVectorDB{
		collections: make(map[string][]VectorEntry),
	}
}

func (db *SimpleVectorDB) Insert(ctx context.Context, collection string, vectors [][]float32, metadata []map[string]interface{}) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if _, exists := db.collections[collection]; !exists {
		db.collections[collection] = []VectorEntry{}
	}

	for i, vector := range vectors {
		entry := VectorEntry{
			ID:       fmt.Sprintf("%s-%d", collection, len(db.collections[collection])+i),
			Vector:   vector,
			Metadata: metadata[i],
		}
		// If metadata has an ID, use it
		if id, ok := metadata[i]["id"].(string); ok {
			entry.ID = id
		}
		db.collections[collection] = append(db.collections[collection], entry)
	}
	return nil
}

func (db *SimpleVectorDB) Search(ctx context.Context, collection string, queryVector []float32, topK int) ([]SearchResult, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	entries, exists := db.collections[collection]
	if !exists {
		return []SearchResult{}, nil
	}

	var results []SearchResult
	for _, entry := range entries {
		score := cosineSimilarity(queryVector, entry.Vector)
		results = append(results, SearchResult{
			ID:    entry.ID,
			Score: float32(score),
			Data:  entry.Metadata,
		})
	}

	// Sort by score descending
	// Simple bubble sort for topK since K is small usually
	for i := 0; i < len(results)-1; i++ {
		for j := 0; j < len(results)-i-1; j++ {
			if results[j].Score < results[j+1].Score {
				results[j], results[j+1] = results[j+1], results[j]
			}
		}
	}

	if len(results) > topK {
		results = results[:topK]
	}

	return results, nil
}

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// GenerateMockEmbedding creates a deterministic vector based on text content.
// This is a simple bag-of-words hash simulation for demo purposes.
// It maps words to dimensions in a fixed-size vector.
func GenerateMockEmbedding(text string, dim int) []float32 {
	vector := make([]float32, dim)
	words := strings.Fields(strings.ToLower(text))
	
	for _, word := range words {
		h := fnv.New32a()
		h.Write([]byte(word))
		idx := int(h.Sum32()) % dim
		vector[idx] += 1.0
	}
	
	// Normalize
	var norm float64
	for _, v := range vector {
		norm += float64(v * v)
	}
	if norm > 0 {
		sqrtNorm := float32(math.Sqrt(norm))
		for i := range vector {
			vector[i] /= sqrtNorm
		}
	}
	
	return vector
}
