package core

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"rag-go-app/models"
	"strconv"
	"strings"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

type VectorDB struct {
	conn *sql.DB
}

func NewVectorDB(dbPath string) (*VectorDB, error) {
	// Load the sqlite-vec extension
	sqlite_vec.Auto()

	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	db := &VectorDB{conn: conn}

	// Verify sqlite-vec is loaded
	var version string
	err = conn.QueryRow("SELECT vec_version()").Scan(&version)
	if err != nil {
		return nil, fmt.Errorf("sqlite-vec not available: %w", err)
	}
	log.Printf("Using sqlite-vec version: %s", version)

	if err := db.createTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}

	return db, nil
}

func (db *VectorDB) createTables() error {
	// Enhanced collections table with metadata support
	collectionsSQL := `
	CREATE TABLE IF NOT EXISTS collections (
		name TEXT PRIMARY KEY,
		description TEXT,
		embedding_model TEXT DEFAULT 'nomic-embed-text-v1.5',
		embedding_dimension INTEGER DEFAULT 1024,
		metadata TEXT, -- JSON metadata
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	// Enhanced documents table with document-level metadata
	documentsSQL := `
	CREATE TABLE IF NOT EXISTS documents (
		id TEXT PRIMARY KEY,
		collection_name TEXT NOT NULL,
		content TEXT NOT NULL,
		source TEXT,
		doc_type TEXT,
		metadata TEXT, -- JSON metadata
		chunk_count INTEGER DEFAULT 0,
		chunking_strategy TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (collection_name) REFERENCES collections(name) ON DELETE CASCADE
	);`

	// Enhanced chunks table with hierarchical and metadata support
	chunksSQL := `
	CREATE TABLE IF NOT EXISTS enhanced_chunks (
		id TEXT PRIMARY KEY,
		document_id TEXT NOT NULL,
		collection_name TEXT NOT NULL,
		text TEXT NOT NULL,
		
		-- Hierarchical information
		parent_chunk_id TEXT,
		child_chunk_ids TEXT, -- JSON array of child IDs
		
		-- Structural metadata
		section TEXT,
		subsection TEXT,
		chunk_type TEXT NOT NULL,
		
		-- Position information
		start_pos INTEGER,
		end_pos INTEGER,
		chunk_index INTEGER,
		
		-- Semantic metadata
		keywords TEXT, -- JSON array of keywords
		metadata TEXT, -- JSON general metadata
		confidence REAL DEFAULT 0.0,
		
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		
		FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE,
		FOREIGN KEY (collection_name) REFERENCES collections(name) ON DELETE CASCADE,
		FOREIGN KEY (parent_chunk_id) REFERENCES enhanced_chunks(id) ON DELETE SET NULL
	);`

	// NOTE: We'll create the embeddings table dynamically when we know the actual dimension
	// This is more flexible than hardcoding 768 or 1024

	// Indexes for better performance
	indexesSQL := []string{
		`CREATE INDEX IF NOT EXISTS idx_chunks_document ON enhanced_chunks(document_id);`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_collection ON enhanced_chunks(collection_name);`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_type ON enhanced_chunks(chunk_type);`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_section ON enhanced_chunks(section);`,
		`CREATE INDEX IF NOT EXISTS idx_chunks_parent ON enhanced_chunks(parent_chunk_id);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_collection ON documents(collection_name);`,
		`CREATE INDEX IF NOT EXISTS idx_documents_type ON documents(doc_type);`,
	}

	// Execute table creation (excluding embeddings table for now)
	for _, sql := range []string{collectionsSQL, documentsSQL, chunksSQL} {
		if _, err := db.conn.Exec(sql); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Execute index creation
	for _, sql := range indexesSQL {
		if _, err := db.conn.Exec(sql); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// ensureEmbeddingTableExists creates or recreates the embedding table with the correct dimension
func (db *VectorDB) ensureEmbeddingTableExists(dimension int) error {
	// Check if the table exists and has the right dimension
	var existingDim int
	var tableExists bool

	// Try to get the current dimension from an existing table
	err := db.conn.QueryRow(`
		SELECT 1 FROM sqlite_master 
		WHERE type='table' AND name='chunk_embeddings'
	`).Scan(&existingDim)

	if err == nil {
		tableExists = true
		// Try to determine the current dimension by checking the schema
		// This is a bit tricky with sqlite-vec, so we'll use a different approach

		// Test with a dummy embedding to see if it works
		testEmbedding := make([]string, dimension)
		for i := range testEmbedding {
			testEmbedding[i] = "0.0"
		}
		testEmbeddingStr := "[" + strings.Join(testEmbedding, ",") + "]"

		// Try to insert a test embedding
		_, testErr := db.conn.Exec(`INSERT OR REPLACE INTO chunk_embeddings (chunk_id, embedding) VALUES (?, ?)`,
			"test_dimension_check", testEmbeddingStr)

		if testErr != nil && strings.Contains(testErr.Error(), "Dimension mismatch") {
			log.Printf("Detected dimension mismatch, recreating embedding table for %d dimensions", dimension)
			// Drop the existing table
			if _, err := db.conn.Exec(`DROP TABLE IF EXISTS chunk_embeddings`); err != nil {
				return fmt.Errorf("failed to drop existing embedding table: %w", err)
			}
			tableExists = false
		} else if testErr == nil {
			// Clean up test record
			db.conn.Exec(`DELETE FROM chunk_embeddings WHERE chunk_id = 'test_dimension_check'`)
			log.Printf("Embedding table already exists with correct dimension (%d)", dimension)
			return nil
		}
	}

	if !tableExists {
		// Create the embedding table with the correct dimension
		embeddingsSQL := fmt.Sprintf(`
		CREATE VIRTUAL TABLE IF NOT EXISTS chunk_embeddings USING vec0(
			chunk_id TEXT PRIMARY KEY,
			embedding FLOAT[%d]
		)`, dimension)

		if _, err := db.conn.Exec(embeddingsSQL); err != nil {
			return fmt.Errorf("failed to create embedding table with dimension %d: %w", dimension, err)
		}

		log.Printf("Created embedding table with %d dimensions", dimension)
	}

	return nil
}

func (db *VectorDB) CreateCollection(name, description string) error {
	sql := `INSERT OR IGNORE INTO collections (name, description) VALUES (?, ?)`
	_, err := db.conn.Exec(sql, name, description)
	if err != nil {
		return fmt.Errorf("failed to create collection: %w", err)
	}
	return nil
}

func (db *VectorDB) AddDocument(collectionName string, doc *models.Document) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Serialize document metadata
	metadataJSON := "{}"
	if doc.Metadata != nil {
		if metadataBytes, err := json.Marshal(doc.Metadata); err == nil {
			metadataJSON = string(metadataBytes)
		}
	}

	// Insert document
	docSQL := `INSERT OR REPLACE INTO documents 
		(id, collection_name, content, source, doc_type, metadata, chunk_count, chunking_strategy) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	chunkCount := len(doc.Chunks)
	chunkingStrategy := ""
	if chunkCount > 0 && doc.Metadata != nil {
		if strategy, ok := doc.Metadata["chunking_strategy"].(string); ok {
			chunkingStrategy = strategy
		}
	}

	_, err = tx.Exec(docSQL, doc.ID, collectionName, doc.Content, doc.Source,
		doc.DocType, metadataJSON, chunkCount, chunkingStrategy)
	if err != nil {
		return fmt.Errorf("failed to insert document: %w", err)
	}

	// Insert enhanced chunks
	for _, chunk := range doc.Chunks {
		if err := db.insertEnhancedChunk(tx, collectionName, chunk); err != nil {
			return fmt.Errorf("failed to insert chunk: %w", err)
		}
	}

	return tx.Commit()
}

func (db *VectorDB) insertEnhancedChunk(tx *sql.Tx, collectionName string, chunk *models.EnhancedChunk) error {
	// Serialize arrays and metadata
	childIDsJSON := "[]"
	if len(chunk.ChildChunkIDs) > 0 {
		if childBytes, err := json.Marshal(chunk.ChildChunkIDs); err == nil {
			childIDsJSON = string(childBytes)
		}
	}

	keywordsJSON := "[]"
	if len(chunk.Keywords) > 0 {
		if keywordBytes, err := json.Marshal(chunk.Keywords); err == nil {
			keywordsJSON = string(keywordBytes)
		}
	}

	metadataJSON := "{}"
	if chunk.Metadata != nil {
		if metadataBytes, err := json.Marshal(chunk.Metadata); err == nil {
			metadataJSON = string(metadataBytes)
		}
	}

	// Insert chunk
	chunkSQL := `INSERT OR REPLACE INTO enhanced_chunks 
		(id, document_id, collection_name, text, parent_chunk_id, child_chunk_ids,
		 section, subsection, chunk_type, start_pos, end_pos, chunk_index,
		 keywords, metadata, confidence) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := tx.Exec(chunkSQL,
		chunk.ID, chunk.DocumentID, collectionName, chunk.Text,
		chunk.ParentChunkID, childIDsJSON,
		chunk.Section, chunk.Subsection, chunk.ChunkType,
		chunk.StartPos, chunk.EndPos, chunk.ChunkIndex,
		keywordsJSON, metadataJSON, chunk.Confidence)

	return err
}

func (db *VectorDB) AddEmbeddings(chunks []*models.EnhancedChunk) error {
	if len(chunks) == 0 {
		return nil
	}

	// Determine embedding dimension from first chunk
	var embeddingDim int
	for _, chunk := range chunks {
		if len(chunk.Embedding) > 0 {
			embeddingDim = len(chunk.Embedding)
			break
		}
	}

	if embeddingDim == 0 {
		return fmt.Errorf("no valid embeddings found in chunks")
	}

	// Ensure the embedding table exists with the correct dimension
	if err := db.ensureEmbeddingTableExists(embeddingDim); err != nil {
		return err
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	for _, chunk := range chunks {
		if len(chunk.Embedding) == 0 {
			continue
		}

		if len(chunk.Embedding) != embeddingDim {
			return fmt.Errorf("chunk %s has embedding dimension %d, expected %d",
				chunk.ID, len(chunk.Embedding), embeddingDim)
		}

		// Convert embedding to string format for sqlite-vec
		embeddingStr := "[" + strings.Join(float32SliceToStringSlice(chunk.Embedding), ",") + "]"

		sql := `INSERT OR REPLACE INTO chunk_embeddings (chunk_id, embedding) VALUES (?, ?)`
		_, err := tx.Exec(sql, chunk.ID, embeddingStr)
		if err != nil {
			return fmt.Errorf("failed to insert embedding for chunk %s: %w", chunk.ID, err)
		}
	}

	return tx.Commit()
}

func (db *VectorDB) QuerySimilarChunks(collectionName string, queryEmbedding []float32, topK int, filters map[string]interface{}) ([]*models.EnhancedChunk, []float64, error) {
	// Build the query with optional filters
	baseQuery := `
		SELECT c.id, c.document_id, c.text, c.parent_chunk_id, c.child_chunk_ids,
		       c.section, c.subsection, c.chunk_type, c.start_pos, c.end_pos, 
		       c.chunk_index, c.keywords, c.metadata, c.confidence,
		       vt.distance
		FROM enhanced_chunks c
		JOIN chunk_embeddings vt ON c.id = vt.chunk_id
		WHERE c.collection_name = ? AND vt.embedding MATCH ? AND k = ?`

	// Add metadata filters
	var args []interface{}
	args = append(args, collectionName)

	// Convert query embedding to string
	queryEmbeddingStr := "[" + strings.Join(float32SliceToStringSlice(queryEmbedding), ",") + "]"
	args = append(args, queryEmbeddingStr)
	args = append(args, topK)

	// Apply metadata filters
	whereConditions := []string{}
	for key, value := range filters {
		switch key {
		case "chunk_type":
			whereConditions = append(whereConditions, "c.chunk_type = ?")
			args = append(args, value)
		case "section":
			whereConditions = append(whereConditions, "c.section = ?")
			args = append(args, value)
		case "doc_type":
			whereConditions = append(whereConditions, "c.document_id IN (SELECT id FROM documents WHERE doc_type = ?)")
			args = append(args, value)
		}
	}

	if len(whereConditions) > 0 {
		baseQuery += " AND " + strings.Join(whereConditions, " AND ")
	}

	baseQuery += " ORDER BY vt.distance"

	rows, err := db.conn.Query(baseQuery, args...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query similar chunks: %w", err)
	}
	defer rows.Close()

	var chunks []*models.EnhancedChunk
	var scores []float64

	for rows.Next() {
		chunk := &models.EnhancedChunk{}
		var childIDsJSON, keywordsJSON, metadataJSON string
		var distance float64

		err := rows.Scan(
			&chunk.ID, &chunk.DocumentID, &chunk.Text, &chunk.ParentChunkID, &childIDsJSON,
			&chunk.Section, &chunk.Subsection, &chunk.ChunkType,
			&chunk.StartPos, &chunk.EndPos, &chunk.ChunkIndex,
			&keywordsJSON, &metadataJSON, &chunk.Confidence, &distance)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to scan chunk: %w", err)
		}

		// Deserialize JSON fields
		if childIDsJSON != "[]" {
			json.Unmarshal([]byte(childIDsJSON), &chunk.ChildChunkIDs)
		}
		if keywordsJSON != "[]" {
			json.Unmarshal([]byte(keywordsJSON), &chunk.Keywords)
		}
		if metadataJSON != "{}" {
			json.Unmarshal([]byte(metadataJSON), &chunk.Metadata)
		}

		chunks = append(chunks, chunk)
		// Convert distance to similarity score (1 - distance for cosine similarity)
		similarity := 1.0 - distance
		scores = append(scores, similarity)
	}

	return chunks, scores, nil
}

func (db *VectorDB) GetChunkWithParents(chunkID string) ([]*models.EnhancedChunk, error) {
	// Get the chunk and its parent hierarchy
	query := `
		WITH RECURSIVE chunk_hierarchy AS (
			-- Base case: get the requested chunk
			SELECT id, document_id, text, parent_chunk_id, child_chunk_ids,
			       section, subsection, chunk_type, start_pos, end_pos,
			       chunk_index, keywords, metadata, confidence, 0 as level
			FROM enhanced_chunks 
			WHERE id = ?
			
			UNION ALL
			
			-- Recursive case: get parent chunks
			SELECT c.id, c.document_id, c.text, c.parent_chunk_id, c.child_chunk_ids,
			       c.section, c.subsection, c.chunk_type, c.start_pos, c.end_pos,
			       c.chunk_index, c.keywords, c.metadata, c.confidence, ch.level + 1
			FROM enhanced_chunks c
			JOIN chunk_hierarchy ch ON c.id = ch.parent_chunk_id
		)
		SELECT * FROM chunk_hierarchy ORDER BY level DESC`

	rows, err := db.conn.Query(query, chunkID)
	if err != nil {
		return nil, fmt.Errorf("failed to query chunk hierarchy: %w", err)
	}
	defer rows.Close()

	var chunks []*models.EnhancedChunk
	for rows.Next() {
		chunk := &models.EnhancedChunk{}
		var childIDsJSON, keywordsJSON, metadataJSON string
		var level int

		err := rows.Scan(
			&chunk.ID, &chunk.DocumentID, &chunk.Text, &chunk.ParentChunkID, &childIDsJSON,
			&chunk.Section, &chunk.Subsection, &chunk.ChunkType,
			&chunk.StartPos, &chunk.EndPos, &chunk.ChunkIndex,
			&keywordsJSON, &metadataJSON, &chunk.Confidence, &level)
		if err != nil {
			return nil, fmt.Errorf("failed to scan chunk: %w", err)
		}

		// Deserialize JSON fields
		if childIDsJSON != "[]" {
			json.Unmarshal([]byte(childIDsJSON), &chunk.ChildChunkIDs)
		}
		if keywordsJSON != "[]" {
			json.Unmarshal([]byte(keywordsJSON), &chunk.Keywords)
		}
		if metadataJSON != "{}" {
			json.Unmarshal([]byte(metadataJSON), &chunk.Metadata)
		}

		chunks = append(chunks, chunk)
	}

	return chunks, nil
}

// Legacy support for backwards compatibility
func (db *VectorDB) AddChunk(collectionName string, chunk *models.DocumentChunk) error {
	// Convert legacy chunk to enhanced chunk
	enhancedChunk := &models.EnhancedChunk{
		ID:         chunk.ID,
		DocumentID: chunk.DocumentID,
		Text:       chunk.Text,
		Embedding:  chunk.Embedding,
		ChunkType:  "legacy",
	}

	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := db.insertEnhancedChunk(tx, collectionName, enhancedChunk); err != nil {
		return err
	}

	return tx.Commit()
}

// Legacy support for simple queries
func (db *VectorDB) QuerySimilar(collectionName string, queryEmbedding []float32, topK int) ([]*models.DocumentChunk, error) {
	enhancedChunks, _, err := db.QuerySimilarChunks(collectionName, queryEmbedding, topK, nil)
	if err != nil {
		return nil, err
	}

	// Convert back to legacy format
	var legacyChunks []*models.DocumentChunk
	for _, chunk := range enhancedChunks {
		legacyChunk := &models.DocumentChunk{
			ID:         chunk.ID,
			DocumentID: chunk.DocumentID,
			Text:       chunk.Text,
			Embedding:  chunk.Embedding,
		}
		legacyChunks = append(legacyChunks, legacyChunk)
	}

	return legacyChunks, nil
}

func (db *VectorDB) Close() error {
	return db.conn.Close()
}

// Collection management methods
func (db *VectorDB) ListCollections() ([]map[string]interface{}, error) {
	sql := `SELECT name, description, created_at FROM collections ORDER BY created_at DESC`
	rows, err := db.conn.Query(sql)
	if err != nil {
		return nil, fmt.Errorf("failed to list collections: %w", err)
	}
	defer rows.Close()

	var collections []map[string]interface{}
	for rows.Next() {
		var name, description, createdAt string
		err := rows.Scan(&name, &description, &createdAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan collection: %w", err)
		}

		// Count documents in collection
		countSQL := `SELECT COUNT(DISTINCT document_id) FROM enhanced_chunks WHERE collection_name = ?`
		var docCount int
		err = db.conn.QueryRow(countSQL, name).Scan(&docCount)
		if err != nil {
			docCount = 0 // Continue if count fails
		}

		// Count total chunks
		chunkCountSQL := `SELECT COUNT(*) FROM enhanced_chunks WHERE collection_name = ?`
		var chunkCount int
		err = db.conn.QueryRow(chunkCountSQL, name).Scan(&chunkCount)
		if err != nil {
			chunkCount = 0
		}

		collections = append(collections, map[string]interface{}{
			"name":        name,
			"description": description,
			"created_at":  createdAt,
			"doc_count":   docCount,
			"chunk_count": chunkCount,
		})
	}

	return collections, nil
}

func (db *VectorDB) DeleteCollection(name string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete embeddings for chunks in this collection
	_, err = tx.Exec(`DELETE FROM chunk_embeddings WHERE chunk_id IN (
		SELECT id FROM enhanced_chunks WHERE collection_name = ?
	)`, name)
	if err != nil {
		return fmt.Errorf("failed to delete chunk embeddings: %w", err)
	}

	// Delete chunks
	_, err = tx.Exec(`DELETE FROM enhanced_chunks WHERE collection_name = ?`, name)
	if err != nil {
		return fmt.Errorf("failed to delete chunks: %w", err)
	}

	// Delete documents
	_, err = tx.Exec(`DELETE FROM documents WHERE collection_name = ?`, name)
	if err != nil {
		return fmt.Errorf("failed to delete documents: %w", err)
	}

	// Delete collection
	result, err := tx.Exec(`DELETE FROM collections WHERE name = ?`, name)
	if err != nil {
		return fmt.Errorf("failed to delete collection: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("collection '%s' not found", name)
	}

	return tx.Commit()
}

// Document management methods
func (db *VectorDB) ListDocuments(collectionName string) ([]map[string]interface{}, error) {
	sql := `
		SELECT d.id, d.source, d.doc_type, d.created_at,
		       COUNT(c.id) as chunk_count,
		       MIN(c.created_at) as first_chunk_created,
		       MAX(c.created_at) as last_chunk_created
		FROM documents d
		LEFT JOIN enhanced_chunks c ON d.id = c.document_id AND c.collection_name = ?
		WHERE d.collection_name = ?
		GROUP BY d.id, d.source, d.doc_type, d.created_at
		ORDER BY d.created_at DESC`

	rows, err := db.conn.Query(sql, collectionName, collectionName)
	if err != nil {
		return nil, fmt.Errorf("failed to list documents: %w", err)
	}
	defer rows.Close()

	var documents []map[string]interface{}
	for rows.Next() {
		var id, source, docType, createdAt string
		var chunkCount int
		var firstChunkCreated, lastChunkCreated *string

		err := rows.Scan(&id, &source, &docType, &createdAt, &chunkCount, &firstChunkCreated, &lastChunkCreated)
		if err != nil {
			return nil, fmt.Errorf("failed to scan document: %w", err)
		}

		doc := map[string]interface{}{
			"id":          id,
			"source":      source,
			"doc_type":    docType,
			"created_at":  createdAt,
			"chunk_count": chunkCount,
		}

		if firstChunkCreated != nil {
			doc["first_chunk_created"] = *firstChunkCreated
		}
		if lastChunkCreated != nil {
			doc["last_chunk_created"] = *lastChunkCreated
		}

		documents = append(documents, doc)
	}

	return documents, nil
}

func (db *VectorDB) DeleteDocument(documentID string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Get document info for verification
	var source string
	err = tx.QueryRow(`SELECT source FROM documents WHERE id = ?`, documentID).Scan(&source)
	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("document with ID '%s' not found", documentID)
		}
		return fmt.Errorf("failed to find document: %w", err)
	}

	// Delete embeddings for chunks of this document
	_, err = tx.Exec(`DELETE FROM chunk_embeddings WHERE chunk_id IN (
		SELECT id FROM enhanced_chunks WHERE document_id = ?
	)`, documentID)
	if err != nil {
		return fmt.Errorf("failed to delete chunk embeddings: %w", err)
	}

	// Delete chunks
	result, err := tx.Exec(`DELETE FROM enhanced_chunks WHERE document_id = ?`, documentID)
	if err != nil {
		return fmt.Errorf("failed to delete chunks: %w", err)
	}

	chunksDeleted, _ := result.RowsAffected()

	// Delete document
	_, err = tx.Exec(`DELETE FROM documents WHERE id = ?`, documentID)
	if err != nil {
		return fmt.Errorf("failed to delete document: %w", err)
	}

	log.Printf("Deleted document '%s' (source: %s) and %d chunks", documentID, source, chunksDeleted)

	return tx.Commit()
}

