package core

import (
	"fmt"
	"log"
	"math"
	"rag-go-app/models"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

const (
	// Adaptive thresholds
	minMeaningfulChunkSize = 200  // Minimum chars for a meaningful chunk
	maxChunkSize           = 1500 // Maximum chunk size
	preferredChunkSize     = 800  // Preferred chunk size
	overlapRatio           = 0.15 // 15% overlap

	// Document size categories
	verySmallDoc = 1000  // < 1KB - keep as single chunk or minimal splits
	smallDoc     = 3000  // < 3KB - conservative chunking
	mediumDoc    = 10000 // < 10KB - normal chunking
	largeDoc     = 50000 // < 50KB - aggressive chunking

	// Minimum chunks before splitting
	minChunksThreshold = 3
)

// DocumentCharacteristics analyzes document properties
type DocumentCharacteristics struct {
	Length        int
	Category      DocumentCategory
	HasStructure  bool
	StructureType DocumentStructureType
	Language      string
	Complexity    float64
}

type DocumentCategory string
type DocumentStructureType string

const (
	VerySmallDocument DocumentCategory = "very_small"
	SmallDocument     DocumentCategory = "small"
	MediumDocument    DocumentCategory = "medium"
	LargeDocument     DocumentCategory = "large"
	VeryLargeDocument DocumentCategory = "very_large"

	NoStructure           DocumentStructureType = "none"
	SimpleStructure       DocumentStructureType = "simple"
	SectionedStructure    DocumentStructureType = "sectioned"
	HierarchicalStructure DocumentStructureType = "hierarchical"
)

// DocumentProcessor handles advanced document processing and chunking
type DocumentProcessor struct{}

// NewDocumentProcessor creates a new document processor
func NewDocumentProcessor() *DocumentProcessor {
	return &DocumentProcessor{}
}

// ProcessDocumentContent intelligently processes documents with adaptive strategies
func ProcessDocumentContent(content string, source string, docType string, config *models.ChunkingConfig) (*models.Document, error) {
	if content == "" {
		return nil, fmt.Errorf("content cannot be empty")
	}

	// Analyze document characteristics
	characteristics := analyzeDocument(content)

	// Override config with adaptive strategy if needed
	adaptiveConfig := adaptChunkingStrategy(characteristics, config)

	log.Printf("Document analysis: %d chars, category: %s, structure: %s, strategy: %s",
		characteristics.Length, characteristics.Category, characteristics.StructureType, adaptiveConfig.Strategy)

	doc := &models.Document{
		ID:      uuid.New().String(),
		Content: content,
		Source:  source,
		DocType: docType,
		Metadata: map[string]interface{}{
			"chunking_strategy": string(adaptiveConfig.Strategy),
			"document_length":   characteristics.Length,
			"document_category": string(characteristics.Category),
			"structure_type":    string(characteristics.StructureType),
			"chunk_count":       0, // Will be updated after chunking
		},
	}

	var chunks []*models.EnhancedChunk
	var err error

	// Apply the determined strategy
	switch adaptiveConfig.Strategy {
	case models.FixedSizeStrategy:
		chunks, err = createFixedSizeChunks(content, doc.ID, adaptiveConfig)
	case models.StructuralStrategy:
		chunks, err = createIntelligentStructuralChunks(content, doc.ID, adaptiveConfig, characteristics)
	case models.SemanticStrategy:
		chunks, err = createSemanticChunks(content, doc.ID, adaptiveConfig)
	case models.SentenceWindowStrategy:
		chunks, err = createSentenceWindowChunks(content, doc.ID, adaptiveConfig)
	case models.ParentDocumentStrategy:
		chunks, err = createParentDocumentChunks(content, doc.ID, adaptiveConfig)
	default:
		chunks, err = createIntelligentStructuralChunks(content, doc.ID, adaptiveConfig, characteristics)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create chunks: %w", err)
	}

	// Post-process chunks for quality
	chunks = postProcessChunks(chunks, characteristics)

	doc.Chunks = chunks
	doc.Metadata["chunk_count"] = len(chunks)

	log.Printf("Document processed: %d chunks created using %s strategy", len(chunks), adaptiveConfig.Strategy)
	return doc, nil
}

// analyzeDocument determines document characteristics
func analyzeDocument(content string) DocumentCharacteristics {
	length := len(content)

	var category DocumentCategory
	switch {
	case length < verySmallDoc:
		category = VerySmallDocument
	case length < smallDoc:
		category = SmallDocument
	case length < mediumDoc:
		category = MediumDocument
	case length < largeDoc:
		category = LargeDocument
	default:
		category = VeryLargeDocument
	}

	// Analyze structure
	structureType, hasStructure := analyzeStructure(content)

	// Calculate complexity (sentence length, vocabulary diversity, etc.)
	complexity := calculateComplexity(content)

	return DocumentCharacteristics{
		Length:        length,
		Category:      category,
		HasStructure:  hasStructure,
		StructureType: structureType,
		Language:      "en", // Could be enhanced with language detection
		Complexity:    complexity,
	}
}

// analyzeStructure detects document structure patterns
func analyzeStructure(content string) (DocumentStructureType, bool) {
	// Check for hierarchical patterns (multiple heading levels)
	hierarchicalPatterns := []string{
		`(?m)^#+\s+`,            // Markdown headers
		`(?m)^[A-Z][A-Z\s]+:?$`, // ALL CAPS sections
		`(?m)^\d+\.\s+[A-Z]`,    // Numbered sections
		`(?m)^[IVX]+\.\s+`,      // Roman numerals
	}

	structureCount := 0
	for _, pattern := range hierarchicalPatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			structureCount++
		}
	}

	// Count section-like patterns
	sectionPatterns := []string{
		`(?i)\b(experience|education|skills|summary|objective|projects|achievements|awards|certifications|languages|references|contact|about)\b`,
		`(?m)^[A-Z][A-Z\s]{3,}:?\s*$`,
		`(?m)^.{1,50}:$`,
	}

	sectionCount := 0
	for _, pattern := range sectionPatterns {
		re := regexp.MustCompile(pattern)
		sectionCount += len(re.FindAllString(content, -1))
	}

	// Determine structure type
	if structureCount >= 3 || sectionCount >= 5 {
		return HierarchicalStructure, true
	} else if structureCount >= 1 || sectionCount >= 2 {
		return SectionedStructure, true
	} else if strings.Count(content, "\n\n") >= 3 {
		return SimpleStructure, true
	}

	return NoStructure, false
}

