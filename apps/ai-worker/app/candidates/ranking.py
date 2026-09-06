"""AI job-fit ranking (Phase H): the deterministic scorer already produced a
TotalScore per job (matching package, Go side); this asks an LLM to judge
genuine fit for a batch of the top candidates, since a deterministic formula
can't reason about e.g. "is this skill gap actually easy to close" the way a
model prompted with the right context can. The heuristic fallback is a
straightforward threshold mapping off deterministic_score - a reasonable
default, not a real substitute for the model's judgment.
"""

from __future__ import annotations

from app.candidates.ranking_models import (
    JobRankingInput,
    JobRankingResult,
    RankJobsRequest,
    RankJobsResult,
)
from app.providers.openai_provider import structured_completion

_SYSTEM_PROMPT = """You are a technical recruiter judging job fit for a candidate. For EACH \
job in the batch, decide: fit_score (0-100, overall fit for this specific role), \
interview_probability_score (0-100, realistic chance of passing screening given the skill \
gaps), career_alignment (0-100, how well this role advances the candidate's stated target \
roles/direction), skill_gap_severity (LOW/MEDIUM/HIGH), strong_evidence (2-4 concrete \
matched-skill/experience points supporting this candidate), gaps (missing_required_skills \
that are genuinely concerning - ignore ones covered by a transferable_note), and \
recommendation: APPLY_NOW (strong fit, apply immediately), STRONG_CONSIDER (good fit, minor \
gaps), CONSIDER (plausible but real gaps), or SKIP (weak fit / major gaps). A lower \
deterministic_score with easily-transferable gaps can still deserve APPLY_NOW; a high \
deterministic_score with a fundamental mismatch (e.g. wrong seniority/domain) can still \
deserve SKIP - use judgment, don't just echo deterministic_score back. Return one ranking \
per job_id given, in any order."""


def rank_jobs_heuristic(request: RankJobsRequest) -> RankJobsResult:
    rankings = []
    for job in request.jobs:
        score = max(0, min(100, job.deterministic_score))
        if score >= 85:
            rec = "APPLY_NOW"
        elif score >= 70:
            rec = "STRONG_CONSIDER"
        elif score >= 50:
            rec = "CONSIDER"
        else:
            rec = "SKIP"
        severity = "LOW" if len(job.missing_required_skills) == 0 else (
            "HIGH" if len(job.missing_required_skills) > 2 else "MEDIUM"
        )
        rankings.append(
            JobRankingResult(
                job_id=job.job_id,
                fit_score=score,
                interview_probability_score=score,
                career_alignment=score,
                skill_gap_severity=severity,
                strong_evidence=job.matched_skills[:4],
                gaps=job.missing_required_skills,
                recommendation=rec,
                reason="Heuristic fallback: based on deterministic score only.",
            )
        )
    return RankJobsResult(rankings=rankings)


def rank_jobs_ai(request: RankJobsRequest) -> RankJobsResult:
    jobs_text = "\n\n".join(_format_job(job) for job in request.jobs)
    user_prompt = f"""Candidate summary: {request.candidate_summary or 'none provided'}
Target roles: {', '.join(request.target_roles) or 'unspecified'}

Jobs to rank ({len(request.jobs)}):
{jobs_text}"""

    return structured_completion(_SYSTEM_PROMPT, user_prompt, RankJobsResult)


def _format_job(job: JobRankingInput) -> str:
    return f"""job_id: {job.job_id}
Title: {job.title} at {job.company_name}
Seniority: {job.seniority or 'unspecified'} | Remote: {job.remote_type or 'unspecified'}
Deterministic score: {job.deterministic_score}
Matched skills: {', '.join(job.matched_skills) or 'none'}
Missing required skills: {', '.join(job.missing_required_skills) or 'none'}
Missing preferred skills: {', '.join(job.missing_preferred_skills) or 'none'}
Transferable notes: {', '.join(job.transferable_notes) or 'none'}"""
