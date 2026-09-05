import type { MatchResult } from "@/types/api";

export function MatchScoreBadge({ result, isLoading }: { result?: MatchResult; isLoading: boolean }) {
  if (isLoading) {
    return <span className="text-xs text-black/40 dark:text-white/40">Scoring…</span>;
  }
  if (!result) return null;

  if (!result.Eligibility.Eligible) {
    return (
      <span className="rounded-full bg-red-100 px-2 py-1 text-xs font-medium text-red-800">Not eligible</span>
    );
  }

  const color = scoreColor(result.TotalScore);
  return (
    <div className="flex flex-col items-end gap-0.5">
      <span className={`rounded-full px-2 py-1 text-xs font-semibold ${color}`}>{result.TotalScore}% {result.Grade}</span>
    </div>
  );
}

function scoreColor(score: number): string {
  if (score >= 90) return "bg-green-100 text-green-800";
  if (score >= 80) return "bg-emerald-100 text-emerald-800";
  if (score >= 70) return "bg-amber-100 text-amber-800";
  if (score >= 60) return "bg-orange-100 text-orange-800";
  return "bg-red-100 text-red-800";
}