// calculateComplexity estimates document complexity
func calculateComplexity(content string) float64 {
	words := strings.Fields(content)
	if len(words) == 0 {
		return 0.0
	}

	sentences := strings.Split(content, ".")
	avgWordsPerSentence := float64(len(words)) / float64(len(sentences))

	// Simple complexity score based on sentence length
	complexity := math.Min(avgWordsPerSentence/15.0, 1.0)
	return complexity
}

// adaptChunkingStrategy chooses the best strategy based on document characteristics
func adaptChunkingStrategy(characteristics DocumentCharacteristics, config *models.ChunkingConfig) *models.ChunkingConfig {
	if config == nil {
		config = &models.ChunkingConfig{}
	}

	// Copy existing config
	adaptiveConfig := *config

	// Calculate optimal chunk count based on document length
	optimalChunkCount := calculateOptimalChunkCount(characteristics.Length)

	log.Printf("Document length: %d chars, optimal chunk count: %d", characteristics.Length, optimalChunkCount)

	// Override strategy based on document characteristics with smart thresholds
	switch characteristics.Category {
	case VerySmallDocument:
		// For very small documents (< 1000 chars), be very conservative
		if characteristics.Length < 600 {
			// Single chunk for very small documents
			adaptiveConfig.Strategy = models.FixedSizeStrategy
			adaptiveConfig.FixedSize = characteristics.Length
			adaptiveConfig.Overlap = 0
			adaptiveConfig.MinChunkSize = characteristics.Length
			log.Printf("Very small document: keeping as single chunk")
		} else {
			// Max 2-3 chunks for small documents
			adaptiveConfig.Strategy = models.StructuralStrategy
			adaptiveConfig.MinChunkSize = characteristics.Length / 3 // Ensure max 3 chunks
			adaptiveConfig.MaxChunkSize = characteristics.Length / 2 // Ensure min 2 chunks
			if adaptiveConfig.MinChunkSize < 250 {
				adaptiveConfig.MinChunkSize = 250
			}
			log.Printf("Small document: conservative chunking with min=%d, max=%d",
				adaptiveConfig.MinChunkSize, adaptiveConfig.MaxChunkSize)
		}

	case SmallDocument:
		// For small documents (1-3KB), aim for 3-5 meaningful chunks
		targetChunkSize := characteristics.Length / optimalChunkCount
		if targetChunkSize < 400 {
			targetChunkSize = 400
		}

		if characteristics.HasStructure {
			adaptiveConfig.Strategy = models.StructuralStrategy
			adaptiveConfig.MinChunkSize = targetChunkSize
			adaptiveConfig.MaxChunkSize = targetChunkSize + 300
		} else {
			adaptiveConfig.Strategy = models.SentenceWindowStrategy
			adaptiveConfig.SentenceWindowSize = 4
			adaptiveConfig.MinChunkSize = targetChunkSize
		}
		log.Printf("Small document: targeting %d chunks with size ~%d", optimalChunkCount, targetChunkSize)

	case MediumDocument:
		// For medium documents, use normal strategies
		if characteristics.StructureType == HierarchicalStructure {
			adaptiveConfig.Strategy = models.ParentDocumentStrategy
		} else if characteristics.HasStructure {
			adaptiveConfig.Strategy = models.StructuralStrategy
		} else {
			adaptiveConfig.Strategy = models.SemanticStrategy
		}

	case LargeDocument, VeryLargeDocument:
		// For large documents, use aggressive chunking
		adaptiveConfig.Strategy = models.ParentDocumentStrategy
		adaptiveConfig.MaxChunkSize = 1200
		adaptiveConfig.MinChunkSize = 400
	}

	// Set intelligent defaults based on document size
	if adaptiveConfig.MinChunkSize == 0 {
		if characteristics.Length < 2000 {
			adaptiveConfig.MinChunkSize = characteristics.Length / 4 // Max 4 chunks for small docs
			if adaptiveConfig.MinChunkSize < 200 {
				adaptiveConfig.MinChunkSize = 200
			}
		} else {
			adaptiveConfig.MinChunkSize = minMeaningfulChunkSize
		}
	}

	if adaptiveConfig.MaxChunkSize == 0 {
		if characteristics.Length < 3000 {
			adaptiveConfig.MaxChunkSize = characteristics.Length / 2 // Min 2 chunks for small docs
		} else {
			adaptiveConfig.MaxChunkSize = maxChunkSize
		}
	}

	if adaptiveConfig.FixedSize == 0 {
		if characteristics.Length < 2000 {
			adaptiveConfig.FixedSize = characteristics.Length / optimalChunkCount
		} else {
			adaptiveConfig.FixedSize = preferredChunkSize
		}
	}

	if adaptiveConfig.Overlap == 0 {
		// Reduce overlap for very small documents
		if characteristics.Length < 1500 {
			adaptiveConfig.Overlap = int(float64(adaptiveConfig.FixedSize) * 0.1) // 10% overlap
		} else {
			adaptiveConfig.Overlap = int(float64(adaptiveConfig.FixedSize) * overlapRatio)
		}
	}

	adaptiveConfig.PreserveParagraphs = true
	adaptiveConfig.ExtractKeywords = true

	return &adaptiveConfig
}

