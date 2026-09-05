"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { AppNav } from "@/components/AppNav";
import { JobCard } from "@/features/jobs/JobCard";
import { DEFAULT_FILTERS, JobFilters, type JobFiltersState } from "@/features/jobs/JobFilters";
import { api } from "@/lib/api";
import type { JobsListResponse } from "@/types/api";

export default function JobsPage() {
  const [filters, setFilters] = useState<JobFiltersState>(DEFAULT_FILTERS);

  const params = new URLSearchParams();
  if (filters.search) params.set("search", filters.search);
  if (filters.remoteType) params.set("remote_type", filters.remoteType);
  if (filters.employmentType) params.set("employment_type", filters.employmentType);
  if (filters.postedWithin) params.set("posted_within", filters.postedWithin);
  if (filters.sort) params.set("sort", filters.sort);
  params.set("limit", "20");

  const jobsQuery = useQuery({
    queryKey: ["jobs", filters],
    queryFn: () => api.get<JobsListResponse>(`/jobs?${params.toString()}`),
  });

  return (
    <>
      <AppNav />
      <main className="mx-auto flex w-full max-w-4xl flex-1 flex-col gap-6 p-8">
        <div>
          <h1 className="text-2xl font-semibold">Jobs</h1>
          <p className="text-sm text-black/60 dark:text-white/60">
            {jobsQuery.data ? `${jobsQuery.data.total} opportunities` : "Loading…"}
          </p>
        </div>

        <JobFilters value={filters} onChange={setFilters} />

        <div className="flex flex-col gap-4">
          {jobsQuery.isLoading && <p className="text-sm text-black/60 dark:text-white/60">Loading jobs…</p>}
          {jobsQuery.data?.items.length === 0 && (
            <p className="text-sm text-black/60 dark:text-white/60">No jobs match your filters yet.</p>
          )}
          {jobsQuery.data?.items.map((job) => <JobCard key={job.id} job={job} />)}
        </div>
      </main>
    </>
  );
}
