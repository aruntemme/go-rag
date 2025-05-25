# 📚 RAG System API Reference

Complete API documentation with curl examples for the Advanced RAG System.

## 🎯 Quick Reference

| Endpoint | Method | Purpose | Speed |
|----------|--------|---------|-------|
| `/health` | GET | Health check | ⚡ Instant |
| `/api/v1/collections` | POST/GET/DELETE | Manage collections | ⚡ Fast |
| `/api/v1/documents` | POST/GET/DELETE | Manage documents | 🐢 Processing |
| `/api/v1/search` | POST | **Pure retrieval** | ⚡ Fast |
| `/api/v1/query` | POST | **Full RAG** | 🐢 LLM dependent |
| `/api/v1/analyze` | POST | Document analysis | 🐢 LLM dependent |
| `/api/v1/compare-chunking` | POST | Strategy comparison | 🐢 Processing |

---

## 🏥 Health Check

### Check Server Status
```bash
curl -X GET http://localhost:8080/health
```

**Response:**
```json
{
  "status": "healthy",
  "service": "rag-go-app"
}
```

---

## 📚 Collection Management

### Create Collection
```bash
curl -X POST http://localhost:8080/api/v1/collections \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my_documents",
    "description": "My document collection"
  }'
```

**Response:**
```json
{
  "message": "Collection created successfully",
  "name": "my_documents",
  "description": "My document collection"
}
```

### List All Collections
```bash
curl -X GET http://localhost:8080/api/v1/collections
```

**Response:**
```json
{
  "collections": [
    {
      "name": "my_documents",
      "description": "My document collection",
      "created_at": "2024-01-15T10:30:00Z",
      "doc_count": 3,
      "chunk_count": 45
    }
  ],
  "total": 1
}
```

### Get Collection Statistics
```bash
curl -X GET http://localhost:8080/api/v1/collections/my_documents
```

**Response:**
```json
{
  "name": "my_documents",
  "description": "My document collection",
  "created_at": "2024-01-15T10:30:00Z",
  "document_count": 3,
  "chunk_count": 45,
  "chunk_types": {
    "section": 30,
    "job_entry": 10,
    "content": 5
  },
  "document_types": {
    "resume": 2,
    "manual": 1
  }
}
```

### Delete Collection
```bash
curl -X DELETE http://localhost:8080/api/v1/collections/my_documents
```

**Response:**
```json
{
  "message": "Collection deleted successfully",
  "collection_name": "my_documents"
}
```

---

## 📄 Document Management

### Add Document (Basic)
```bash
curl -X POST http://localhost:8080/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "my_documents",
    "content": "This is a sample document about machine learning and AI.",
    "source": "sample.txt"
  }'
```

### Add Document (Advanced with Chunking Config)
```bash
curl -X POST http://localhost:8080/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "my_documents",
    "content": "Your document content here...",
    "source": "document.txt",
    "doc_type": "resume",
    "chunking_config": {
      "strategy": "structural",
      "fixed_size": 500,
      "overlap": 50,
      "min_chunk_size": 100,
      "max_chunk_size": 2000,
      "preserve_paragraphs": true,
      "extract_keywords": true
    }
  }'
```

### Add Document from File Path
```bash
curl -X POST http://localhost:8080/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "my_documents",
    "file_path": "./sample_document.txt",
    "source": "uploaded_file"
  }'
```

**Response:**
```json
{
  "message": "Document added successfully",
  "collection_name": "my_documents",
  "chunking_strategy": "structural",
  "source": "document.txt"
}
```

### List Documents in Collection
```bash
curl -X GET http://localhost:8080/api/v1/collections/my_documents/documents
```

**Response:**
```json
{
  "collection_name": "my_documents",
  "documents": [
    {
      "id": "af94d028-b7b6-49de-8978-c5e504c269c7",
      "source": "resume.txt",
      "doc_type": "resume",
      "created_at": "2024-01-15T10:30:00Z",
      "chunk_count": 15,
      "first_chunk_created": "2024-01-15 10:30:01",
      "last_chunk_created": "2024-01-15 10:30:05"
    }
  ],
  "total": 1
}
```

### Delete Specific Document
```bash
curl -X DELETE http://localhost:8080/api/v1/documents/af94d028-b7b6-49de-8978-c5e504c269c7
```

**Response:**
```json
{
  "message": "Document deleted successfully",
  "document_id": "af94d028-b7b6-49de-8978-c5e504c269c7"
}
```

### Delete All Documents in Collection
```bash
curl -X DELETE "http://localhost:8080/api/v1/collections/my_documents/documents?confirm=true"
```

**Response:**
```json
{
  "message": "All documents deleted successfully",
  "collection_name": "my_documents"
}
```

---

## 🔍 Search & Query

### Search Only (No LLM) - Basic
```bash
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "my_documents",
    "query": "What is machine learning?",
    "top_k": 5
  }'
```