// calculateOptimalChunkCount determines the ideal number of chunks for a document
func calculateOptimalChunkCount(length int) int {
	switch {
	case length < 600:
		return 1 // Single chunk
	case length < 1200:
		return 2 // Two chunks
	case length < 2000:
		return 3 // Three chunks
	case length < 4000:
		return 4 // Four chunks
	case length < 8000:
		return int(math.Ceil(float64(length) / 1500)) // ~1500 chars per chunk
	default:
		return int(math.Ceil(float64(length) / 1000)) // ~1000 chars per chunk for larger docs
	}
}

// createIntelligentStructuralChunks creates context-aware structural chunks
func createIntelligentStructuralChunks(content string, docID string, config *models.ChunkingConfig, characteristics DocumentCharacteristics) ([]*models.EnhancedChunk, error) {
	var chunks []*models.EnhancedChunk

	// For very small documents, prefer minimal chunking
	if characteristics.Category == VerySmallDocument {
		return createMinimalChunks(content, docID, config)
	}

	// Detect sections and create meaningful chunks
	sections := detectSections(content)

	chunkIndex := 0
	for _, section := range sections {
		sectionChunks := createSectionChunks(section, docID, config, &chunkIndex)
		chunks = append(chunks, sectionChunks...)
	}

	// If no meaningful sections found, fall back to sentence-based chunking
	if len(chunks) == 0 {
		return createSentenceWindowChunks(content, docID, config)
	}

	return chunks, nil
}

