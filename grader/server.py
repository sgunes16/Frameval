from __future__ import annotations

from concurrent import futures

import grpc

import json
import time

from grader.code_grader import grade as grade_code
from grader.composite import compute_composite
from grader.config import get_settings
from grader.failure_classifier import classify as classify_failure
from grader.failure_classifier.grader import FailureClassifier
from grader.llm_judge import grade as judge_grade
from grader.process_grader import grade as process_grade
from grader.stats import compute_stats
from grader.proto import grader_pb2, grader_pb2_grpc


class GraderService(grader_pb2_grpc.GraderServiceServicer):
    def HealthCheck(self, request: grader_pb2.Empty, context: grpc.ServicerContext) -> grader_pb2.HealthResponse:
        settings = get_settings()
        return grader_pb2.HealthResponse(healthy=True, version=settings.version)

    def GradeRun(self, request: grader_pb2.GradeRunRequest, context: grpc.ServicerContext) -> grader_pb2.GradeRunResponse:
        settings = get_settings()
        output_files = [{"path": f.path, "content": bytes(f.content)} for f in request.output_files]
        task = {
            "id": request.task.id,
            "prompt": request.task.prompt,
            "codebase_type": request.task.codebase_type,
            "setup_script": request.task.setup_script,
            "test_cases": [{"name": tc.name, "command": tc.command, "expected_result": tc.expected_result} for tc in request.task.test_cases],
        }
        code = grade_code(task, output_files)
        print(f"[judge_debug] code_grade done; test_pass_rate={code.get('test_pass_rate')}", flush=True)
        process = process_grade(request.transcript_json)
        print(f"[judge_debug] process_grade done", flush=True)
        judge_cfg = request.judge_config if request.HasField("judge_config") else None
        print(f"[judge_debug] judge_cfg={'<set>' if judge_cfg else None} env_enable={settings.enable_llm_judge} provider={getattr(judge_cfg, 'provider', None)} model={getattr(judge_cfg, 'model', None)} has_key={bool(getattr(judge_cfg, 'api_key', None))}", flush=True)
        if settings.enable_llm_judge or judge_cfg is not None:
            print(f"[judge_debug] calling judge_grade...", flush=True)
            judge = judge_grade(
                code,
                process,
                task=task,
                output_files=output_files,
                transcript_json=request.transcript_json.encode(),
                config_override=judge_cfg,
            )
            print(f"[judge_debug] judge_grade returned; raw_responses_head={(judge.get('raw_responses') or ['<empty>'])[0][:120]}", flush=True)
        else:
            judge = disabled_judge_result()
            print(f"[judge_debug] judge disabled (env=false, no proto config)", flush=True)
        adherence = disabled_adherence_result()
        judge_active = settings.enable_llm_judge or judge_cfg is not None
        composite = compute_composite(
            code,
            process,
            judge if judge_active else None,
            None,
        )
        return grader_pb2.GradeRunResponse(
            code=grader_pb2.CodeGradeResult(
                test_pass_rate=code["test_pass_rate"],
                test_pass_count=code["test_pass_count"],
                test_fail_count=code["test_fail_count"],
                lint_score=code["lint_score"],
                type_check_pass=code["type_check_pass"],
                file_state_valid=code["file_state_valid"],
                test_results=[grader_pb2.TestResult(name=item["name"], passed=item["passed"], output=item["output"]) for item in code["test_results"]],
            ),
            process=grader_pb2.ProcessGradeResult(**process),
            judge=grader_pb2.JudgeGradeResult(**judge),
            adherence=grader_pb2.SpecAdherenceResult(
                instruction_compliance=adherence["instruction_compliance"],
                constraint_violations=adherence["constraint_violations"],
                convention_adherence=adherence["convention_adherence"],
                per_instruction=[grader_pb2.InstructionResult(**item) for item in adherence["per_instruction"]],
            ),
            composite_score=composite,
        )

    def ComputeStats(self, request: grader_pb2.ComputeStatsRequest, context: grpc.ServicerContext) -> grader_pb2.ComputeStatsResponse:
        payload = []
        for variant in request.variant_grades:
            grades = []
            for grade in variant.grades:
                grades.append({
                    "composite_score": grade.composite_score,
                    "test_pass_rate": grade.code.test_pass_rate if grade.HasField("code") else 0.0,
                    "token_efficiency": grade.process.token_efficiency if grade.HasField("process") else 0.0,
                    "context_utilization": grade.process.context_utilization if grade.HasField("process") else 0.0,
                })
            payload.append({"variant_id": variant.variant_id, "grades": grades})
        stats = compute_stats(request.experiment_id, payload)
        return grader_pb2.ComputeStatsResponse(stats=[grader_pb2.PairwiseStat(**stat) for stat in stats])

    def ClassifyFailure(
        self,
        request: grader_pb2.ClassifyFailureRequest,
        context: grpc.ServicerContext,
    ) -> grader_pb2.ClassifyFailureResponse:
        """Run the LLM failure classifier on the supplied run data.

        Never raises — on any classifier-side exception, the underlying
        FailureClassifier returns the unclassified() sentinel. We translate
        that into a ClassifyFailureResponse with primary="NONE" and
        confidence=0 so the Go orchestrator can persist a row and move on.
        """
        started = time.perf_counter()
        try:
            symptoms = json.loads(request.symptoms_json or b"{}")
        except json.JSONDecodeError:
            symptoms = {}
        # When the caller supplies a non-empty classifier_model override,
        # run a one-shot FailureClassifier with that model. Empty falls
        # through to the module-level default (Haiku 4.5 per spec §4.7.4).
        # This is the override hook calibration ablation runs use (Story #25).
        override_model = (request.classifier_model or "").strip()
        if override_model:
            verdict = FailureClassifier(model=override_model).classify(
                symptoms=symptoms,
                task_description=request.task_description,
                transcript_tail=request.transcript_tail,
            )
        else:
            verdict = classify_failure(
                symptoms=symptoms,
                task_description=request.task_description,
                transcript_tail=request.transcript_tail,
            )
        latency_ms = int((time.perf_counter() - started) * 1000)
        evidence_pbs = [
            grader_pb2.EvidenceSpanProto(
                code=e.code.value,
                quote=e.quote,
                turn_index=e.turn_index,
            )
            for e in verdict.evidence
        ]
        classification = grader_pb2.FailureClassificationProto(
            primary=verdict.primary.value,
            secondary=[c.value for c in verdict.secondary],
            evidence=evidence_pbs,
            confidence=verdict.confidence,
            rationale=verdict.rationale,
        )
        return grader_pb2.ClassifyFailureResponse(
            classification=classification,
            latency_ms=latency_ms,
        )

    def ClassifyDimensions(self, request: grader_pb2.ClassifyDimensionsRequest, context: grpc.ServicerContext) -> grader_pb2.ClassifyDimensionsResponse:
        return grader_pb2.ClassifyDimensionsResponse(
            framing="instructional",
            specificity="medium",
            structure="hierarchical",
            scope="task-focused",
            tone="neutral",
            constraint_density=0.5,
            example_presence="low",
            priority_signaling="medium",
            tool_guidance=request.artifact_type,
            error_handling="explicit" if "error" in request.content.lower() else "implicit",
        )


def serve() -> None:
    settings = get_settings()
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=8))
    grader_pb2_grpc.add_GraderServiceServicer_to_server(GraderService(), server)
    server.add_insecure_port(f"[::]:{settings.port}")
    server.start()
    server.wait_for_termination()


def disabled_judge_result() -> dict[str, float | list[str]]:
    return {
        "correctness": 0.0,
        "maintainability": 0.0,
        "completeness": 0.0,
        "best_practices": 0.0,
        "error_handling": 0.0,
        "irr_alpha": 0.0,
        "raw_responses": ["llm_judge_disabled"],
    }


def disabled_adherence_result() -> dict[str, float | int | list[dict[str, str]]]:
    return {
        "instruction_compliance": 0.0,
        "constraint_violations": 0,
        "convention_adherence": 0.0,
        "per_instruction": [],
    }


if __name__ == "__main__":
    serve()
