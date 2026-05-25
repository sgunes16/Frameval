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

The `stub_llm` fixture intercepts outbound HTTP calls to the OpenRouter
chat/completions endpoint via `respx` so GradeRun tests exercise the full
judge code path without a real LLM key. This is Option B from the task spec.
"""
from __future__ import annotations

from concurrent import futures
from dataclasses import dataclass

import grpc
import httpx
import pytest
import respx

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
    # 5s upper bound — a hung RPC thread should not wedge the pytest session
    # forever. grpc.Server.stop returns a threading.Event; without a timeout
    # .wait() blocks indefinitely.
    stopped = server.stop(grace=None)
    stopped.wait(timeout=5)


@pytest.fixture
def stub_llm(monkeypatch):
    """Intercept OpenRouter HTTP calls and return a canned judge response.

    Sets FRAMEVAL_LLM_PROVIDER=openrouter and OPENROUTER_API_KEY to a dummy
    value so llm_client.build_client() creates an OpenAI-compat client that
    points at OpenRouter. respx then intercepts every POST to
    /chat/completions and returns a deterministic JSON response instructor
    can parse into JudgeResult.
    """
    monkeypatch.setenv("FRAMEVAL_LLM_PROVIDER", "openrouter")
    monkeypatch.setenv("OPENROUTER_API_KEY", "test-key")
    monkeypatch.delenv("FRAMEVAL_LLM_BASE_URL", raising=False)
    canned_content = (
        '{"correctness":7.0,"maintainability":7.0,"completeness":7.0,'
        '"best_practices":7.0,"error_handling":7.0,"rationale":"stub"}'
    )
    canned_response = {
        "id": "stub",
        "object": "chat.completion",
        "created": 0,
        "model": "stub",
        "choices": [
            {
                "index": 0,
                "finish_reason": "stop",
                "message": {"role": "assistant", "content": canned_content},
            }
        ],
        "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
    }
    with respx.mock(base_url="https://openrouter.ai/api/v1") as mock:
        mock.post("/chat/completions").mock(
            return_value=httpx.Response(200, json=canned_response)
        )
        yield mock
