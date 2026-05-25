"""Driver integration tests for the grader gRPC service.

Each test boots a real grpc.server bound to localhost:0 (an ephemeral port)
inside the test process and exercises one RPC end-to-end against the live
GraderService implementation. No external network calls except where noted —
LLM calls are intercepted by the `stub_llm` respx fixture (Option B).

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
    # Unconfigured path returns the FailureCode.NONE sentinel with 0
    # confidence — same contract the Go GraderClient relies on to detect
    # "classifier was unavailable". The proto must populate `primary`;
    # accepting an empty string here would mask a server-side regression
    # where the field is never set.
    assert response.classification.primary == "NONE"
    assert response.classification.confidence == 0.0


def test_grade_run_returns_disabled_judge_when_env_off(live_grader_server, monkeypatch) -> None:
    """GradeRun returns a zeroed judge block when FRAMEVAL_ENABLE_LLM_JUDGE=false.

    The default is now true, so we explicitly opt out in this test to keep
    the integration suite free of real LLM calls. The response must still be
    a valid GradeRunResponse — code/process fields populated, judge zeroed,
    composite non-negative.

    Note: respx-based Option B stub is provided in conftest.stub_llm for
    future tests that need end-to-end judge coverage. It is not used here
    because instructor's OpenAI client runs in a gRPC ThreadPoolExecutor and
    respx context managers only intercept the calling thread's httpx
    transport, making cross-thread interception unreliable.
    """
    monkeypatch.setenv("FRAMEVAL_ENABLE_LLM_JUDGE", "false")
    stub = live_grader_server.stub
    request = grader_pb2.GradeRunRequest(
        run_id="r-test",
        transcript_json=b"[]",
        task=grader_pb2.TaskSpec(
            id="t1",
            prompt="Write a hello-world function.",
            codebase_type="python",
            setup_script="",
        ),
    )
    response = stub.GradeRun(request)
    # Judge is disabled — scores and rationales maps must be empty
    assert len(response.judge.scores) == 0
    assert len(response.judge.rationales) == 0
    assert "llm_judge_disabled" in response.judge.raw_responses
    # Composite score is still non-negative
    assert response.composite_score >= 0.0
