"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { AppNav } from "@/components/AppNav";
import { JobCard } from "@/features/jobs/JobCard";
import { RecommendedJobCard } from "@/features/jobs/RecommendedJobCard";
import { DEFAULT_FILTERS, JobFilters, type JobFiltersState } from "@/features/jobs/JobFilters";
import { api } from "@/lib/api";
import type { JobsListResponse, RecommendationsResponse } from "@/types/api";

const TABS = [
  { key: "recommended", label: "Recommended" },
  { key: "latest", label: "Latest 24h" },
  { key: "all", label: "All Software Jobs" },
] as const;
type TabKey = (typeof TABS)[number]["key"];

export default function JobsPage() {
  const [tab, setTab] = useState<TabKey>("recommended");
  const [filters, setFilters] = useState<JobFiltersState>(DEFAULT_FILTERS);
  const [offset, setOffset] = useState(0);
  const limit = 20;

  const params = new URLSearchParams();
  params.set("country", "US");
  if (filters.search) params.set("search", filters.search);
  if (filters.location) params.set("location", filters.location);
  if (filters.remoteType) params.set("remote_type", filters.remoteType);
  if (filters.employmentType) params.set("employment_type", filters.employmentType);
  if (tab === "latest") {
    params.set("posted_within", "24h");
    params.set("sort", "newest");
  } else {
    if (filters.postedWithin) params.set("posted_within", filters.postedWithin);
    if (filters.sort) params.set("sort", filters.sort);
  }
  params.set("limit", String(limit));
  params.set("offset", String(offset));

  const jobsQuery = useQuery({
    queryKey: ["jobs", tab, filters, offset],
    queryFn: () => api.get<JobsListResponse>(`/jobs?${params.toString()}`),
    enabled: tab !== "recommended",
  });

  const recommendationsQuery = useQuery({
    queryKey: ["recommendations"],
    queryFn: () => api.get<RecommendationsResponse>("/recommendations?limit=20"),
    enabled: tab === "recommended",
  });

  const total = jobsQuery.data?.total ?? 0;
  const page = Math.floor(offset / limit) + 1;
  const pageCount = Math.max(1, Math.ceil(total / limit));
  const recommendations = recommendationsQuery.data?.recommendations ?? [];

  function updateFilters(next: JobFiltersState) {
    setFilters(next);
    setOffset(0);
  }

  function selectTab(next: TabKey) {
    setTab(next);
    setOffset(0);
  }

  return (
    <>
      <AppNav />
      <main className="mx-auto flex w-full max-w-4xl flex-1 flex-col gap-6 p-8">
        <div>
          <h1 className="text-2xl font-semibold">Jobs</h1>
          <p className="text-sm text-black/60 dark:text-white/60">
            {tab === "recommended"
              ? recommendationsQuery.data
                ? `${recommendations.length} best matches from the last 24h`
                : "Loading…"
              : jobsQuery.data
                ? `${total} opportunities`
                : "Loading…"}
          </p>
        </div>

        <div className="flex gap-2 border-b border-black/10 dark:border-white/15">
          {TABS.map((t) => (
            <button
              key={t.key}
              type="button"
              onClick={() => selectTab(t.key)}
              className={`px-3 py-2 text-sm font-medium ${
                tab === t.key
                  ? "border-b-2 border-foreground text-foreground"
                  : "text-black/60 dark:text-white/60"
              }`}
            >
              {t.label}
            </button>
          ))}
        </div>

        {tab !== "recommended" && <JobFilters value={filters} onChange={updateFilters} />}

        <div className="flex flex-col gap-4">
          {tab === "recommended" && (
            <>
              {recommendationsQuery.isLoading && <p className="text-sm text-black/60 dark:text-white/60">Loading recommendations…</p>}
              {recommendationsQuery.data && recommendations.length === 0 && (
                <p className="text-sm text-black/60 dark:text-white/60">
                  No strong matches from the last 24 hours yet. New jobs are ranked as they arrive.
                </p>
              )}
              {recommendations.map((job) => (
                <RecommendedJobCard key={job.JobID} job={job} />
              ))}
            </>
          )}

          {tab !== "recommended" && (
            <>
              {jobsQuery.isLoading && <p className="text-sm text-black/60 dark:text-white/60">Loading jobs…</p>}
              {jobsQuery.data?.items.length === 0 && (
                <p className="text-sm text-black/60 dark:text-white/60">No jobs match your filters yet.</p>
              )}
              {jobsQuery.data?.items.map((job) => <JobCard key={job.id} job={job} />)}
            </>
          )}
        </div>

        {tab !== "recommended" && total > limit && (
          <div className="flex items-center justify-between border-t border-black/10 pt-4 dark:border-white/15">
            <button
              type="button"
              onClick={() => setOffset((o) => Math.max(0, o - limit))}
              disabled={offset === 0}
              className="rounded-md border border-black/10 px-3 py-1.5 text-sm disabled:opacity-40 dark:border-white/15"
            >
              Previous
            </button>
            <span className="text-sm text-black/60 dark:text-white/60">
              Page {page} of {pageCount}
            </span>
            <button
              type="button"
              onClick={() => setOffset((o) => o + limit)}
              disabled={offset + limit >= total}
              className="rounded-md border border-black/10 px-3 py-1.5 text-sm disabled:opacity-40 dark:border-white/15"
            >
              Next
            </button>
          </div>
        )}
      </main>
    </>
  );
}

