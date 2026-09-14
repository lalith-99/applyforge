"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useMemo, useState } from "react";

import { AppNav } from "@/components/AppNav";
import { ApplyReviewModal } from "@/features/applications/ApplyReviewModal";
import { api } from "@/lib/api";
import type { ApplicationStatus, ApplicationWithJob } from "@/types/api";
import type { SubmissionIntent } from "@/types/apply";

const STATUS_COLUMNS: { status: ApplicationStatus; label: string }[] = [
  { status: "SAVED", label: "Saved" },
  { status: "READY_TO_APPLY", label: "Ready to Apply" },
  { status: "APPLIED", label: "Applied" },
  { status: "RECRUITER_SCREEN", label: "Recruiter Screen" },
  { status: "ASSESSMENT", label: "Assessment" },
  { status: "TECHNICAL_INTERVIEW", label: "Technical Interview" },
  { status: "FINAL_INTERVIEW", label: "Final Interview" },
  { status: "OFFER", label: "Offer" },
  { status: "REJECTED", label: "Rejected" },
  { status: "WITHDRAWN", label: "Withdrawn" },
];

export default function ApplicationsPage() {
  const [view, setView] = useState<"kanban" | "table">("kanban");
  const [recentlyMoved, setRecentlyMoved] = useState<string | null>(null);
  const [moveError, setMoveError] = useState<string | null>(null);
  const [reviewing, setReviewing] = useState<ApplicationWithJob | null>(null);
  const queryClient = useQueryClient();

  const applicationsQuery = useQuery({
    queryKey: ["applications"],
    queryFn: () => api.get<ApplicationWithJob[]>("/applications"),
  });
  const submissionStatesQuery = useQuery({
    queryKey: ["submission-intents", "latest"],
    queryFn: () => api.get<SubmissionIntent[]>("/submission-intents"),
    refetchInterval: 15_000,
  });

  const updateStatus = useMutation({
    mutationFn: ({ id, status }: { id: string; status: ApplicationStatus }) =>
      api.patch<ApplicationWithJob>(`/applications/${id}`, { status }),
    onMutate: async ({ id, status }) => {
      setMoveError(null);
      await queryClient.cancelQueries({ queryKey: ["applications"] });
      const previous = queryClient.getQueryData<ApplicationWithJob[]>(["applications"]) ?? [];
      queryClient.setQueryData<ApplicationWithJob[]>(["applications"], (current = []) =>
        current.map((app) => app.ID === id ? { ...app, Status: status, UpdatedAt: new Date().toISOString() } : app),
      );
      setRecentlyMoved(id);
      return { previous };
    },
    onError: (error, _variables, context) => {
      if (context?.previous) queryClient.setQueryData(["applications"], context.previous);
      setRecentlyMoved(null);
      setMoveError(error instanceof Error ? error.message : "Could not update application status.");
    },
    onSettled: () => void queryClient.invalidateQueries({ queryKey: ["applications"] }),
  });

  const applications = applicationsQuery.data ?? [];
  const submissionByApplication = useMemo(() => {
    const map = new Map<string, SubmissionIntent>();
    for (const intent of submissionStatesQuery.data ?? []) map.set(intent.application_id, intent);
    return map;
  }, [submissionStatesQuery.data]);
  const uncertain = (submissionStatesQuery.data ?? []).filter((intent) => intent.status === "UNCERTAIN");

  useEffect(() => {
    if (!recentlyMoved || view !== "kanban") return;
    const timer = window.setTimeout(() => {
      document.querySelector(`[data-application-id="${recentlyMoved}"]`)?.scrollIntoView({ behavior: "smooth", block: "nearest", inline: "center" });
    }, 40);
    const clear = window.setTimeout(() => setRecentlyMoved(null), 1800);
    return () => {
      window.clearTimeout(timer);
      window.clearTimeout(clear);
    };
  }, [recentlyMoved, view]);

  return (
    <>
      <AppNav />
      <main className="mx-auto flex w-full max-w-6xl flex-1 flex-col gap-6 p-8">
        <div className="flex items-center justify-between">
          <div>
            <h1 className="text-2xl font-semibold">Applications</h1>
            <p className="mt-1 text-sm text-black/55 dark:text-white/55">Review the exact resume and answers before authorizing any application submission.</p>
          </div>
          <div className="flex gap-2 text-sm">
            <button type="button" onClick={() => setView("kanban")} className={`rounded-md border border-black/10 px-3 py-1.5 dark:border-white/15 ${view === "kanban" ? "bg-foreground text-background" : ""}`}>Kanban</button>
            <button type="button" onClick={() => setView("table")} className={`rounded-md border border-black/10 px-3 py-1.5 dark:border-white/15 ${view === "table" ? "bg-foreground text-background" : ""}`}>Table</button>
          </div>
        </div>

        {uncertain.length > 0 && (
          <div className="rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-950 dark:bg-amber-950/20 dark:text-amber-100">
            <p className="font-semibold">{uncertain.length} submission{uncertain.length === 1 ? "" : "s"} need reconciliation.</p>
            <p className="mt-1">ApplyForge crossed the final submit boundary but could not prove whether the ATS accepted the application. These are never retried automatically. Check the employer portal or confirmation email before changing status.</p>
          </div>
        )}

        {applicationsQuery.isLoading && <p className="text-sm text-black/60 dark:text-white/60">Loading…</p>}
        {moveError && <p role="alert" className="rounded-md bg-red-50 p-3 text-sm text-red-700">{moveError}</p>}

        {view === "kanban" ? (
          <div className="flex gap-4 overflow-x-auto pb-4">
            {STATUS_COLUMNS.map((col) => {
              const items = applications.filter((a) => a.Status === col.status);
              return (
                <div key={col.status} className="flex w-64 shrink-0 flex-col gap-3">
                  <p className="text-xs font-semibold uppercase tracking-wide text-black/50 dark:text-white/50">{col.label} ({items.length})</p>
                  <div className="flex flex-col gap-2">
                    {items.map((app) => (
                      <ApplicationCard
                        key={app.ID}
                        app={app}
                        submission={submissionByApplication.get(app.ID)}
                        highlighted={recentlyMoved === app.ID}
                        updating={updateStatus.isPending && updateStatus.variables?.id === app.ID}
                        onAdvance={(status) => updateStatus.mutate({ id: app.ID, status })}
                        onReview={() => setReviewing(app)}
                      />
                    ))}
                  </div>
                </div>
              );
            })}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead>
                <tr className="border-b border-black/10 text-xs uppercase tracking-wide text-black/50 dark:border-white/15 dark:text-white/50">
                  <th className="py-2">Company</th><th className="py-2">Title</th><th className="py-2">Status</th><th className="py-2">Submission</th><th className="py-2">Match</th><th className="py-2">Updated</th><th className="py-2">Action</th>
                </tr>
              </thead>
              <tbody>
                {applications.map((app) => {
                  const submission = submissionByApplication.get(app.ID);
                  return (
                    <tr key={app.ID} className="border-b border-black/5 dark:border-white/10">
                      <td className="py-2">{app.CompanyName}</td>
                      <td className="py-2"><Link href={`/jobs/${app.JobID}`} className="hover:underline">{app.Title}</Link></td>
                      <td className="py-2">
                        <select value={app.Status} onChange={(e) => updateStatus.mutate({ id: app.ID, status: e.target.value as ApplicationStatus })} className="rounded-md border border-black/10 bg-transparent px-2 py-1 text-xs dark:border-white/15">
                          {STATUS_COLUMNS.map((col) => <option key={col.status} value={col.status}>{col.label}</option>)}
                        </select>
                      </td>
                      <td className="py-2"><SubmissionBadge submission={submission} /></td>
                      <td className="py-2">{app.MatchScore != null ? `${app.MatchScore}%` : "—"}</td>
                      <td className="py-2 text-black/60 dark:text-white/60">{new Date(app.UpdatedAt).toLocaleDateString()}</td>
                      <td className="py-2">{app.Status === "READY_TO_APPLY" && submission?.status !== "UNCERTAIN" ? <button type="button" onClick={() => setReviewing(app)} className="rounded-md bg-foreground px-3 py-1.5 text-xs text-background">Review & apply</button> : "—"}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}

        {!applicationsQuery.isLoading && applications.length === 0 && <p className="text-sm text-black/60 dark:text-white/60">No applications yet. Save a job from its detail page to start tracking it here.</p>}
      </main>

      {reviewing && <ApplyReviewModal app={reviewing} onClose={() => setReviewing(null)} />}
    </>
  );
}

function ApplicationCard({ app, submission, onAdvance, onReview, highlighted, updating }: {
  app: ApplicationWithJob;
  submission?: SubmissionIntent;
  onAdvance: (status: ApplicationStatus) => void;
  onReview: () => void;
  highlighted: boolean;
  updating: boolean;
}) {
  const currentIndex = STATUS_COLUMNS.findIndex((c) => c.status === app.Status);
  const next = STATUS_COLUMNS[currentIndex + 1];

  return (
    <div data-application-id={app.ID} className={`flex flex-col gap-2 rounded-md border p-3 text-sm transition-all dark:border-white/15 ${highlighted ? "border-blue-400 bg-blue-50 ring-2 ring-blue-200 dark:bg-blue-950/20" : submission?.status === "UNCERTAIN" ? "border-amber-400 bg-amber-50 dark:bg-amber-950/20" : "border-black/10"}`}>
      <Link href={`/jobs/${app.JobID}`} className="font-medium hover:underline">{app.Title}</Link>
      <p className="text-xs text-black/60 dark:text-white/60">{app.CompanyName}</p>
      {app.MatchScore != null && <p className="text-xs text-black/60 dark:text-white/60">Match: {app.MatchScore}%</p>}
      <SubmissionBadge submission={submission} />
      {submission?.status === "UNCERTAIN" && <p className="text-xs text-amber-800 dark:text-amber-200">Check the ATS or confirmation email before retrying. ApplyForge will not auto-submit this again.</p>}
      {app.Status === "READY_TO_APPLY" && submission?.status !== "UNCERTAIN" ? (
        <button type="button" onClick={onReview} className="mt-1 rounded-md bg-foreground px-2 py-1.5 text-xs font-medium text-background">Review & apply →</button>
      ) : next && next.status !== "REJECTED" && next.status !== "WITHDRAWN" && submission?.status !== "UNCERTAIN" ? (
        <button type="button" onClick={() => onAdvance(next.status)} disabled={updating} className="mt-1 rounded-md border border-black/10 px-2 py-1 text-xs disabled:opacity-60 dark:border-white/15">{updating ? "Moving…" : `Move to ${next.label} →`}</button>
      ) : null}
    </div>
  );
}

function SubmissionBadge({ submission }: { submission?: SubmissionIntent }) {
  if (!submission) return <span className="text-xs text-black/40 dark:text-white/40">No submission attempt</span>;
  const labels: Record<SubmissionIntent["status"], string> = {
    PENDING: "Approved · pending",
    CLAIMED: "Companion claimed",
    SUBMITTING: "Submitting",
    CONFIRMED: "Submission confirmed",
    UNCERTAIN: "Submission uncertain",
    FAILED: "Submission failed",
    CANCELLED: "Submission cancelled",
  };
  const className = submission.status === "UNCERTAIN"
    ? "bg-amber-100 text-amber-800 dark:bg-amber-950/40 dark:text-amber-200"
    : submission.status === "CONFIRMED"
      ? "bg-green-100 text-green-800 dark:bg-green-950/40 dark:text-green-200"
      : "bg-black/5 text-black/60 dark:bg-white/10 dark:text-white/70";
  return <span className={`w-fit rounded-full px-2 py-1 text-[11px] font-medium ${className}`}>{labels[submission.status]}</span>;
}
