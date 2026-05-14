"""Driver integration tests for the grader gRPC service.

Each test boots a real grpc.server bound to localhost:0 (an ephemeral port)
inside the test process and exercises one RPC end-to-end against the live
GraderService implementation. No external network, no LLM calls — LLM
clients are stubbed via the test config and the LLM-judge stage is disabled
in the test settings.

The `live_grader_server` fixture (conftest.py) handles port allocation,
server start/stop, and channel construction; tests receive a ready-to-use
GraderServiceStub.
"""
from __future__ import annotations

from grader.proto import grader_pb2


def test_health_check_reports_healthy(live_grader_server) -> None:
    stub = live_grader_server.stub
    response = stub.HealthCheck(grader_pb2.Empty())
    assert response.healthy is True
    assert response.version != ""


def test_classify_failure_returns_unconfigured_sentinel_when_disabled(live_grader_server) -> None:
    # With no Anthropic key in the test config, ClassifyFailure must
    # gracefully return the NONE/0.0 sentinel rather than raise.
    stub = live_grader_server.stub
    response = stub.ClassifyFailure(
        grader_pb2.ClassifyFailureRequest(
            run_id="r1",
            symptoms_json=b"{}",
            task_description="t",
            transcript_tail="",
            classifier_model="claude-haiku-4-5-20251001",
        )
    )
    assert response.classification is not None
    # Empty/unconfigured path returns NONE with 0 confidence — same contract
    # the Go GraderClient relies on to detect "classifier was unavailable".
    assert response.classification.primary in ("NONE", "")
    assert response.classification.confidence == 0.0
