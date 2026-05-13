"""MCP client for optional Qdrant MCP server integration.

This provides an alternative retrieval path via MCP (Model Context Protocol).
When enabled, MCP tools can be registered alongside custom tools in the agent graph.
"""

from __future__ import annotations

import logging
from typing import Any

from config import settings

logger = logging.getLogger("agent-service.mcp")


class MCPQdrantClient:
    """Client for the Qdrant MCP server.

    Uses SSE transport to connect to the MCP server and exposes
    search tools that can be bound to the LangGraph agent.
    """

    def __init__(self, url: str | None = None):
        self.url = url or settings.mcp_qdrant_url
        self._enabled = settings.mcp_qdrant_enabled
        self._tools: list[dict[str, Any]] = []

        if self._enabled:
            logger.info("MCP Qdrant client configured: url=%s", self.url)
        else:
            logger.info("MCP Qdrant client disabled")

    @property
    def enabled(self) -> bool:
        return self._enabled

    async def connect(self) -> None:
        """Connect to the MCP server and discover tools."""
        if not self._enabled:
            return

        try:
            from langchain_mcp_adapters.client import MultiServerMCPClient

            self._client = MultiServerMCPClient(
                {
                    "qdrant": {
                        "transport": "sse",
                        "url": self.url,
                    }
                }
            )
            self._tools = await self._client.list_tools()
            logger.info("MCP connected: %d tools discovered", len(self._tools))
        except Exception as e:
            logger.warning("MCP connection failed: %s (continuing without MCP)", e)
            self._enabled = False

    async def get_tools(self) -> list[Any]:
        """Get LangChain-compatible tools from the MCP server."""
        if not self._enabled:
            return []

        try:
            return await self._client.get_tools()
        except Exception as e:
            logger.warning("Failed to get MCP tools: %s", e)
            return []

    async def close(self) -> None:
        """Close the MCP client connection."""
        if hasattr(self, "_client"):
            try:
                await self._client.close()
            except Exception:
                pass


# Module-level singleton
_mcp_client: MCPQdrantClient | None = None


def get_mcp_client() -> MCPQdrantClient:
    """Get or create the global MCP client."""
    global _mcp_client
    if _mcp_client is None:
        _mcp_client = MCPQdrantClient()
    return _mcp_client