// createMinimalChunks for very small documents
func createMinimalChunks(content string, docID string, config *models.ChunkingConfig) ([]*models.EnhancedChunk, error) {
	// For very small content, create just 1-2 meaningful chunks
	if len(content) <= config.MinChunkSize {
		// Single chunk
		chunk := &models.EnhancedChunk{
			ID:         uuid.New().String(),
			DocumentID: docID,
			Text:       strings.TrimSpace(content),
			ChunkType:  "document",
			Section:    "complete",
			StartPos:   0,
			EndPos:     len(content),
			ChunkIndex: 0,
		}

		if config.ExtractKeywords {
			chunk.Keywords = extractKeywords(chunk.Text)
		}

		return []*models.EnhancedChunk{chunk}, nil
	}

	// Split into 2-3 meaningful parts based on paragraphs or sentences
	paragraphs := strings.Split(content, "\n\n")
	if len(paragraphs) < 2 {
		// Fall back to sentence splitting
		return createSentenceWindowChunks(content, docID, config)
	}

	// Group paragraphs into meaningful chunks
	var chunks []*models.EnhancedChunk
	currentChunk := ""
	chunkIndex := 0
	startPos := 0

	for i, para := range paragraphs {
		testChunk := currentChunk
		if testChunk != "" {
			testChunk += "\n\n"
		}
		testChunk += para

		if len(testChunk) >= config.MinChunkSize || i == len(paragraphs)-1 {
			chunk := &models.EnhancedChunk{
				ID:         uuid.New().String(),
				DocumentID: docID,
				Text:       strings.TrimSpace(testChunk),
				ChunkType:  "paragraph_group",
				Section:    fmt.Sprintf("section_%d", chunkIndex+1),
				StartPos:   startPos,
				EndPos:     startPos + len(testChunk),
				ChunkIndex: chunkIndex,
			}

			if config.ExtractKeywords {
				chunk.Keywords = extractKeywords(chunk.Text)
			}

			chunks = append(chunks, chunk)
			chunkIndex++
			startPos += len(testChunk) + 2 // +2 for \n\n
			currentChunk = ""
		} else {
			currentChunk = testChunk
		}
	}

	return chunks, nil
}