### Search Only (No LLM) - Advanced
```bash
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "my_documents",
    "query": "machine learning experience",
    "top_k": 10,
    "semantic_threshold": 0.3,
    "metadata_filters": {
      "section": "experience",
      "chunk_type": "job_entry"
    }
  }'
```

**Search Response:**
```json
{
  "query": "machine learning experience",
  "collection_name": "my_documents",
  "chunks_found": 3,
  "chunks": [
    {
      "id": "chunk-uuid",
      "document_id": "doc-uuid",
      "text": "Machine Learning Engineer at TechCorp...",
      "section": "Experience",
      "subsection": "TechCorp",
      "chunk_type": "job_entry",
      "start_pos": 120,
      "end_pos": 450,
      "chunk_index": 3,
      "keywords": ["machine", "learning", "engineer", "python"],
      "confidence": 0.95,
      "similarity_score": 0.87,
      "metadata": {
        "position": "Machine Learning Engineer",
        "company": "TechCorp"
      }
    }
  ],
  "context": "Context 1: Machine Learning Engineer at TechCorp...",
  "context_strings": [
    "Context 1: Machine Learning Engineer at TechCorp..."
  ],
  "processing_time": 0.056,
  "metadata": {
    "semantic_threshold": 0.3,
    "metadata_filters": {"section": "experience"},
    "filters_applied": true,
    "note": "Advanced features available in /api/v1/query endpoint"
  },
  "score_statistics": {
    "min_similarity": 0.65,
    "max_similarity": 0.87,
    "avg_similarity": 0.76,
    "total_scores": 3
  }
}
```

### Full RAG Query - Basic
```bash
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "my_documents",
    "query": "What programming skills do I have?",
    "top_k": 5
  }'
```

### Full RAG Query - Advanced
```bash
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "my_documents",
    "query": "What team lead positions have I held?",
    "top_k": 10,
    "reranker_enabled": true,
    "include_parents": true,
    "query_expansion": true,
    "semantic_threshold": 0.2,
    "metadata_filters": {
      "chunk_type": "job_entry",
      "section": "experience"
    }
  }'
```

**Query Response:**
```json
{
  "answer": "Based on your resume, you have held the following team lead positions: Senior Software Engineer Team Lead at TechCorp (2020-2022) where you led a team of 5 developers...",
  "retrieved_context": [
    "EXPERIENCE\n\nTechCorp\nSENIOR SOFTWARE ENGINEER TEAM LEAD\n2020-2022\n• Led team of 5 developers..."
  ],
  "enhanced_chunks": [
    {
      "id": "chunk-uuid",
      "text": "TechCorp\nSENIOR SOFTWARE ENGINEER TEAM LEAD...",
      "section": "Experience",
      "chunk_type": "job_entry",
      "keywords": ["team", "lead", "senior", "engineer"],
      "confidence": 0.95,
      "metadata": {
        "position": "Senior Software Engineer Team Lead",
        "company": "TechCorp"
      }
    }
  ],
  "similarity_scores": [0.89],
  "reranked_scores": [0.92],
  "processing_time": 2.34,
  "metadata_used": true
}
```

---

## 📊 Analysis & Comparison

### Document Analysis
```bash
curl -X POST http://localhost:8080/api/v1/analyze \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "my_documents",
    "query": "Tell me about my experience",
    "show_metadata": true
  }'
```

**Response:**
```json
{
  "query": "Tell me about my experience",
  "answer": "Based on your documents...",
  "processing_time": 1.85,
  "chunks_found": 5,
  "reranking_applied": true,
  "parent_chunks_included": true,
  "query_expansion": true,
  "chunk_analysis": [
    {
      "chunk_id": "chunk-uuid",
      "chunk_type": "job_entry",
      "section": "Experience",
      "subsection": "TechCorp",
      "text_length": 245,
      "keywords": ["team", "lead", "python", "aws"],
      "similarity_score": 0.89,
      "reranked_score": 0.92,
      "has_parent": false,
      "child_count": 2
    }
  ]
}
```

### Compare Chunking Strategies
```bash
curl -X POST http://localhost:8080/api/v1/compare-chunking \
  -H "Content-Type: application/json" \
  -d '{
    "content": "EXPERIENCE\n\nTechCorp\nSoftware Engineer\n2020-2022\nDeveloped web applications using React and Node.js\n\nSkills: JavaScript, Python, AWS",
    "doc_type": "resume",
    "strategies": ["fixed_size", "structural", "semantic"]
  }'
```

**Response:**
```json
{
  "content_length": 156,
  "doc_type": "resume",
  "strategies": [
    {
      "strategy": "fixed_size",
      "chunk_count": 2,
      "chunks": [
        {
          "id": "chunk-1",
          "text_length": 100,
          "chunk_type": "fixed_size",
          "section": "document"
        }
      ]
    },
    {
      "strategy": "structural",
      "chunk_count": 3,
      "chunks": [
        {
          "id": "chunk-1",
          "text_length": 45,
          "chunk_type": "section",
          "section": "EXPERIENCE",
          "keywords": ["experience"]
        }
      ]
    }
  ]
}
```

