# 🔍 Search-Only Endpoint Documentation

## Overview

The `/api/v1/search` endpoint provides **retrieval-only functionality** without LLM generation. This is perfect for separating concerns, allowing external services to handle the LLM part while leveraging our advanced RAG retrieval system.

## Endpoint Details

- **URL**: `POST /api/v1/search`
- **Purpose**: Semantic search and retrieval without LLM generation
- **Content-Type**: `application/json`

## Request Format

```json
{
  "collection_name": "string (required)",
  "query": "string (required)",
  "top_k": "number (optional, default: 5)",
  "semantic_threshold": "number (optional, 0.0-1.0)",
  "metadata_filters": "object (optional)"
}
```

### Request Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `collection_name` | string | ✅ | Name of the collection to search |
| `query` | string | ✅ | Search query text |
| `top_k` | number | ❌ | Number of results to return (default: 5) |
| `semantic_threshold` | number | ❌ | Minimum similarity score (0.0-1.0) |
| `metadata_filters` | object | ❌ | Key-value filters for metadata |

## Response Format

```json
{
  "query": "original query text",
  "collection_name": "collection name",
  "chunks_found": "number of chunks returned",
  "chunks": [
    {
      "id": "unique chunk ID",
      "document_id": "parent document ID",
      "text": "chunk content",
      "section": "document section",
      "subsection": "document subsection",
      "chunk_type": "type of chunk",
      "start_pos": "start position in document",
      "end_pos": "end position in document",
      "chunk_index": "chunk order index",
      "keywords": ["extracted", "keywords"],
      "confidence": "confidence score",
      "similarity_score": "semantic similarity score",
      "parent_chunk_id": "parent chunk ID (if applicable)",
      "child_chunk_ids": ["child chunk IDs"],
      "metadata": "chunk metadata object"
    }
  ],
  "context": "concatenated context string for LLM",
  "context_strings": ["individual context strings"],
  "processing_time": "processing time in seconds",
  "metadata": {
    "semantic_threshold": "applied threshold",
    "metadata_filters": "applied filters",
    "filters_applied": "boolean indicating if filters were used",
    "note": "information about advanced features"
  },
  "score_statistics": {
    "min_similarity": "lowest similarity score",
    "max_similarity": "highest similarity score", 
    "avg_similarity": "average similarity score",
    "total_scores": "number of scores"
  }
}
```

## Usage Examples

### Basic Search

```bash
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "test_resumes",
    "query": "What is GUVI",
    "top_k": 3
  }'
```

### Search with Semantic Threshold

```bash
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "documents",
    "query": "machine learning experience",
    "top_k": 5,
    "semantic_threshold": 0.1
  }'
```

### Search with Metadata Filters

```bash
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "resumes",
    "query": "software engineer",
    "top_k": 10,
    "metadata_filters": {
      "experience_level": "senior",
      "location": "San Francisco"
    }
  }'
```

## Response Examples

### Successful Search Response

```json
{
  "query": "What is GUVI",
  "collection_name": "test_resumes",
  "chunks_found": 1,
  "chunks": [
    {
      "id": "c02bf9ef-b85d-4780-9490-8c88c783f53b",
      "document_id": "50e974e9-1d5a-4603-9ac9-b334b1a197d1",
      "text": "EXPERIENCE\n\nGUVI GEEK NETWORK\nPRODUCT ENGINEER (FOUNDING TEAM)\nJan 2022 - Present\n• Led the development of the company core product, increasing user engagement by 40%.\n• Implemented microservices architecture reducing system downtime by 60%.\n• Mentored junior developers and established coding standards.",
      "section": "GUVI GEEK NETWORK",
      "subsection": "",
      "chunk_type": "section",
      "start_pos": 0,
      "end_pos": 298,
      "chunk_index": 6,
      "keywords": ["product", "system", "junior", "engineer", "standards", "geek", "led", "implemented", "increasing", "coding", "experience"],
      "confidence": 0,
      "similarity_score": 0.046425819396972656
    }
  ],
  "context": "Context 1: EXPERIENCE\n\nGUVI GEEK NETWORK\nPRODUCT ENGINEER (FOUNDING TEAM)\nJan 2022 - Present\n• Led the development of the company core product, increasing user engagement by 40%.\n• Implemented microservices architecture reducing system downtime by 60%.\n• Mentored junior developers and established coding standards.",
  "context_strings": [
    "Context 1: EXPERIENCE\n\nGUVI GEEK NETWORK\nPRODUCT ENGINEER (FOUNDING TEAM)\nJan 2022 - Present\n• Led the development of the company core product, increasing user engagement by 40%.\n• Implemented microservices architecture reducing system downtime by 60%.\n• Mentored junior developers and established coding standards."
  ],
  "processing_time": 0.073841959,
  "metadata": {
    "semantic_threshold": 0,
    "metadata_filters": null,
    "filters_applied": false,
    "note": "Advanced features available in /api/v1/query endpoint"
  },
  "score_statistics": {
    "min_similarity": 0.046425819396972656,
    "max_similarity": 0.046425819396972656,
    "avg_similarity": 0.046425819396972656,
    "total_scores": 1
  }
}
```