// postProcessChunks ensures chunk quality and adds relationships
func postProcessChunks(chunks []*models.EnhancedChunk, characteristics DocumentCharacteristics) []*models.EnhancedChunk {
	// Remove too-small chunks by merging with neighbors
	filteredChunks := []*models.EnhancedChunk{}

	for i, chunk := range chunks {
		if len(chunk.Text) < minMeaningfulChunkSize/2 && i < len(chunks)-1 {
			// Merge with next chunk
			nextChunk := chunks[i+1]
			nextChunk.Text = chunk.Text + "\n\n" + nextChunk.Text
			nextChunk.StartPos = chunk.StartPos
			if len(chunk.Keywords) > 0 {
				nextChunk.Keywords = append(nextChunk.Keywords, chunk.Keywords...)
			}
			// Skip current chunk
			continue
		}
		filteredChunks = append(filteredChunks, chunk)
	}

	// Add parent-child relationships for larger documents
	if characteristics.Category == LargeDocument || characteristics.Category == VeryLargeDocument {
		filteredChunks = addParentChildRelationships(filteredChunks)
	}

	return filteredChunks
}

// Enhanced detectSections function
func detectSections(content string) []DocumentSection {
	var sections []DocumentSection

	// Enhanced section detection patterns
	sectionPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)^([A-Z][A-Z\s]{2,}):?\s*$`), // ALL CAPS sections
		regexp.MustCompile(`(?i)^(EXPERIENCE|EDUCATION|SKILLS|SUMMARY|OBJECTIVE|PROJECTS|ACHIEVEMENTS|AWARDS|CERTIFICATIONS|LANGUAGES|REFERENCES|CONTACT|ABOUT).*$`), // Common resume sections
		regexp.MustCompile(`(?m)^#+\s+(.+)$`),       // Markdown headers
		regexp.MustCompile(`(?m)^(\d+\.\s+.+)$`),    // Numbered sections
		regexp.MustCompile(`(?m)^([IVX]+\.\s+.+)$`), // Roman numeral sections
	}

	lines := strings.Split(content, "\n")
	currentSection := DocumentSection{Title: "document", StartLine: 0}

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		isSection := false
		var sectionTitle string

		for _, pattern := range sectionPatterns {
			if matches := pattern.FindStringSubmatch(line); len(matches) > 1 {
				isSection = true
				sectionTitle = matches[1]
				break
			}
		}

		if isSection {
			// Save previous section
			if currentSection.StartLine < i {
				currentSection.EndLine = i - 1
				currentSection.Content = strings.Join(lines[currentSection.StartLine:i], "\n")
				if strings.TrimSpace(currentSection.Content) != "" {
					sections = append(sections, currentSection)
				}
			}

			// Start new section
			currentSection = DocumentSection{
				Title:     sectionTitle,
				StartLine: i,
			}
		}
	}

	// Add final section
	if currentSection.StartLine < len(lines) {
		currentSection.EndLine = len(lines) - 1
		currentSection.Content = strings.Join(lines[currentSection.StartLine:], "\n")
		if strings.TrimSpace(currentSection.Content) != "" {
			sections = append(sections, currentSection)
		}
	}

	// If no sections detected, treat whole document as one section
	if len(sections) == 0 {
		sections = []DocumentSection{{
			Title:     "document",
			Content:   content,
			StartLine: 0,
			EndLine:   len(lines) - 1,
		}}
	}

	return sections
}

// DocumentSection represents a detected section
type DocumentSection struct {
	Title     string
	Content   string
	StartLine int
	EndLine   int
}