func (db *VectorDB) DeleteAllDocumentsInCollection(collectionName string) error {
	tx, err := db.conn.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Count documents before deletion
	var docCount int
	err = tx.QueryRow(`SELECT COUNT(*) FROM documents WHERE collection_name = ?`, collectionName).Scan(&docCount)
	if err != nil {
		return fmt.Errorf("failed to count documents: %w", err)
	}

	if docCount == 0 {
		return fmt.Errorf("no documents found in collection '%s'", collectionName)
	}

	// Delete embeddings for chunks in this collection
	_, err = tx.Exec(`DELETE FROM chunk_embeddings WHERE chunk_id IN (
		SELECT id FROM enhanced_chunks WHERE collection_name = ?
	)`, collectionName)
	if err != nil {
		return fmt.Errorf("failed to delete chunk embeddings: %w", err)
	}

	// Delete chunks
	result, err := tx.Exec(`DELETE FROM enhanced_chunks WHERE collection_name = ?`, collectionName)
	if err != nil {
		return fmt.Errorf("failed to delete chunks: %w", err)
	}

	chunksDeleted, _ := result.RowsAffected()

	// Delete documents
	_, err = tx.Exec(`DELETE FROM documents WHERE collection_name = ?`, collectionName)
	if err != nil {
		return fmt.Errorf("failed to delete documents: %w", err)
	}

	log.Printf("Deleted %d documents and %d chunks from collection '%s'", docCount, chunksDeleted, collectionName)

	return tx.Commit()
}

