"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useEffect, useState } from "react";
import { AppNav } from "@/components/AppNav";
import { api } from "@/lib/api";
import type { ApplicationStatus, ApplicationWithJob } from "@/types/api";

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
  const queryClient = useQueryClient();

  const applicationsQuery = useQuery({
    queryKey: ["applications"],
    queryFn: () => api.get<ApplicationWithJob[]>("/applications"),
  });

  const updateStatus = useMutation({
    mutationFn: ({ id, status }: { id: string; status: ApplicationStatus }) =>
      api.patch<ApplicationWithJob>(`/applications/${id}`, { status }),
    onMutate: async ({ id, status }) => {
      setMoveError(null);
      await queryClient.cancelQueries({ queryKey: ["applications"] });
      const previous = queryClient.getQueryData<ApplicationWithJob[]>(["applications"]) ?? [];
      queryClient.setQueryData<ApplicationWithJob[]>(["applications"], (current = []) =>
        current.map((app) =>
          app.ID === id
            ? { ...app, Status: status, UpdatedAt: new Date().toISOString() }
            : app,
        ),
      );
      setRecentlyMoved(id);
      return { previous };
    },
    onError: (error, _variables, context) => {
      if (context?.previous) {
        queryClient.setQueryData(["applications"], context.previous);
      }
      setRecentlyMoved(null);
      setMoveError(error instanceof Error ? error.message : "Could not update application status.");
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: ["applications"] });
    },
  });

  const applications = applicationsQuery.data ?? [];

  useEffect(() => {
    if (!recentlyMoved || view !== "kanban") return;
    const timer = window.setTimeout(() => {
      const element = document.querySelector(`[data-application-id="${recentlyMoved}"]`);
      element?.scrollIntoView({ behavior: "smooth", block: "nearest", inline: "center" });
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
          <h1 className="text-2xl font-semibold">Applications</h1>
          <div className="flex gap-2 text-sm">
            <button
              type="button"
              onClick={() => setView("kanban")}
              className={`rounded-md border border-black/10 px-3 py-1.5 dark:border-white/15 ${view === "kanban" ? "bg-foreground text-background" : ""}`}
            >
              Kanban
            </button>
            <button
              type="button"
              onClick={() => setView("table")}
              className={`rounded-md border border-black/10 px-3 py-1.5 dark:border-white/15 ${view === "table" ? "bg-foreground text-background" : ""}`}
            >
              Table
            </button>
          </div>
        </div>

        {applicationsQuery.isLoading && <p className="text-sm text-black/60 dark:text-white/60">Loading…</p>}
        {moveError && (
          <p role="alert" className="rounded-md bg-red-50 p-3 text-sm text-red-700">
            {moveError}
          </p>
        )}

        {view === "kanban" ? (
          <div className="flex gap-4 overflow-x-auto pb-4">
            {STATUS_COLUMNS.map((col) => {
              const items = applications.filter((a) => a.Status === col.status);
              return (
                <div key={col.status} className="flex w-64 shrink-0 flex-col gap-3">
                  <p className="text-xs font-semibold uppercase tracking-wide text-black/50 dark:text-white/50">
                    {col.label} ({items.length})
                  </p>
                  <div className="flex flex-col gap-2">
                    {items.map((app) => (
                      <ApplicationCard
                        key={app.ID}
                        app={app}
                        highlighted={recentlyMoved === app.ID}
                        updating={updateStatus.isPending && updateStatus.variables?.id === app.ID}
                        onAdvance={(status) => updateStatus.mutate({ id: app.ID, status })}
                      />
                    ))}
                  </div>
                </div>
              );
            })}
          </div>
        ) : (
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-black/10 text-xs uppercase tracking-wide text-black/50 dark:border-white/15 dark:text-white/50">
                <th className="py-2">Company</th>
                <th className="py-2">Title</th>
                <th className="py-2">Status</th>
                <th className="py-2">Match</th>
                <th className="py-2">Updated</th>
              </tr>
            </thead>
            <tbody>
              {applications.map((app) => (
                <tr key={app.ID} className="border-b border-black/5 dark:border-white/10">
                  <td className="py-2">{app.CompanyName}</td>
                  <td className="py-2">
                    <Link href={`/jobs/${app.JobID}`} className="hover:underline">
                      {app.Title}
                    </Link>
                  </td>
                  <td className="py-2">
                    <select
                      value={app.Status}
                      onChange={(e) => updateStatus.mutate({ id: app.ID, status: e.target.value as ApplicationStatus })}
                      className="rounded-md border border-black/10 bg-transparent px-2 py-1 text-xs dark:border-white/15"
                    >
                      {STATUS_COLUMNS.map((col) => (
                        <option key={col.status} value={col.status}>
                          {col.label}
                        </option>
                      ))}
                    </select>
                  </td>
                  <td className="py-2">{app.MatchScore != null ? `${app.MatchScore}%` : "—"}</td>
                  <td className="py-2 text-black/60 dark:text-white/60">
                    {new Date(app.UpdatedAt).toLocaleDateString()}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}

        {!applicationsQuery.isLoading && applications.length === 0 && (
          <p className="text-sm text-black/60 dark:text-white/60">
            No applications yet. Save a job from its detail page to start tracking it here.
          </p>
        )}
      </main>
    </>
  );
}

function ApplicationCard({
  app,
  onAdvance,
  highlighted,
  updating,
}: {
  app: ApplicationWithJob;
  onAdvance: (status: ApplicationStatus) => void;
  highlighted: boolean;
  updating: boolean;
}) {
  const currentIndex = STATUS_COLUMNS.findIndex((c) => c.status === app.Status);
  const next = STATUS_COLUMNS[currentIndex + 1];

  return (
    <div
      data-application-id={app.ID}
      className={`flex flex-col gap-2 rounded-md border p-3 text-sm transition-all dark:border-white/15 ${
        highlighted
          ? "border-blue-400 bg-blue-50 ring-2 ring-blue-200 dark:bg-blue-950/20"
          : "border-black/10"
      }`}
    >
      <Link href={`/jobs/${app.JobID}`} className="font-medium hover:underline">
        {app.Title}
      </Link>
      <p className="text-xs text-black/60 dark:text-white/60">{app.CompanyName}</p>
      {app.MatchScore != null && (
        <p className="text-xs text-black/60 dark:text-white/60">Match: {app.MatchScore}%</p>
      )}
      {next && next.status !== "REJECTED" && next.status !== "WITHDRAWN" && (
        <button
          type="button"
          onClick={() => onAdvance(next.status)}
          disabled={updating}
          className="mt-1 rounded-md border border-black/10 px-2 py-1 text-xs disabled:opacity-60 dark:border-white/15"
        >
          {updating ? "Moving…" : `Move to ${next.label} →`}
        </button>
      )}
    </div>
  );
}