// createSectionChunks creates chunks from a document section
func createSectionChunks(section DocumentSection, docID string, config *models.ChunkingConfig, chunkIndex *int) []*models.EnhancedChunk {
	var chunks []*models.EnhancedChunk

	content := strings.TrimSpace(section.Content)
	if content == "" {
		return chunks
	}

	// If section is small enough, keep as single chunk
	if len(content) <= config.MaxChunkSize {
		chunk := &models.EnhancedChunk{
			ID:         uuid.New().String(),
			DocumentID: docID,
			Text:       content,
			Section:    section.Title,
			ChunkType:  "section",
			StartPos:   0,
			EndPos:     len(content),
			ChunkIndex: *chunkIndex,
		}

		if config.ExtractKeywords {
			chunk.Keywords = extractKeywords(content)
		}

		chunks = append(chunks, chunk)
		*chunkIndex++
		return chunks
	}

	// Split large sections into meaningful chunks
	paragraphs := strings.Split(content, "\n\n")
	currentChunk := ""
	startPos := 0

	for i, para := range paragraphs {
		testChunk := currentChunk
		if testChunk != "" {
			testChunk += "\n\n"
		}
		testChunk += para

		shouldChunk := len(testChunk) >= config.MinChunkSize &&
			(len(testChunk) >= config.MaxChunkSize || i == len(paragraphs)-1)

		if shouldChunk {
			chunk := &models.EnhancedChunk{
				ID:         uuid.New().String(),
				DocumentID: docID,
				Text:       strings.TrimSpace(testChunk),
				Section:    section.Title,
				ChunkType:  "section_part",
				StartPos:   startPos,
				EndPos:     startPos + len(testChunk),
				ChunkIndex: *chunkIndex,
			}

			if config.ExtractKeywords {
				chunk.Keywords = extractKeywords(testChunk)
			}

			chunks = append(chunks, chunk)
			*chunkIndex++
			startPos += len(testChunk) + 2
			currentChunk = ""
		} else {
			currentChunk = testChunk
		}
	}

	return chunks
}

// Keep all existing helper functions but enhance them...
// (createFixedSizeChunks, createSemanticChunks, etc. - existing implementations)

// addParentChildRelationships creates hierarchical chunk relationships
func addParentChildRelationships(chunks []*models.EnhancedChunk) []*models.EnhancedChunk {
	// Group chunks by section
	sectionGroups := make(map[string][]*models.EnhancedChunk)

	for _, chunk := range chunks {
		section := chunk.Section
		if section == "" {
			section = "document"
		}
		sectionGroups[section] = append(sectionGroups[section], chunk)
	}

	var enhancedChunks []*models.EnhancedChunk

	for section, sectionChunks := range sectionGroups {
		if len(sectionChunks) > 2 {
			// Create parent chunk for section
			combinedText := ""
			var childIDs []string

			for i, chunk := range sectionChunks {
				if i > 0 {
					combinedText += "\n\n"
				}
				combinedText += chunk.Text
				childIDs = append(childIDs, chunk.ID)
			}

			parentChunk := &models.EnhancedChunk{
				ID:            uuid.New().String(),
				DocumentID:    sectionChunks[0].DocumentID,
				Text:          combinedText,
				Section:       section,
				ChunkType:     "parent",
				ChildChunkIDs: childIDs,
				ChunkIndex:    sectionChunks[0].ChunkIndex,
			}

			enhancedChunks = append(enhancedChunks, parentChunk)

			// Update child chunks
			for _, chunk := range sectionChunks {
				chunk.ParentChunkID = &parentChunk.ID
				enhancedChunks = append(enhancedChunks, chunk)
			}
		} else {
			enhancedChunks = append(enhancedChunks, sectionChunks...)
		}
	}

	return enhancedChunks
}

// Enhanced keyword extraction
func extractKeywords(text string) []string {
	if text == "" {
		return []string{}
	}

	// Clean text
	text = strings.ToLower(text)

	// Remove common stop words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true, "but": true,
		"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
		"with": true, "by": true, "is": true, "are": true, "was": true, "were": true,
		"be": true, "been": true, "have": true, "has": true, "had": true, "do": true,
		"does": true, "did": true, "will": true, "would": true, "could": true, "should": true,
		"this": true, "that": true, "these": true, "those": true, "i": true, "you": true,
		"he": true, "she": true, "it": true, "we": true, "they": true, "my": true,
		"your": true, "his": true, "her": true, "its": true, "our": true, "their": true,
	}

	// Extract words
	words := regexp.MustCompile(`\b[a-zA-Z]{3,}\b`).FindAllString(text, -1)

	// Count frequency and filter
	wordCount := make(map[string]int)
	for _, word := range words {
		word = strings.ToLower(word)
		if !stopWords[word] && len(word) >= 3 {
			wordCount[word]++
		}
	}

	// Sort by frequency
	type wordFreq struct {
		word  string
		count int
	}

	var frequencies []wordFreq
	for word, count := range wordCount {
		frequencies = append(frequencies, wordFreq{word, count})
	}

	sort.Slice(frequencies, func(i, j int) bool {
		return frequencies[i].count > frequencies[j].count
	})

	// Return top keywords
	var keywords []string
	maxKeywords := 10
	for i, wf := range frequencies {
		if i >= maxKeywords {
			break
		}
		keywords = append(keywords, wf.word)
	}

	return keywords
}

