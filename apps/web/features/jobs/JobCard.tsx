"use client";

import { useQuery } from "@tanstack/react-query";
import Link from "next/link";
import { api } from "@/lib/api";
import type { JobSummary, MatchResult } from "@/types/api";
import { JobAgeBadge } from "./JobAgeBadge";
import { MatchScoreBadge } from "./MatchScoreBadge";

export function JobCard({ job }: { job: JobSummary }) {
  const matchQuery = useQuery({
    queryKey: ["job-match", job.id],
    queryFn: () => api.get<MatchResult>(`/jobs/${job.id}/match`),
    staleTime: 5 * 60 * 1000,
  });

  const salary =
    job.salary_min || job.salary_max
      ? `${job.salary_currency ?? "$"}${job.salary_min ?? "?"}\u2013${job.salary_max ?? "?"}`
      : null;

  return (
    <div className="flex flex-col gap-3 rounded-md border border-black/10 p-4 dark:border-white/15">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h3 className="font-medium">{job.title}</h3>
          <p className="text-sm text-black/60 dark:text-white/60">
            {job.company_name} {job.location_text ? `\u00b7 ${job.location_text}` : ""}
          </p>
        </div>
        <MatchScoreBadge result={matchQuery.data} isLoading={matchQuery.isLoading} />
      </div>

      <div className="flex flex-wrap items-center gap-2 text-xs text-black/60 dark:text-white/60">
        {job.employment_type && <span className="rounded-full bg-black/5 px-2 py-0.5 dark:bg-white/10">{job.employment_type}</span>}
        {job.remote_type && <span className="rounded-full bg-black/5 px-2 py-0.5 dark:bg-white/10">{job.remote_type}</span>}
        {salary && <span className="rounded-full bg-black/5 px-2 py-0.5 dark:bg-white/10">{salary}</span>}
        <JobAgeBadge postedAt={job.posted_at} firstSeenAt={job.first_seen_at} />
      </div>

      {matchQuery.data && matchQuery.data.MatchedSkills.length > 0 && (
        <div className="flex flex-wrap gap-1">
          {Array.from(new Set(matchQuery.data.MatchedSkills)).slice(0, 5).map((skill) => (
            <span key={skill} className="rounded-full bg-green-100 px-2 py-0.5 text-xs text-green-800">
              {skill}
            </span>
          ))}
        </div>
      )}

      <div className="flex gap-3 text-sm">
        <Link href={`/jobs/${job.id}`} className="rounded-md border border-black/10 px-3 py-1.5 dark:border-white/15">
          View Analysis
        </Link>
        {job.apply_url && (
          <a
            href={job.apply_url}
            target="_blank"
            rel="noopener noreferrer"
            className="rounded-md bg-foreground px-3 py-1.5 text-background"
          >
            Apply
          </a>
        )}
      </div>
    </div>
  );
}