---

## 📝 Request Schemas

### Document Schema
```json
{
  "collection_name": "string (required)",
  "content": "string (optional - direct content)",
  "file_path": "string (optional - file path)",
  "source": "string (optional - identifier)",
  "doc_type": "string (optional - resume, manual, etc.)",
  "chunking_config": {
    "strategy": "structural|fixed_size|semantic|sentence_window|parent_document",
    "fixed_size": 500,
    "overlap": 50,
    "min_chunk_size": 100,
    "max_chunk_size": 2000,
    "preserve_paragraphs": true,
    "extract_keywords": true
  }
}
```

### Search Schema
```json
{
  "collection_name": "string (required)",
  "query": "string (required)",
  "top_k": 5,
  "semantic_threshold": 0.0,
  "metadata_filters": {
    "section": "string",
    "chunk_type": "string",
    "doc_type": "string"
  }
}
```

### Query Schema
```json
{
  "collection_name": "string (required)",
  "query": "string (required)",
  "top_k": 5,
  "reranker_enabled": true,
  "include_parents": false,
  "query_expansion": true,
  "semantic_threshold": 0.1,
  "metadata_filters": {
    "section": "skills",
    "chunk_type": "job_entry"
  }
}
```

---

## 🚨 Error Responses

### 400 Bad Request
```json
{
  "error": "Collection name is required"
}
```

### 404 Not Found
```json
{
  "error": "Document with ID 'doc-123' not found"
}
```

### 500 Internal Server Error
```json
{
  "error": "Failed to generate query embedding"
}
```

---

## 🎯 Use Case Examples

### Resume/CV Processing
```bash
# 1. Add resume
curl -X POST http://localhost:8080/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "resumes",
    "file_path": "./john_doe_resume.txt",
    "source": "john_doe_resume"
  }'

# 2. Find leadership experience
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "resumes",
    "query": "team lead management experience",
    "top_k": 5,
    "metadata_filters": {
      "chunk_type": "job_entry"
    }
  }'

# 3. Generate experience summary
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "resumes",
    "query": "Summarize my leadership and management experience",
    "reranker_enabled": true,
    "query_expansion": true
  }'
```

### Technical Documentation
```bash
# 1. Add documentation
curl -X POST http://localhost:8080/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "tech_docs",
    "content": "API Documentation\n\nAuthentication\nUse JWT tokens...",
    "source": "api_docs"
  }'

# 2. Search for specific information
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "tech_docs",
    "query": "how to authenticate API requests",
    "top_k": 3,
    "semantic_threshold": 0.2
  }'
```

### Multi-Document Analysis
```bash
# 1. Query across all documents
curl -X POST http://localhost:8080/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{
    "collection_name": "knowledge_base",
    "query": "What are the best practices for user authentication?",
    "top_k": 8,
    "include_parents": true,
    "query_expansion": true
  }'

# 2. Compare chunking strategies before upload
curl -X POST http://localhost:8080/api/v1/compare-chunking \
  -H "Content-Type: application/json" \
  -d '{
    "content": "Your long document content...",
    "strategies": ["structural", "semantic", "parent_document"]
  }'
```

---

## 🔄 Workflow Examples

### Development Workflow
```bash
# 1. Health check
curl -X GET http://localhost:8080/health

# 2. Create collection
curl -X POST http://localhost:8080/api/v1/collections \
  -d '{"name": "dev_docs", "description": "Development documentation"}'

# 3. Add documents
curl -X POST http://localhost:8080/api/v1/documents \
  -d '{"collection_name": "dev_docs", "content": "..."}'

# 4. Test search
curl -X POST http://localhost:8080/api/v1/search \
  -d '{"collection_name": "dev_docs", "query": "test query"}'

# 5. Analyze results
curl -X POST http://localhost:8080/api/v1/analyze \
  -d '{"collection_name": "dev_docs", "query": "test", "show_metadata": true}'
```

### Production Workflow
```bash
# 1. Upload documents
for file in docs/*.txt; do
  curl -X POST http://localhost:8080/api/v1/documents \
    -d "{\"collection_name\": \"production\", \"file_path\": \"$file\"}"
done

# 2. Production search (fast)
curl -X POST http://localhost:8080/api/v1/search \
  -d '{"collection_name": "production", "query": "user question"}'

# 3. Send context to external LLM service
# (use the context from search response)
```

---

## 📈 Performance Tips

1. **Use `/search` for external LLM processing** - 500x faster than `/query`
2. **Set semantic thresholds** to filter low-quality results
3. **Use metadata filters** for precise targeting
4. **Leverage adaptive chunking** by not specifying chunking_config
5. **Monitor processing times** in responses for optimization

For more detailed information, see:
- [SEARCH_ENDPOINT.md](SEARCH_ENDPOINT.md) - Search endpoint details
- [ADAPTIVE_CHUNKING.md](ADAPTIVE_CHUNKING.md) - Chunking system guide 