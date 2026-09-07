"use client";

import Link from "next/link";
import type { RecommendedJob } from "@/types/api";

export function RecommendedJobCard({ job }: { job: RecommendedJob }) {
  return (
    <div className="flex flex-col gap-3 rounded-md border border-black/10 p-4 dark:border-white/15">
      <div className="flex items-start justify-between gap-4">
        <div>
          <h3 className="font-medium">{job.Title}</h3>
          <p className="text-sm text-black/60 dark:text-white/60">
            {job.CompanyName} {job.LocationText ? `\u00b7 ${job.LocationText}` : ""}
          </p>
        </div>
        <span className="rounded-full bg-green-100 px-2 py-0.5 text-xs font-medium text-green-800">
          {job.FinalScore}% match
        </span>
      </div>

      <div className="flex flex-wrap items-center gap-2 text-xs text-black/60 dark:text-white/60">
        {job.EmploymentType && <span className="rounded-full bg-black/5 px-2 py-0.5 dark:bg-white/10">{job.EmploymentType}</span>}
        {job.RemoteType && <span className="rounded-full bg-black/5 px-2 py-0.5 dark:bg-white/10">{job.RemoteType}</span>}
        {job.AIRecommendation && (
          <span className="rounded-full bg-purple-100 px-2 py-0.5 font-medium text-purple-800">{job.AIRecommendation}</span>
        )}
      </div>

      {job.AIReason && <p className="text-sm text-black/70 dark:text-white/70">{job.AIReason}</p>}

      <div className="flex gap-3 text-sm">
        <Link href={`/jobs/${job.JobID}`} className="rounded-md border border-black/10 px-3 py-1.5 dark:border-white/15">
          View Analysis
        </Link>
        {job.ApplyURL && (
          <a
            href={job.ApplyURL}
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
