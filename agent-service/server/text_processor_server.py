"""gRPC servicer for TextProcessorService — CJK-aware tokenization and chunking."""

import logging
import time
import uuid

import grpc
import psycopg2
from qdrant_client.models import FieldCondition, Filter, MatchValue

from agent.llm import get_embeddings
from config import settings
from proto.generated import agent_pb2, agent_pb2_grpc
from retriever.hybrid import HybridRetriever
from text_processor import chunk_document, tokenize

logger = logging.getLogger("agent-service.text_processor")


class TextProcessorServiceServicer(agent_pb2_grpc.TextProcessorServiceServicer):
    """Implements the TextProcessorService gRPC interface."""

    async def Tokenize(
        self,
        request: agent_pb2.TokenizeRequest,
        context: grpc.aio.ServicerContext,
    ) -> agent_pb2.TokenizeResponse:
        started = time.monotonic()
        tokens = tokenize(request.text)
        elapsed = (time.monotonic() - started) * 1000
        logger.debug("Tokenize: %d chars → %d tokens in %.1fms", len(request.text), len(tokens), elapsed)
        return agent_pb2.TokenizeResponse(tokens=tokens)

    async def ChunkDocument(
        self,
        request: agent_pb2.ChunkRequest,
        context: grpc.aio.ServicerContext,
    ) -> agent_pb2.ChunkResponse:
        started = time.monotonic()
        md_aware = request.markdown_aware if request.HasField("markdown_aware") else True
        chunks = chunk_document(
            content=request.content,
            max_chunk_size=request.max_chunk_size or 500,
            overlap=request.overlap or 100,
            markdown_aware=md_aware,
        )
        elapsed = (time.monotonic() - started) * 1000
        logger.debug("ChunkDocument: %d chars → %d chunks in %.1fms", len(request.content), len(chunks), elapsed)
        return agent_pb2.ChunkResponse(chunks=chunks)

    async def TextHealthCheck(
        self,
        request: agent_pb2.HealthRequest,
        context: grpc.aio.ServicerContext,
    ) -> agent_pb2.HealthResponse:
        return agent_pb2.HealthResponse(
            status="ok",
            checks={"text_processor": "running"},
        )

    async def RetrieveChunks(
        self,
        request: agent_pb2.RetrieveChunksRequest,
        context: grpc.aio.ServicerContext,
    ) -> agent_pb2.RetrieveChunksResponse:
        from agent.tools import get_retriever

        started = time.monotonic()
        retriever = get_retriever()
        results, degraded = retriever.search(
            query=request.query,
            kb_id=request.kb_id,
            top_k=request.top_k or 6,
        )
        elapsed = (time.monotonic() - started) * 1000
        logger.debug(
            "RetrieveChunks: kb=%s query=%.40s → %d results in %.1fms",
            request.kb_id, request.query, len(results), elapsed,
        )

        chunks = [
            agent_pb2.RetrievedChunk(
                chunk_id=r["chunk_id"],
                doc_id=r.get("doc_id", ""),
                content=r.get("content", ""),
                score=r.get("score", 0.0),
                source=r.get("source", "unknown"),
            )
            for r in results
        ]
        return agent_pb2.RetrieveChunksResponse(chunks=chunks, degraded=degraded)

    async def InvalidateCache(
        self,
        request: agent_pb2.InvalidateCacheRequest,
        context: grpc.aio.ServicerContext,
    ) -> agent_pb2.InvalidateCacheResponse:
        from agent.tools import get_retriever

        retriever = get_retriever()
        retriever.invalidate_cache(request.kb_id)
        logger.info("InvalidateCache: kb=%s cleared", request.kb_id)
        return agent_pb2.InvalidateCacheResponse(cleared=True)

    async def IndexDocument(
        self,
        request: agent_pb2.IndexDocumentRequest,
        context: grpc.aio.ServicerContext,
    ) -> agent_pb2.IndexDocumentResponse:
        """Chunk + embed + store to PG + upsert to Qdrant."""
        started = time.monotonic()
        kb_id = request.kb_id
        doc_id = request.doc_id
        content = request.content
        chunk_size = request.chunk_size or 500
        overlap = request.overlap or 100
        collection = request.collection_name

        logger.info(
            "IndexDocument: kb=%s doc=%s collection=%s content_len=%d",
            kb_id, doc_id, collection, len(content),
        )

        try:
            # 1. Chunk
            chunks = chunk_document(content, max_chunk_size=chunk_size, overlap=overlap, markdown_aware=True)
            if not chunks:
                return agent_pb2.IndexDocumentResponse(
                    chunk_count=0, chunk_ids=[], success=False, error_message="empty document after chunking",
                )

            # 2. Generate chunk IDs and store to PostgreSQL
            chunk_ids = []
            db_rows = []
            for i, chunk_text in enumerate(chunks):
                cid = f"ck_{uuid.uuid4()}"
                chunk_ids.append(cid)
                db_rows.append((cid, doc_id, kb_id, i, chunk_text))

            conn = psycopg2.connect(settings.database_url)
            cur = conn.cursor()
            # Delete old chunks for this document first
            cur.execute("DELETE FROM document_chunks WHERE doc_id = %s", (doc_id,))
            cur.executemany(
                """INSERT INTO document_chunks (id, doc_id, kb_id, chunk_index, content)
                   VALUES (%s, %s, %s, %s, %s)""",
                db_rows,
            )
            conn.commit()
            cur.close()
            conn.close()

            # 3. Embed and upsert to Qdrant
            embeddings = get_embeddings()
            retriever = HybridRetriever()
            qdrant = retriever.qdrant

            # Ensure collection exists
            first_vec = embeddings.embed_query(chunks[0])
            expected_dim = len(first_vec)

            from qdrant_client.models import Distance, VectorParams
            try:
                collection_info = qdrant.get_collection(collection)
                existing_dim = None
                if collection_info.config.params.vectors:
                    existing_dim = collection_info.config.params.vectors.size
                if existing_dim and existing_dim != expected_dim:
                    return agent_pb2.IndexDocumentResponse(
                        chunk_count=0, chunk_ids=[], success=False,
                        error_message=f"embedding dim mismatch: collection={existing_dim} model={expected_dim}",
                    )
            except Exception:
                qdrant.create_collection(
                    collection_name=collection,
                    vectors_config=VectorParams(size=expected_dim, distance=Distance.COSINE),
                )

            # Build points: first chunk already embedded
            from qdrant_client.models import PointStruct

            points = [PointStruct(
                id=str(uuid.uuid5(uuid.NAMESPACE_OID, f"chunk:{chunk_ids[0]}")),
                vector=first_vec,
                payload={"kb_id": kb_id, "doc_id": doc_id, "chunk_id": chunk_ids[0], "chunk_index": 0, "source": "document"},
            )]

            for i in range(1, len(chunks)):
                vec = embeddings.embed_query(chunks[i])
                points.append(PointStruct(
                    id=str(uuid.uuid5(uuid.NAMESPACE_OID, f"chunk:{chunk_ids[i]}")),
                    vector=vec,
                    payload={"kb_id": kb_id, "doc_id": doc_id, "chunk_id": chunk_ids[i], "chunk_index": i, "source": "document"},
                ))

            # Delete old points for this doc, then upsert
            qdrant.delete(collection_name=collection, points_selector=Filter(
                must=[FieldCondition(key="doc_id", match=MatchValue(value=doc_id))],
            ))
            qdrant.upsert(collection_name=collection, points=points)

            # Invalidate BM25 cache
            retriever.invalidate_cache(kb_id)

            elapsed = int((time.monotonic() - started) * 1000)
            logger.info("IndexDocument: done, %d chunks in %dms", len(chunks), elapsed)

            return agent_pb2.IndexDocumentResponse(
                chunk_count=len(chunks),
                chunk_ids=chunk_ids,
                success=True,
            )

        except Exception as e:
            logger.exception("IndexDocument failed")
            return agent_pb2.IndexDocumentResponse(
                chunk_count=0, chunk_ids=[], success=False, error_message=str(e),
            )

    async def DeleteDocumentVectors(
        self,
        request: agent_pb2.DeleteDocVectorsRequest,
        context: grpc.aio.ServicerContext,
    ) -> agent_pb2.DeleteDocVectorsResponse:
        """Delete all Qdrant vectors for a document."""
        try:
            retriever = HybridRetriever()
            qdrant = retriever.qdrant
            qdrant.delete(
                collection_name=request.collection_name,
                points_selector=Filter(
                    must=[FieldCondition(key="doc_id", match=MatchValue(value=request.doc_id))],
                ),
            )
            logger.info("DeleteDocumentVectors: collection=%s doc=%s", request.collection_name, request.doc_id)
            return agent_pb2.DeleteDocVectorsResponse(success=True)
        except Exception as e:
            logger.exception("DeleteDocumentVectors failed")
            return agent_pb2.DeleteDocVectorsResponse(success=False)

    async def DeleteKBCollection(
        self,
        request: agent_pb2.DeleteKBCollectionRequest,
        context: grpc.aio.ServicerContext,
    ) -> agent_pb2.DeleteKBCollectionResponse:
        """Delete entire Qdrant collection for a KB."""
        try:
            retriever = HybridRetriever()
            qdrant = retriever.qdrant
            qdrant.delete_collection(collection_name=request.collection_name)
            retriever.invalidate_cache(request.collection_name)
            logger.info("DeleteKBCollection: collection=%s", request.collection_name)
            return agent_pb2.DeleteKBCollectionResponse(success=True)
        except Exception as e:
            logger.exception("DeleteKBCollection failed")
            return agent_pb2.DeleteKBCollectionResponse(success=False)
