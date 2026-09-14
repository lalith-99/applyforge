"use client";

import { useMutation } from "@tanstack/react-query";
import { useMemo, useState } from "react";

import { API_BASE_URL, api } from "@/lib/api";
import type { ApplicationWithJob, ResumeVersion } from "@/types/api";
import type { ApplicationApproval, ApplicationPackage, SubmissionIntent } from "@/types/apply";

const CONFIRMATION_VERSION = "submit-once-v1";
const CONFIRMATION_TEXT = "I approve submitting this exact application package once.";

type Step = "review" | "approved" | "ready";

export function ApplyReviewModal({
  app,
  onClose,
}: {
  app: ApplicationWithJob;
  onClose: () => void;
}) {
  const [pkg, setPackage] = useState<ApplicationPackage | null>(null);
  const [resume, setResume] = useState<ResumeVersion | null>(null);
  const [approval, setApproval] = useState<ApplicationApproval | null>(null);
  const [intent, setIntent] = useState<SubmissionIntent | null>(null);
  const [acknowledged, setAcknowledged] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const buildReview = useMutation({
    mutationFn: async () => {
      const built = await api.post<ApplicationPackage>(`/applications/${app.ID}/package`);
      const version = await api.get<ResumeVersion>(`/resume-versions/${built.resume_version_id}`);
      return { built, version };
    },
    onSuccess: ({ built, version }) => {
      setError(null);
      setPackage(built);
      setResume(version);
      setApproval(null);
      setIntent(null);
      setAcknowledged(false);
    },
    onError: (e) => setError(e instanceof Error ? e.message : "Could not build application package."),
  });

  const approve = useMutation({
    mutationFn: async () => {
      if (!pkg) throw new Error("Build the review package first.");
      return api.post<ApplicationApproval>(`/application-packages/${pkg.id}/approve`, {
        confirmation_version: CONFIRMATION_VERSION,
        confirmation_text: CONFIRMATION_TEXT,
      });
    },
    onSuccess: (data) => {
      setError(null);
      setApproval(data);
    },
    onError: (e) => setError(e instanceof Error ? e.message : "Could not approve application package."),
  });

  const createIntent = useMutation({
    mutationFn: async () => {
      if (!pkg) throw new Error("Application package is missing.");
      return api.post<SubmissionIntent>(`/application-packages/${pkg.id}/submission-intent`);
    },
    onSuccess: (data) => {
      setError(null);
      setIntent(data);
    },
    onError: (e) => setError(e instanceof Error ? e.message : "Could not create submission intent."),
  });

  const step: Step = intent ? "ready" : approval ? "approved" : "review";
  const answers = useMemo(() => pkg?.answers_json ?? {}, [pkg]);

  const launch = () => {
    if (!pkg || !intent) return;
    window.open(pkg.destination_url, "_blank", "noopener,noreferrer");
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" role="dialog" aria-modal="true">
      <div className="max-h-[92vh] w-full max-w-4xl overflow-y-auto rounded-xl bg-white p-6 shadow-xl dark:bg-neutral-950">
        <div className="mb-5 flex items-start justify-between gap-4">
          <div>
            <p className="text-xs font-semibold uppercase tracking-wide text-black/50 dark:text-white/50">Application review</p>
            <h2 className="text-xl font-semibold">{app.Title}</h2>
            <p className="text-sm text-black/60 dark:text-white/60">{app.CompanyName}</p>
          </div>
          <button type="button" onClick={onClose} className="rounded-md border px-3 py-1.5 text-sm dark:border-white/15">
            Close
          </button>
        </div>

        {!pkg && (
          <div className="rounded-lg border border-black/10 p-5 dark:border-white/15">
            <h3 className="font-medium">Build exact application package</h3>
            <p className="mt-2 text-sm text-black/60 dark:text-white/60">
              ApplyForge will snapshot the job-scoped tailored resume, saved application answers, and the stored ATS destination. Any later change creates a different package and requires another approval.
            </p>
            <button type="button" onClick={() => buildReview.mutate()} disabled={buildReview.isPending} className="mt-4 rounded-md bg-foreground px-4 py-2 text-sm text-background disabled:opacity-60">
              {buildReview.isPending ? "Building review…" : "Build review package"}
            </button>
          </div>
        )}

        {pkg && resume && (
          <div className="space-y-5">
            <section className="grid gap-3 rounded-lg border border-black/10 p-4 text-sm dark:border-white/15 md:grid-cols-2">
              <div><p className="text-xs uppercase text-black/50 dark:text-white/50">Destination</p><p className="mt-1 break-all font-medium">{pkg.destination_origin}</p></div>
              <div><p className="text-xs uppercase text-black/50 dark:text-white/50">Package fingerprint</p><p className="mt-1 break-all font-mono text-xs">{pkg.package_hash}</p></div>
              <div><p className="text-xs uppercase text-black/50 dark:text-white/50">Resume version</p><p className="mt-1">v{resume.VersionNumber} · match {resume.MatchScore ?? "—"}%</p></div>
              <div><p className="text-xs uppercase text-black/50 dark:text-white/50">Approval scope</p><p className="mt-1">Submit this exact package once</p></div>
            </section>

            <section className="rounded-lg border border-black/10 p-4 dark:border-white/15">
              <div className="flex items-center justify-between gap-3">
                <h3 className="font-medium">Tailored resume preview</h3>
                <a href={`${API_BASE_URL}/resume-versions/${resume.ID}/download?format=pdf`} target="_blank" rel="noreferrer" className="text-sm underline">Open PDF</a>
              </div>
              <div className="mt-3 space-y-3 text-sm">
                {resume.Content.Summary && <p>{resume.Content.Summary}</p>}
                {resume.Content.Skills.length > 0 && <div><p className="text-xs font-semibold uppercase text-black/50 dark:text-white/50">Skills</p><p className="mt-1">{resume.Content.Skills.join(" · ")}</p></div>}
                <div className="space-y-3">
                  {resume.Content.Experiences.map((experience, index) => (
                    <div key={`${experience.Company ?? "experience"}-${index}`}>
                      <p className="font-medium">{experience.Title ?? "Role"} · {experience.Company ?? "Company"}</p>
                      <ul className="mt-1 list-disc space-y-1 pl-5 text-black/70 dark:text-white/70">{experience.Bullets.slice(0, 4).map((bullet) => <li key={bullet}>{bullet}</li>)}</ul>
                    </div>
                  ))}
                </div>
              </div>
            </section>

            <section className="rounded-lg border border-black/10 p-4 dark:border-white/15">
              <h3 className="font-medium">Application answers snapshot</h3>
              <div className="mt-3 grid gap-2 text-sm md:grid-cols-2">
                {Object.entries(answers).map(([key, value]) => (
                  <div key={key} className="rounded-md bg-black/[0.03] p-2 dark:bg-white/[0.05]">
                    <p className="text-xs uppercase text-black/50 dark:text-white/50">{key.replaceAll("_", " ")}</p>
                    <p className="mt-1 break-words">{typeof value === "object" ? JSON.stringify(value) : String(value ?? "—")}</p>
                  </div>
                ))}
              </div>
            </section>

            {step === "review" && (
              <section className="rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-950 dark:bg-amber-950/20 dark:text-amber-100">
                <label className="flex items-start gap-3"><input type="checkbox" checked={acknowledged} onChange={(e) => setAcknowledged(e.target.checked)} className="mt-1" /><span>{CONFIRMATION_TEXT} Approval expires after 24 hours and does not authorize a different resume, answer set, destination, or future package.</span></label>
                <button type="button" disabled={!acknowledged || approve.isPending} onClick={() => approve.mutate()} className="mt-4 rounded-md bg-foreground px-4 py-2 text-background disabled:opacity-50">{approve.isPending ? "Approving…" : "Approve exact package"}</button>
              </section>
            )}

            {step === "approved" && (
              <section className="rounded-lg border border-green-300 bg-green-50 p-4 text-sm text-green-950 dark:bg-green-950/20 dark:text-green-100">
                <p className="font-medium">Package approved.</p><p className="mt-1">Create the durable one-time submission intent before opening the ATS.</p>
                <button type="button" onClick={() => createIntent.mutate()} disabled={createIntent.isPending} className="mt-4 rounded-md bg-foreground px-4 py-2 text-background disabled:opacity-50">{createIntent.isPending ? "Preparing…" : "Prepare application"}</button>
              </section>
            )}

            {step === "ready" && intent && (
              <section className="rounded-lg border border-blue-300 bg-blue-50 p-4 text-sm text-blue-950 dark:bg-blue-950/20 dark:text-blue-100">
                <p className="font-medium">Submission intent ready.</p><p className="mt-1">Intent {intent.id.slice(0, 8)}… is bound to this package with idempotency key {intent.idempotency_key.slice(0, 12)}…</p>
                <div className="mt-4 flex flex-wrap gap-2"><button type="button" onClick={launch} className="rounded-md bg-foreground px-4 py-2 text-background">Open ATS application →</button><a href={pkg.destination_url} target="_blank" rel="noreferrer" className="rounded-md border border-blue-300 px-4 py-2">Open in new tab</a></div>
                <p className="mt-3 text-xs opacity-80">The server will not mark this application as applied merely because the ATS page was opened. A confirmed executor receipt is required.</p>
              </section>
            )}
          </div>
        )}

        {approval && <p className="mt-4 text-xs text-black/50 dark:text-white/50">Approval expires {new Date(approval.expires_at).toLocaleString()}.</p>}
        {error && <p role="alert" className="mt-4 rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</p>}
      </div>
    </div>
  );
}
