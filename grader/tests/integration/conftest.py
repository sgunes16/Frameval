"""Shared fixtures for grader integration tests.

The `live_grader_server` fixture brings up a real grpc.server bound to an
ephemeral localhost port, registers the production `GraderService`, and
returns a small bundle (address + ready-to-call stub) for tests. Server
lifecycle is managed via pytest's fixture teardown — tests never have to
remember to shut it down.

Why a real server (not grpcio-testing in-process): the production code that
talks to the grader uses `grpc.NewClient` against a TCP address (see Go
`engine/internal/experiment/grader_client.go`). Running the same wire
contract in tests catches bugs that an in-process stub would mask, at the
cost of a few milliseconds per test.
"""
from __future__ import annotations

from concurrent import futures
from dataclasses import dataclass

import grpc
import pytest

from grader.proto import grader_pb2_grpc
from grader.server import GraderService


@dataclass
class GraderServerHandle:
    """A live grader server's address plus a connected client stub."""

    address: str
    stub: grader_pb2_grpc.GraderServiceStub


@pytest.fixture
def live_grader_server():
    """Boot a GraderService on an ephemeral localhost port; tear it down at test end."""
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=2))
    grader_pb2_grpc.add_GraderServiceServicer_to_server(GraderService(), server)
    port = server.add_insecure_port("127.0.0.1:0")
    server.start()

    channel = grpc.insecure_channel(f"127.0.0.1:{port}")
    stub = grader_pb2_grpc.GraderServiceStub(channel)

    yield GraderServerHandle(address=f"127.0.0.1:{port}", stub=stub)

    channel.close()
    server.stop(grace=None).wait()