// Keep existing implementations for other strategies...
// These would be implemented with similar intelligence and quality controls

// createFixedSizeChunks creates chunks of fixed size with intelligent overlaps
func createFixedSizeChunks(content string, docID string, config *models.ChunkingConfig) ([]*models.EnhancedChunk, error) {
	var chunks []*models.EnhancedChunk

	if len(content) <= config.FixedSize {
		// Single chunk
		chunk := &models.EnhancedChunk{
			ID:         uuid.New().String(),
			DocumentID: docID,
			Text:       strings.TrimSpace(content),
			ChunkType:  "fixed_size",
			Section:    "document",
			StartPos:   0,
			EndPos:     len(content),
			ChunkIndex: 0,
		}

		if config.ExtractKeywords {
			chunk.Keywords = extractKeywords(chunk.Text)
		}

		return []*models.EnhancedChunk{chunk}, nil
	}

	// Create fixed-size chunks with overlap
	start := 0
	chunkIndex := 0

	for start < len(content) {
		end := start + config.FixedSize
		if end > len(content) {
			end = len(content)
		}

		// Try to end at word boundary
		if end < len(content) && !unicode.IsSpace(rune(content[end])) {
			// Find last space within reasonable distance
			for i := end; i > start+config.FixedSize-50 && i > start; i-- {
				if unicode.IsSpace(rune(content[i])) {
					end = i
					break
				}
			}
		}

		chunkText := strings.TrimSpace(content[start:end])
		if len(chunkText) > 0 {
			chunk := &models.EnhancedChunk{
				ID:         uuid.New().String(),
				DocumentID: docID,
				Text:       chunkText,
				ChunkType:  "fixed_size",
				Section:    "document",
				StartPos:   start,
				EndPos:     end,
				ChunkIndex: chunkIndex,
			}

			if config.ExtractKeywords {
				chunk.Keywords = extractKeywords(chunkText)
			}

			chunks = append(chunks, chunk)
			chunkIndex++
		}

		// Move start position with overlap
		start = end - config.Overlap
		if start >= end {
			break
		}
	}

	return chunks, nil
}

// createSemanticChunks creates chunks based on semantic boundaries
func createSemanticChunks(content string, docID string, config *models.ChunkingConfig) ([]*models.EnhancedChunk, error) {
	// For now, fall back to paragraph-based chunking with semantic awareness
	paragraphs := strings.Split(content, "\n\n")
	var chunks []*models.EnhancedChunk

	currentChunk := ""
	chunkIndex := 0
	startPos := 0

	for i, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		testChunk := currentChunk
		if testChunk != "" {
			testChunk += "\n\n"
		}
		testChunk += para

		shouldChunk := len(testChunk) >= config.MinChunkSize &&
			(len(testChunk) >= config.MaxChunkSize || i == len(paragraphs)-1)

		if shouldChunk {
			chunk := &models.EnhancedChunk{
				ID:         uuid.New().String(),
				DocumentID: docID,
				Text:       strings.TrimSpace(testChunk),
				ChunkType:  "semantic",
				Section:    "content",
				StartPos:   startPos,
				EndPos:     startPos + len(testChunk),
				ChunkIndex: chunkIndex,
			}

			if config.ExtractKeywords {
				chunk.Keywords = extractKeywords(testChunk)
			}

			chunks = append(chunks, chunk)
			chunkIndex++
			startPos += len(testChunk) + 2
			currentChunk = ""
		} else {
			currentChunk = testChunk
		}
	}

	return chunks, nil
}

