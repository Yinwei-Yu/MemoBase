"""gRPC servicer for TextProcessorService — CJK-aware tokenization and chunking."""

import logging
import time

import grpc

from proto.generated import agent_pb2, agent_pb2_grpc
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