### No Results Response

```json
{
  "query": "nonexistent topic",
  "collection_name": "test_resumes",
  "chunks_found": 0,
  "chunks": [],
  "context": "",
  "processing_time": 0.023451234,
  "metadata": {
    "semantic_threshold": 0,
    "metadata_filters": null,
    "filters_applied": false,
    "note": "Advanced features available in /api/v1/query endpoint"
  }
}
```

## Key Features

### ✅ **Pure Retrieval**
- No LLM processing overhead
- Fast response times
- Raw search results

### ✅ **Rich Metadata**
- Complete chunk information
- Similarity scores
- Hierarchical relationships
- Processing statistics

### ✅ **Multiple Output Formats**
- Individual chunk details
- Ready-to-use context strings
- Concatenated context for LLMs

### ✅ **Advanced Filtering**
- Semantic similarity thresholds
- Metadata-based filtering
- Configurable result limits

### ✅ **Adaptive Chunking Benefits**
- Leverages the intelligent chunking system
- Context-aware chunk relationships
- Optimal chunk sizes for better retrieval

## Use Cases

### 🎯 **External LLM Processing**
Perfect for sending context to external LLM services:
```javascript
const searchResponse = await fetch('/api/v1/search', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({
    collection_name: 'documents',
    query: userQuery,
    top_k: 5
  })
});

const { context } = await searchResponse.json();
// Send context to external LLM service
```

### 🎯 **Custom RAG Pipelines**
Build custom processing pipelines:
- Retrieve with `/search`
- Apply custom ranking/filtering
- Process with custom LLM chains
- Add custom post-processing

### 🎯 **Analysis & Debugging**
Understand retrieval quality:
- Inspect similarity scores
- Analyze chunk relevance
- Debug search performance
- Tune threshold parameters

### 🎯 **Multi-Modal Processing**
Combine with other AI services:
- Retrieve text context
- Generate images based on context
- Create audio summaries
- Build custom applications

## Performance Benefits

- **Faster Response**: No LLM generation overhead
- **Scalable**: Handle more concurrent requests
- **Flexible**: Use any LLM service or model
- **Cost-Effective**: Separate expensive LLM calls

## Error Handling

### Common Error Responses

```json
{
  "error": "Collection name is required"
}
```

```json
{
  "error": "Failed to generate query embedding"
}
```

```json
{
  "error": "Failed to search similar chunks"
}
```

## Comparison with Full RAG Endpoint

| Feature | `/search` | `/query` |
|---------|-----------|----------|
| **Retrieval** | ✅ Full | ✅ Full |
| **LLM Generation** | ❌ None | ✅ Included |
| **Speed** | ⚡ Fast | 🐢 Slower |
| **Context Output** | ✅ Raw | ✅ + Answer |
| **Custom Processing** | ✅ Flexible | ❌ Fixed |
| **External LLM** | ✅ Perfect | ❌ Redundant |

## Best Practices

1. **Use appropriate `top_k`** values (3-10 for most cases)
2. **Set semantic thresholds** to filter low-quality matches
3. **Leverage metadata filters** for precise targeting
4. **Monitor processing times** for performance optimization
5. **Use `context_strings`** for structured LLM prompts
6. **Check `score_statistics`** for result quality assessment

The search endpoint provides the perfect foundation for building custom RAG applications while leveraging our advanced adaptive chunking system! 🚀 