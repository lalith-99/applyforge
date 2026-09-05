"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { use } from "react";
import { AppNav } from "@/components/AppNav";
import { api, ApiError } from "@/lib/api";
import { QuickPrepDrawer } from "@/features/learning/QuickPrepDrawer";
import type { Application, JobDetail, MatchResult, QualifiedResult } from "@/types/api";

export default function JobDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);

  const jobQuery = useQuery({
    queryKey: ["job", id],
    queryFn: () => api.get<JobDetail>(`/jobs/${id}`),
  });
  const matchQuery = useQuery({
    queryKey: ["job-match", id],
    queryFn: () => api.get<MatchResult>(`/jobs/${id}/match`),
  });
  const qualifyMutation = useMutation({
    mutationFn: () => api.post<QualifiedResult>(`/jobs/${id}/make-me-qualified`),
  });
  const saveMutation = useMutation({
    mutationFn: () =>
      api.post<Application>("/applications", { job_id: id, match_score: match?.TotalScore ?? null }),
  });

  if (jobQuery.isLoading) {
    return <main className="flex flex-1 items-center justify-center p-8">Loading…</main>;
  }
  if (!jobQuery.data) {
    return <main className="flex flex-1 items-center justify-center p-8">Job not found.</main>;
  }

  const job = jobQuery.data;
  const match = matchQuery.data;

  return (
    <>
      <AppNav />
      <main className="mx-auto flex w-full max-w-3xl flex-1 flex-col gap-6 p-8">
        <div>
          <h1 className="text-2xl font-semibold">{job.title}</h1>
          <p className="text-black/60 dark:text-white/60">
            {job.company_name} {job.location_text ? `\u00b7 ${job.location_text}` : ""}
          </p>
        </div>

        <div className="flex gap-3">
          <Link
            href={`/jobs/${job.id}/tailor`}
            className="rounded-md bg-foreground px-4 py-2 text-sm font-medium text-background"
          >
            Tailor Resume
          </Link>
          <button
            type="button"
            onClick={() => qualifyMutation.mutate()}
            disabled={qualifyMutation.isPending}
            className="rounded-md border border-black/10 px-4 py-2 text-sm font-medium disabled:opacity-60 dark:border-white/15"
          >
            {qualifyMutation.isPending ? "Analyzing…" : "Make Me Qualified"}
          </button>
          <button
            type="button"
            onClick={() => saveMutation.mutate()}
            disabled={saveMutation.isPending || saveMutation.isSuccess}
            className="rounded-md border border-black/10 px-4 py-2 text-sm font-medium disabled:opacity-60 dark:border-white/15"
          >
            {saveMutation.isSuccess ? "Saved ✓" : saveMutation.isPending ? "Saving…" : "Save Job"}
          </button>
          {job.apply_url && (
            <a
              href={job.apply_url}
              target="_blank"
              rel="noopener noreferrer"
              className="rounded-md border border-black/10 px-4 py-2 text-sm font-medium dark:border-white/15"
            >
              Apply
            </a>
          )}
        </div>

        {match && (
          <section className="rounded-md border border-black/10 p-6 dark:border-white/15">
            <h2 className="text-lg font-medium">Job Match</h2>
            <p className="mt-1 text-3xl font-semibold">
              {match.TotalScore}% <span className="text-lg font-normal text-black/60 dark:text-white/60">{match.Grade}</span>
            </p>
            <p className="mt-2 text-sm text-black/70 dark:text-white/70">{match.Explanation}</p>

            {!match.Eligibility.Eligible && (
              <div className="mt-3 rounded-md bg-red-50 p-3 text-sm text-red-800">
                Not eligible: {match.Eligibility.HardFailures?.join(", ")}
              </div>
            )}

            <div className="mt-4 grid grid-cols-2 gap-4 text-sm">
              <div>
                <p className="font-medium">Current Profile Match</p>
                <p className="text-2xl">{match.CurrentProfileMatch}%</p>
              </div>
              <div>
                <p className="font-medium">Target Profile Match</p>
                <p className="text-2xl">{match.TargetProfileMatch}%</p>
                {match.SuggestedTargetAdditions && match.SuggestedTargetAdditions.length > 0 && (
                  <p className="text-xs text-black/60 dark:text-white/60">
                    Suggested additions: {match.SuggestedTargetAdditions.join(", ")}
                  </p>
                )}
              </div>
            </div>

            <div className="mt-4 flex flex-col gap-3">
              {match.MatchedSkills.length > 0 && (
                <SkillGroup title="Matched Skills" skills={match.MatchedSkills} color="bg-green-100 text-green-800" />
              )}
              {match.TransferableSkills && match.TransferableSkills.length > 0 && (
                <div>
                  <p className="mb-1 text-sm font-medium">Transferable Skills</p>
                  <div className="flex flex-wrap gap-2">
                    {match.TransferableSkills.map((t) => (
                      <span key={t.TargetSkill} className="rounded-full bg-blue-100 px-2 py-1 text-xs text-blue-800">
                        {t.SourceSkill} → {t.TargetSkill} ({t.Level})
                      </span>
                    ))}
                  </div>
                </div>
              )}
              {match.MissingRequiredSkills && match.MissingRequiredSkills.length > 0 && (
                <div>
                  <p className="mb-1 text-sm font-medium">Missing Skills</p>
                  <div className="flex flex-wrap gap-2">
                    {match.MissingRequiredSkills.map((skill) => (
                      <QuickPrepDrawer key={skill} skill={skill} />
                    ))}
                  </div>
                </div>
              )}
            </div>

            {match.Concerns && match.Concerns.length > 0 && (
              <div className="mt-4">
                <p className="mb-1 text-sm font-medium">Potential Concerns</p>
                <ul className="list-inside list-disc text-sm text-black/70 dark:text-white/70">
                  {match.Concerns.map((c, i) => (
                    <li key={i}>{c}</li>
                  ))}
                </ul>
              </div>
            )}
          </section>
        )}

        {qualifyMutation.isError && (
          <p className="text-sm text-red-600">
            {qualifyMutation.error instanceof ApiError ? qualifyMutation.error.message : "Could not analyze qualification gap."}
          </p>
        )}

        {qualifyMutation.data && (
          <section className="rounded-md border border-black/10 p-6 dark:border-white/15">
            <h2 className="text-lg font-medium">Make Me Qualified</h2>
            <div className="mt-3 grid grid-cols-2 gap-4 text-sm">
              <div>
                <p className="font-medium">Interview Readiness</p>
                <p className="text-2xl">{qualifyMutation.data.InterviewReadiness}%</p>
              </div>
              <div>
                <p className="font-medium">Prep Effort</p>
                <p className="text-2xl">{qualifyMutation.data.LearningPlan.EstimatedEffortCategory.replace("_", " ")}</p>
              </div>
            </div>

            {qualifyMutation.data.HighValueGaps && qualifyMutation.data.HighValueGaps.length > 0 && (
              <div className="mt-4">
                <p className="mb-1 text-sm font-medium">High-Value Gaps</p>
                <div className="flex flex-wrap gap-2">
                  {qualifyMutation.data.HighValueGaps.map((skill) => (
                    <QuickPrepDrawer key={skill} skill={skill} />
                  ))}
                </div>
              </div>
            )}

            {qualifyMutation.data.LowValueGaps && qualifyMutation.data.LowValueGaps.length > 0 && (
              <div className="mt-4">
                <p className="mb-1 text-sm font-medium">Lower-Priority Gaps</p>
                <div className="flex flex-wrap gap-2">
                  {qualifyMutation.data.LowValueGaps.map((skill) => (
                    <QuickPrepDrawer key={skill} skill={skill} />
                  ))}
                </div>
              </div>
            )}

            {qualifyMutation.data.LearningPlan.Projects.length > 0 && (
              <div className="mt-4">
                <p className="mb-1 text-sm font-medium">Practice Projects</p>
                <ul className="list-inside list-disc text-sm">
                  {qualifyMutation.data.LearningPlan.Projects.map((p) => (
                    <li key={p}>{p}</li>
                  ))}
                </ul>
              </div>
            )}
          </section>
        )}

        {job.requirements && job.requirements.Responsibilities.length > 0 && (
          <section>
            <h2 className="mb-2 text-lg font-medium">Responsibilities</h2>
            <ul className="list-inside list-disc text-sm">
              {job.requirements.Responsibilities.map((r, i) => (
                <li key={i}>{r}</li>
              ))}
            </ul>
          </section>
        )}

        <section>
          <h2 className="mb-2 text-lg font-medium">Full Job Description</h2>
          <p className="whitespace-pre-wrap text-sm text-black/70 dark:text-white/70">{job.description}</p>
        </section>
      </main>
    </>
  );
}

function SkillGroup({ title, skills, color }: { title: string; skills: string[]; color: string }) {
  return (
    <div>
      <p className="mb-1 text-sm font-medium">{title}</p>
      <div className="flex flex-wrap gap-2">
        {skills.map((s) => (
          <span key={s} className={`rounded-full px-2 py-1 text-xs ${color}`}>
            {s}
          </span>
        ))}
      </div>
    </div>
  );
}