func (db *VectorDB) GetCollectionStats(collectionName string) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Check if collection exists
	var exists bool
	err := db.conn.QueryRow(`SELECT EXISTS(SELECT 1 FROM collections WHERE name = ?)`, collectionName).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("failed to check collection existence: %w", err)
	}

	if !exists {
		return nil, fmt.Errorf("collection '%s' not found", collectionName)
	}

	// Get basic collection info
	var description, createdAt string
	err = db.conn.QueryRow(`SELECT description, created_at FROM collections WHERE name = ?`, collectionName).Scan(&description, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection info: %w", err)
	}

	stats["name"] = collectionName
	stats["description"] = description
	stats["created_at"] = createdAt

	// Count documents
	var docCount int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM documents WHERE collection_name = ?`, collectionName).Scan(&docCount)
	if err == nil {
		stats["document_count"] = docCount
	}

	// Count chunks
	var chunkCount int
	err = db.conn.QueryRow(`SELECT COUNT(*) FROM enhanced_chunks WHERE collection_name = ?`, collectionName).Scan(&chunkCount)
	if err == nil {
		stats["chunk_count"] = chunkCount
	}

	// Get chunk type distribution
	chunkTypeSQL := `SELECT chunk_type, COUNT(*) FROM enhanced_chunks WHERE collection_name = ? GROUP BY chunk_type ORDER BY COUNT(*) DESC`
	rows, err := db.conn.Query(chunkTypeSQL, collectionName)
	if err == nil {
		defer rows.Close()
		chunkTypes := make(map[string]int)
		for rows.Next() {
			var chunkType string
			var count int
			if err := rows.Scan(&chunkType, &count); err == nil {
				chunkTypes[chunkType] = count
			}
		}
		stats["chunk_types"] = chunkTypes
	}

	// Get document types
	docTypeSQL := `SELECT doc_type, COUNT(*) FROM documents WHERE collection_name = ? GROUP BY doc_type ORDER BY COUNT(*) DESC`
	rows, err = db.conn.Query(docTypeSQL, collectionName)
	if err == nil {
		defer rows.Close()
		docTypes := make(map[string]int)
		for rows.Next() {
			var docType string
			var count int
			if err := rows.Scan(&docType, &count); err == nil {
				docTypes[docType] = count
			}
		}
		stats["document_types"] = docTypes
	}

	return stats, nil
}

// Helper function to convert float32 slice to string slice
func float32SliceToStringSlice(floats []float32) []string {
	strings := make([]string, len(floats))
	for i, f := range floats {
		strings[i] = strconv.FormatFloat(float64(f), 'f', -1, 32)
	}
	return strings
}
