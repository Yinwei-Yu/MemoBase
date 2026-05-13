"""Trace collector for agent graph execution events.

Collects node-level events from the agent graph and produces
TraceStep protobuf-compatible records.
"""

from __future__ import annotations

import logging
import time
from typing import Any

logger = logging.getLogger("agent-service.tracing")


class TraceCollector:
    """Collects trace events during agent graph execution."""

    def __init__(self) -> None:
        self._steps: list[dict[str, Any]] = []
        self._start_time: float = time.monotonic()

    def add_step(
        self,
        node: str,
        action: str,
        duration_ms: int | None = None,
        data: dict[str, str] | None = None,
    ) -> None:
        """Add a trace step."""
        self._steps.append(
            {
                "node": node,
                "action": action,
                "duration_ms": duration_ms or 0,
                "data": data or {},
            }
        )

    @property
    def steps(self) -> list[dict[str, Any]]:
        return list(self._steps)

    @property
    def total_latency_ms(self) -> int:
        return int((time.monotonic() - self._start_time) * 1000)

    def to_proto_trace_steps(self) -> list[dict[str, Any]]:
        """Convert collected steps to proto-compatible TraceStep dicts."""
        return [
            {
                "node": s["node"],
                "action": s["action"],
                "duration_ms": s["duration_ms"],
                "data": {str(k): str(v) for k, v in s["data"].items()},
            }
            for s in self._steps
        ]


def collect_from_state(state: dict[str, Any]) -> list[dict[str, Any]]:
    """Extract trace steps from agent state.

    Use this after graph execution to get trace steps that were
    accumulated via operator.add in trace_steps.
    """
    return state.get("trace_steps", [])
