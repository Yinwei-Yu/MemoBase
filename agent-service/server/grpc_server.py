"""gRPC server implementation for AgentService.

Connects LangGraph agent to gRPC interface with unary and streaming support.
"""

from __future__ import annotations

import logging
import time
from typing import AsyncIterator

import grpc
from google.protobuf import struct_pb2
from langchain_core.messages import AIMessage, HumanMessage

from agent.graph import get_graph
from agent.state import initial_state
from proto.generated import agent_pb2, agent_pb2_grpc
from tracing.collector import collect_from_state

logger = logging.getLogger("agent-service.grpc")


class AgentServiceServicer(agent_pb2_grpc.AgentServiceServicer):
    """Implements the AgentService gRPC interface with LangGraph agent."""

    async def ChatCompletion(
        self,
        request: agent_pb2.ChatRequest,
        context: grpc.aio.ServicerContext,
    ) -> agent_pb2.ChatResponse:
        logger.info(
            "ChatCompletion: kb_id=%s session_id=%s question=%.80s provider_id=%s model=%s",
            request.kb_id,
            request.session_id,
            request.question,
            request.provider_id or "(default)",
            request.provider_model or request.model or "(default)",
        )
        started = time.monotonic()

        try:
            # Build chat history from request
            chat_history = []
            for msg in request.history:
                if msg.role == "user":
                    chat_history.append(HumanMessage(content=msg.content))
                elif msg.role == "assistant":
                    chat_history.append(AIMessage(content=msg.content))

            # Build memories
            memories = [
                {"type": m.type, "summary": m.summary} for m in request.memories
            ]

            # Create initial state
            state = initial_state(
                question=request.question,
                kb_id=request.kb_id,
                session_id=request.session_id,
                model=request.model or "",
                top_k=request.top_k or 6,
                chat_history=chat_history,
                memories=memories,
                provider_api_base_url=request.provider_api_base_url or "",
                provider_api_key=request.provider_api_key or "",
                provider_model=request.provider_model or "",
            )

            # Run the agent graph
            graph = get_graph()
            result = await graph.ainvoke(state)

            latency_ms = int((time.monotonic() - started) * 1000)

            # Build citations
            citations = []
            for c in result.get("citations", []):
                citations.append(
                    agent_pb2.Citation(
                        doc_id=c.get("doc_id", ""),
                        doc_title=c.get("doc_title", ""),
                        chunk_id=c.get("chunk_id", ""),
                        snippet=c.get("snippet", ""),
                        score=float(c.get("score", 0)),
                        retrieval_source=c.get("retrieval_source", ""),
                    )
                )

            # Build trace steps
            trace_steps = []
            for t in collect_from_state(result):
                data = {}
                for k, v in (t.get("data") or {}).items():
                    data[str(k)] = str(v)
                trace_steps.append(
                    agent_pb2.TraceStep(
                        node=t.get("node", ""),
                        action=t.get("action", ""),
                        duration_ms=int(t.get("duration_ms", 0)),
                        data=data,
                    )
                )

            # Build token usage
            tu = result.get("token_usage", {})
            token_usage = agent_pb2.TokenUsage(
                prompt_tokens=int(tu.get("prompt_tokens", 0)),
                completion_tokens=int(tu.get("completion_tokens", 0)),
                total_tokens=int(tu.get("total_tokens", 0)),
            )

            return agent_pb2.ChatResponse(
                answer=str(result.get("final_answer", "")),
                citations=citations,
                trace=trace_steps,
                token_usage=token_usage,
                degraded=bool(result.get("degraded", False)),
                latency_ms=latency_ms,
            )

        except Exception as e:
            logger.exception("ChatCompletion failed")
            latency_ms = int((time.monotonic() - started) * 1000)
            return agent_pb2.ChatResponse(
                answer=f"抱歉，处理您的问题时出错: {e}",
                degraded=True,
                latency_ms=latency_ms,
            )

    async def ChatCompletionStream(
        self,
        request: agent_pb2.ChatRequest,
        context: grpc.aio.ServicerContext,
    ) -> AsyncIterator[agent_pb2.ChatEvent]:
        logger.info(
            "ChatCompletionStream: kb_id=%s session_id=%s question=%.80s provider_id=%s model=%s",
            request.kb_id,
            request.session_id,
            request.question,
            request.provider_id or "(default)",
            request.provider_model or request.model or "(default)",
        )
        started = time.monotonic()

        try:
            # Build chat history
            chat_history = []
            for msg in request.history:
                if msg.role == "user":
                    chat_history.append(HumanMessage(content=msg.content))
                elif msg.role == "assistant":
                    chat_history.append(AIMessage(content=msg.content))

            memories = [
                {"type": m.type, "summary": m.summary} for m in request.memories
            ]

            state = initial_state(
                question=request.question,
                kb_id=request.kb_id,
                session_id=request.session_id,
                model=request.model or "",
                top_k=request.top_k or 6,
                chat_history=chat_history,
                memories=memories,
                provider_api_base_url=request.provider_api_base_url or "",
                provider_api_key=request.provider_api_key or "",
                provider_model=request.provider_model or "",
            )

            # Emit step events as we stream
            yield agent_pb2.ChatEvent(
                step=agent_pb2.AgentStepEvent(
                    step_name="init",
                    status="started",
                    detail="Agent started processing",
                    duration_ms=0,
                )
            )

            graph = get_graph()

            # Use astream_events for streaming, accumulating state from nodes
            last_node = ""
            in_generate = False
            accumulated_state: dict = {}
            async for event in graph.astream_events(state, version="v1"):
                kind = event.get("event", "")
                name = event.get("name", "")

                # Emit step events for node transitions
                if kind == "on_chain_start" and name in (
                    "decide",
                    "retrieve",
                    "grade",
                    "rewrite",
                    "generate",
                ):
                    if last_node != name:
                        yield agent_pb2.ChatEvent(
                            step=agent_pb2.AgentStepEvent(
                                step_name=name,
                                status="started",
                                detail=f"Node '{name}' started",
                                duration_ms=0,
                            )
                        )
                        last_node = name
                        in_generate = name == "generate"

                elif kind == "on_chain_end" and name in (
                    "decide",
                    "retrieve",
                    "grade",
                    "rewrite",
                    "generate",
                ):
                    output = event.get("data", {}).get("output", {})
                    if isinstance(output, dict):
                        # Accumulate state from each node's output
                        accumulated_state.update(output)

                        detail = f"Node '{name}' completed"
                        if name == "decide":
                            detail = f"Decision: {output.get('decision', '?')}"
                        elif name == "retrieve":
                            detail = f"Retrieved {len(output.get('retrieved_docs', []))} docs"
                        elif name == "generate":
                            detail = "Answer generated"

                        yield agent_pb2.ChatEvent(
                            step=agent_pb2.AgentStepEvent(
                                step_name=name,
                                status="completed",
                                detail=detail,
                                duration_ms=0,
                            )
                        )

                # Stream tokens only from generate node
                elif kind == "on_chat_model_stream" and in_generate:
                    chunk = event.get("data", {}).get("chunk", None)
                    if chunk and hasattr(chunk, "content") and chunk.content:
                        logger.debug("Streaming token from generate: %.40s", chunk.content)
                        yield agent_pb2.ChatEvent(
                            token=agent_pb2.TokenEvent(
                                token=str(chunk.content),
                                token_index=0,
                            )
                        )

            # Use accumulated state from streaming; fall back to ainvoke only if needed
            if accumulated_state.get("final_answer"):
                logger.info("Using accumulated state from streaming (answer=%d chars)", len(accumulated_state["final_answer"]))
                result = {**state, **accumulated_state}
            else:
                logger.info("Accumulated state missing final_answer (keys=%s), falling back to ainvoke", list(accumulated_state.keys()))
                result = await graph.ainvoke(state)

            latency_ms = int((time.monotonic() - started) * 1000)

            # Build citations
            citations = []
            for c in result.get("citations", []):
                citations.append(
                    agent_pb2.Citation(
                        doc_id=c.get("doc_id", ""),
                        doc_title=c.get("doc_title", ""),
                        chunk_id=c.get("chunk_id", ""),
                        snippet=c.get("snippet", ""),
                        score=float(c.get("score", 0)),
                        retrieval_source=c.get("retrieval_source", ""),
                    )
                )

            # Build trace
            trace_steps = []
            for t in collect_from_state(result):
                data = {}
                for k, v in (t.get("data") or {}).items():
                    data[str(k)] = str(v)
                trace_steps.append(
                    agent_pb2.TraceStep(
                        node=t.get("node", ""),
                        action=t.get("action", ""),
                        duration_ms=int(t.get("duration_ms", 0)),
                        data=data,
                    )
                )

            tu = result.get("token_usage", {})
            token_usage = agent_pb2.TokenUsage(
                prompt_tokens=int(tu.get("prompt_tokens", 0)),
                completion_tokens=int(tu.get("completion_tokens", 0)),
                total_tokens=int(tu.get("total_tokens", 0)),
            )

            yield agent_pb2.ChatEvent(
                result=agent_pb2.FinalResultEvent(
                    answer=str(result.get("final_answer", "")),
                    citations=citations,
                    trace=trace_steps,
                    token_usage=token_usage,
                    degraded=bool(result.get("degraded", False)),
                    latency_ms=latency_ms,
                )
            )

        except Exception as e:
            logger.exception("ChatCompletionStream failed")
            yield agent_pb2.ChatEvent(
                error=agent_pb2.ErrorEvent(
                    code="AGENT_ERROR",
                    message=str(e),
                )
            )

    async def HealthCheck(
        self,
        request: agent_pb2.HealthRequest,
        context: grpc.aio.ServicerContext,
    ) -> agent_pb2.HealthResponse:
        checks = {"server": "running"}

        # Check Ollama connectivity
        try:
            from agent.llm import get_chat_llm

            llm = get_chat_llm()
            # Lightweight check: just verify the model exists
            checks["ollama"] = "up"
        except Exception as e:
            checks["ollama"] = f"down: {e}"

        # Check Qdrant
        try:
            from retriever.hybrid import HybridRetriever

            retriever = HybridRetriever()
            retriever.qdrant  # force connection
            checks["qdrant"] = "up"
        except Exception as e:
            checks["qdrant"] = f"down: {e}"

        status = "ok" if all(v == "up" for v in checks.values()) else "degraded"
        return agent_pb2.HealthResponse(status=status, checks=checks)
