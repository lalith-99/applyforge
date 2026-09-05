"use client";

import { useQuery } from "@tanstack/react-query";
import { AppNav } from "@/components/AppNav";
import { api } from "@/lib/api";
import type { AnalyticsDashboard } from "@/types/api";

const STATUS_LABELS: Record<string, string> = {
  SAVED: "Saved",
  READY_TO_APPLY: "Ready to Apply",
  APPLIED: "Applied",
  RECRUITER_SCREEN: "Recruiter Screen",
  ASSESSMENT: "Assessment",
  TECHNICAL_INTERVIEW: "Technical Interview",
  FINAL_INTERVIEW: "Final Interview",
  OFFER: "Offer",
  REJECTED: "Rejected",
  WITHDRAWN: "Withdrawn",
};

export default function AnalyticsPage() {
  const dashboardQuery = useQuery({
    queryKey: ["analytics-dashboard"],
    queryFn: () => api.get<AnalyticsDashboard>("/analytics/dashboard"),
  });

  const dashboard = dashboardQuery.data;
  const maxFunnelCount = dashboard ? Math.max(1, ...dashboard.Funnel.map((f) => f.Count)) : 1;

  return (
    <>
      <AppNav />
      <main className="mx-auto flex w-full max-w-4xl flex-1 flex-col gap-8 p-8">
        <h1 className="text-2xl font-semibold">Analytics</h1>

        {dashboardQuery.isLoading && <p className="text-sm text-black/60 dark:text-white/60">Loading…</p>}

        {dashboard && (
          <>
            <section className="grid grid-cols-2 gap-4 sm:grid-cols-4">
              <StatCard label="Jobs Discovered" value={dashboard.JobsDiscovered} />
              <StatCard label="Applications Tracked" value={dashboard.TotalApplications} />
              <StatCard label="Tailoring Runs" value={dashboard.TailoringRunsCount} />
              <StatCard label="High Matches (90%+)" value={dashboard.HighMatchesCount} />
            </section>

            <section className="grid grid-cols-2 gap-4 sm:grid-cols-2">
              <StatCard
                label="Response Rate"
                value={`${dashboard.ResponseRatePercent.toFixed(0)}%`}
                hint="Applied → Recruiter Screen"
              />
              <StatCard
                label="Average Match Score"
                value={dashboard.AverageMatchScore != null ? `${dashboard.AverageMatchScore.toFixed(0)}%` : "—"}
              />
            </section>

            <section>
              <h2 className="mb-3 text-lg font-medium">Conversion Funnel</h2>
              <div className="flex flex-col gap-2">
                {dashboard.Funnel.map((stage) => (
                  <div key={stage.Status} className="flex items-center gap-3 text-sm">
                    <span className="w-40 shrink-0 text-black/70 dark:text-white/70">
                      {STATUS_LABELS[stage.Status] ?? stage.Status}
                    </span>
                    <div className="h-4 flex-1 rounded bg-black/5 dark:bg-white/10">
                      <div
                        className="h-4 rounded bg-foreground"
                        style={{ width: `${(stage.Count / maxFunnelCount) * 100}%` }}
                      />
                    </div>
                    <span className="w-10 shrink-0 text-right font-medium">{stage.Count}</span>
                  </div>
                ))}
              </div>
            </section>

            {dashboard.ApplicationsByStatus && dashboard.ApplicationsByStatus.length > 0 && (
              <section>
                <h2 className="mb-3 text-lg font-medium">Applications by Current Status</h2>
                <div className="flex flex-wrap gap-2">
                  {dashboard.ApplicationsByStatus.map((sc) => (
                    <span
                      key={sc.Status}
                      className="rounded-full border border-black/10 px-3 py-1 text-xs dark:border-white/15"
                    >
                      {STATUS_LABELS[sc.Status] ?? sc.Status}: {sc.Count}
                    </span>
                  ))}
                </div>
              </section>
            )}
          </>
        )}
      </main>
    </>
  );
}

function StatCard({ label, value, hint }: { label: string; value: string | number; hint?: string }) {
  return (
    <div className="rounded-md border border-black/10 p-4 dark:border-white/15">
      <p className="text-xs font-medium text-black/50 dark:text-white/50">{label}</p>
      <p className="text-2xl font-semibold">{value}</p>
      {hint && <p className="text-xs text-black/50 dark:text-white/50">{hint}</p>}
    </div>
  );
}
