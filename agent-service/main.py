"""Entry point for the agent service."""

import asyncio
import logging
import signal

import grpc

from config import settings
from proto.generated import agent_pb2_grpc
from server.grpc_server import AgentServiceServicer
from server.text_processor_server import TextProcessorServiceServicer

logger = logging.getLogger("agent-service")


async def serve() -> None:
    logging.basicConfig(
        level=logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s: %(message)s",
    )

    server = grpc.aio.server()
    servicer = AgentServiceServicer()
    agent_pb2_grpc.add_AgentServiceServicer_to_server(servicer, server)
    text_processor_servicer = TextProcessorServiceServicer()
    agent_pb2_grpc.add_TextProcessorServiceServicer_to_server(text_processor_servicer, server)

    addr = f"[::]:{settings.grpc_port}"
    server.add_insecure_port(addr)
    logger.info("agent-service listening on %s", addr)

    await server.start()

    stop = asyncio.Event()

    def _signal_handler() -> None:
        logger.info("shutting down...")
        stop.set()

    loop = asyncio.get_running_loop()
    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, _signal_handler)

    await stop.wait()
    await server.stop(grace=5)
    logger.info("agent-service stopped")


if __name__ == "__main__":
    asyncio.run(serve())