// createSentenceWindowChunks creates overlapping sentence windows
func createSentenceWindowChunks(content string, docID string, config *models.ChunkingConfig) ([]*models.EnhancedChunk, error) {
	// Split into sentences
	sentences := regexp.MustCompile(`[.!?]+\s+`).Split(content, -1)
	var chunks []*models.EnhancedChunk

	windowSize := config.SentenceWindowSize
	if windowSize == 0 {
		windowSize = 3 // Default
	}

	chunkIndex := 0

	for i := 0; i < len(sentences); i += windowSize / 2 { // 50% overlap
		end := i + windowSize
		if end > len(sentences) {
			end = len(sentences)
		}

		windowText := strings.Join(sentences[i:end], ". ")
		windowText = strings.TrimSpace(windowText)

		if len(windowText) < config.MinChunkSize && i+windowSize < len(sentences) {
			continue // Skip if too small and not last
		}

		if len(windowText) > 0 {
			chunk := &models.EnhancedChunk{
				ID:         uuid.New().String(),
				DocumentID: docID,
				Text:       windowText,
				ChunkType:  "sentence_window",
				Section:    "content",
				StartPos:   0, // Could calculate actual positions
				EndPos:     len(windowText),
				ChunkIndex: chunkIndex,
			}

			if config.ExtractKeywords {
				chunk.Keywords = extractKeywords(windowText)
			}

			chunks = append(chunks, chunk)
			chunkIndex++
		}

		// Break if we've reached the end
		if end >= len(sentences) {
			break
		}
	}

	return chunks, nil
}

// createParentDocumentChunks creates hierarchical parent-child chunks
func createParentDocumentChunks(content string, docID string, config *models.ChunkingConfig) ([]*models.EnhancedChunk, error) {
	// First create large parent chunks
	parentSize := config.MaxChunkSize * 2 // Parents are larger
	var parentChunks []*models.EnhancedChunk
	var allChunks []*models.EnhancedChunk

	// Create parent chunks
	start := 0
	parentIndex := 0

	for start < len(content) {
		end := start + parentSize
		if end > len(content) {
			end = len(content)
		}

		// Try to end at paragraph boundary
		if end < len(content) {
			for i := end; i > start+parentSize-200 && i > start; i-- {
				if content[i:i+2] == "\n\n" {
					end = i
					break
				}
			}
		}

		parentText := strings.TrimSpace(content[start:end])
		if len(parentText) > 0 {
			parentChunk := &models.EnhancedChunk{
				ID:         uuid.New().String(),
				DocumentID: docID,
				Text:       parentText,
				ChunkType:  "parent",
				Section:    fmt.Sprintf("section_%d", parentIndex+1),
				StartPos:   start,
				EndPos:     end,
				ChunkIndex: parentIndex,
			}

			if config.ExtractKeywords {
				parentChunk.Keywords = extractKeywords(parentText)
			}

			parentChunks = append(parentChunks, parentChunk)

			// Create child chunks from this parent
			childChunks, err := createFixedSizeChunks(parentText, docID, &models.ChunkingConfig{
				Strategy:        models.FixedSizeStrategy,
				FixedSize:       config.MinChunkSize,
				Overlap:         config.Overlap / 2,
				ExtractKeywords: config.ExtractKeywords,
			})

			if err != nil {
				return nil, err
			}

			// Link children to parent
			var childIDs []string
			for _, child := range childChunks {
				child.ParentChunkID = &parentChunk.ID
				child.Section = parentChunk.Section
				child.ChunkType = "child"
				childIDs = append(childIDs, child.ID)
			}

			parentChunk.ChildChunkIDs = childIDs
			allChunks = append(allChunks, childChunks...)
			parentIndex++
		}

		start = end
	}

	// Add parents at the beginning
	result := append(parentChunks, allChunks...)
	return result, nil
}
