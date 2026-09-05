"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { use, useState } from "react";
import { AppNav } from "@/components/AppNav";
import { api, ApiError } from "@/lib/api";
import type { ResumeSummary, TailoringRun, TailoringSuggestion } from "@/types/api";

const MODES = ["STRICT", "GROWTH", "MAX_MATCH"] as const;

export default function TailorResumePage({ params }: { params: Promise<{ id: string }> }) {
  const { id: jobId } = use(params);
  const [resumeId, setResumeId] = useState<string>("");
  const [mode, setMode] = useState<(typeof MODES)[number]>("GROWTH");
  const [run, setRun] = useState<TailoringRun | null>(null);

  const resumesQuery = useQuery({
    queryKey: ["resumes"],
    queryFn: () => api.get<ResumeSummary[]>("/resumes"),
  });

  const tailorMutation = useMutation({
    mutationFn: () => api.post<TailoringRun>(`/jobs/${jobId}/tailor`, { resume_id: resumeId, mode }),
    onSuccess: (data) => setRun(data),
  });

  const updateSuggestion = useMutation({
    mutationFn: ({ suggestionId, status }: { suggestionId: string; status: string }) =>
      api.patch<TailoringSuggestion>(`/tailoring/${run!.id}/suggestions/${suggestionId}`, { status }),
    onSuccess: (updated) => {
      setRun((prev) =>
        prev
          ? { ...prev, suggestions: prev.suggestions.map((s) => (s.ID === updated.ID ? updated : s)) }
          : prev,
      );
    },
  });

  const approveAll = useMutation({
    mutationFn: () => api.post<TailoringSuggestion[]>(`/tailoring/${run!.id}/approve-all`),
    onSuccess: (updated) => setRun((prev) => (prev ? { ...prev, suggestions: updated } : prev)),
  });

  const parsedResumes = resumesQuery.data?.filter((r) => r.status === "PARSED") ?? [];

  return (
    <>
      <AppNav />
      <main className="mx-auto flex w-full max-w-3xl flex-1 flex-col gap-6 p-8">
        <h1 className="text-2xl font-semibold">Tailor Resume</h1>

        {!run && (
          <div className="flex flex-col gap-4 rounded-md border border-black/10 p-6 dark:border-white/15">
            <label className="flex flex-col gap-1 text-sm">
              <span className="font-medium">Resume</span>
              <select
                value={resumeId}
                onChange={(e) => setResumeId(e.target.value)}
                className="rounded-md border border-black/10 bg-transparent px-3 py-2 dark:border-white/15"
              >
                <option value="">Select a parsed resume…</option>
                {parsedResumes.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.original_filename}
                  </option>
                ))}
              </select>
            </label>

            <label className="flex flex-col gap-1 text-sm">
              <span className="font-medium">Tailoring mode</span>
              <select
                value={mode}
                onChange={(e) => setMode(e.target.value as (typeof MODES)[number])}
                className="rounded-md border border-black/10 bg-transparent px-3 py-2 dark:border-white/15"
              >
                {MODES.map((m) => (
                  <option key={m} value={m}>
                    {m}
                  </option>
                ))}
              </select>
            </label>

            {tailorMutation.isError && (
              <p className="text-sm text-red-600">
                {tailorMutation.error instanceof ApiError ? tailorMutation.error.message : "Tailoring failed."}
              </p>
            )}

            <button
              type="button"
              disabled={!resumeId || tailorMutation.isPending}
              onClick={() => tailorMutation.mutate()}
              className="rounded-md bg-foreground px-4 py-2 text-sm font-medium text-background disabled:opacity-60"
            >
              {tailorMutation.isPending ? "Tailoring…" : "Tailor Resume"}
            </button>
          </div>
        )}

        {run && (
          <div className="flex flex-col gap-6">
            <div className="flex items-center justify-between rounded-md border border-black/10 p-4 dark:border-white/15">
              <div>
                <p className="text-sm text-black/60 dark:text-white/60">Resume Alignment</p>
                <p className="text-2xl font-semibold">
                  {run.alignment_score_before}% → {run.alignment_score_after}%
                </p>
              </div>
              <button
                type="button"
                onClick={() => approveAll.mutate()}
                className="rounded-md border border-black/10 px-4 py-2 text-sm dark:border-white/15"
              >
                Approve All Selected
              </button>
            </div>

            <div className="flex flex-col gap-4">
              {run.suggestions.map((s) => (
                <SuggestionCard
                  key={s.ID}
                  suggestion={s}
                  onApprove={() => updateSuggestion.mutate({ suggestionId: s.ID, status: "APPROVED" })}
                  onReject={() => updateSuggestion.mutate({ suggestionId: s.ID, status: "REJECTED" })}
                />
              ))}
            </div>
          </div>
        )}
      </main>
    </>
  );
}

function SuggestionCard({
  suggestion,
  onApprove,
  onReject,
}: {
  suggestion: TailoringSuggestion;
  onApprove: () => void;
  onReject: () => void;
}) {
  return (
    <div className="flex flex-col gap-3 rounded-md border border-black/10 p-4 dark:border-white/15">
      <div className="flex items-center justify-between">
        <span className="text-xs font-semibold uppercase tracking-wide text-black/60 dark:text-white/60">
          {suggestion.Section}
        </span>
        <div className="flex items-center gap-2">
          {suggestion.Source === "AI_SUGGESTED" && (
            <span className="rounded-full bg-purple-100 px-2 py-0.5 text-xs font-medium text-purple-800">
              AI Suggested
            </span>
          )}
          <span
            className={`rounded-full px-2 py-0.5 text-xs font-medium ${
              suggestion.UserStatus === "APPROVED"
                ? "bg-green-100 text-green-800"
                : suggestion.UserStatus === "REJECTED"
                  ? "bg-red-100 text-red-800"
                  : "bg-black/5 dark:bg-white/10"
            }`}
          >
            {suggestion.UserStatus}
          </span>
        </div>
      </div>

      {suggestion.OriginalText && (
        <div>
          <p className="text-xs font-medium text-black/50 dark:text-white/50">ORIGINAL</p>
          <p className="text-sm">{suggestion.OriginalText}</p>
        </div>
      )}
      <div>
        <p className="text-xs font-medium text-black/50 dark:text-white/50">SUGGESTED</p>
        <p className="text-sm">{suggestion.SuggestedText}</p>
      </div>
      <p className="text-xs text-black/60 dark:text-white/60">Why: {suggestion.Reason}</p>

      {suggestion.UserStatus === "PENDING" && (
        <div className="flex gap-2">
          <button onClick={onApprove} className="rounded-md bg-foreground px-3 py-1.5 text-sm text-background">
            Approve
          </button>
          <button onClick={onReject} className="rounded-md border border-black/10 px-3 py-1.5 text-sm dark:border-white/15">
            Reject
          </button>
        </div>
      )}
    </div>
  );
}
