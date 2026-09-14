"use client";

import { useMutation } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";

import { API_BASE_URL, api } from "@/lib/api";
import type { ApplicationWithJob, ResumeVersion } from "@/types/api";
import type { ApplicationApproval, ApplicationPackage, CompanionHandoff, SubmissionIntent } from "@/types/apply";

const CONFIRMATION_VERSION = "submit-once-v1";
const CONFIRMATION_TEXT = "I approve submitting this exact application package once.";
const HANDOFF_MESSAGE = "APPLYFORGE_COMPANION_HANDOFF";
const HANDOFF_ACK = "APPLYFORGE_COMPANION_ACK";

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
  const [resumeLoading, setResumeLoading] = useState(Boolean(app.ResumeVersionID));
  const [resumeConfirmed, setResumeConfirmed] = useState(false);
  const [approval, setApproval] = useState<ApplicationApproval | null>(null);
  const [intent, setIntent] = useState<SubmissionIntent | null>(null);
  const [acknowledged, setAcknowledged] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [companionStarting, setCompanionStarting] = useState(false);
  const [companionNotice, setCompanionNotice] = useState<string | null>(null);

  useEffect(() => {
    let active = true;

    setPackage(null);
    setResume(null);
    setResumeConfirmed(false);
    setApproval(null);
    setIntent(null);
    setAcknowledged(false);
    setCompanionNotice(null);
    setError(null);

    if (!app.ResumeVersionID) {
      setResumeLoading(false);
      return () => {
        active = false;
      };
    }

    setResumeLoading(true);
    void api
      .get<ResumeVersion>(`/resume-versions/${app.ResumeVersionID}`)
      .then((version) => {
        if (!active) return;
        if (version.JobID !== app.JobID) {
          setError("The resume linked to this application is not tailored for this job. Tailor and attach a job-scoped resume before building the application package.");
          return;
        }
        setResume(version);
      })
      .catch((e) => {
        if (!active) return;
        setError(e instanceof Error ? e.message : "Could not load the resume linked to this application.");
      })
      .finally(() => {
        if (active) setResumeLoading(false);
      });

    return () => {
      active = false;
    };
  }, [app.ID, app.JobID, app.ResumeVersionID]);

  const buildReview = useMutation({
    mutationFn: async () => {
      if (!resume || !resumeConfirmed) {
        throw new Error("Confirm the job-scoped resume before building the application package.");
      }
      if (resume.ID !== app.ResumeVersionID || resume.JobID !== app.JobID) {
        throw new Error("The application resume changed. Review the latest job-scoped resume before continuing.");
      }

      const built = await api.post<ApplicationPackage>(`/applications/${app.ID}/package`);
      if (built.resume_version_id !== resume.ID) {
        throw new Error("The application resume changed while the package was being built. Reopen the review and confirm the latest resume.");
      }
      const version = await api.get<ResumeVersion>(`/resume-versions/${built.resume_version_id}`);
      if (version.JobID !== app.JobID) {
        throw new Error("The packaged resume is not scoped to this job.");
      }
      return { built, version };
    },
    onSuccess: ({ built, version }) => {
      setError(null);
      setPackage(built);
      setResume(version);
      setApproval(null);
      setIntent(null);
      setAcknowledged(false);
      setCompanionNotice(null);
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
  const resumeReady = Boolean(resume && resume.JobID === app.JobID && resume.ID === app.ResumeVersionID);

  const launchCompanion = async () => {
    if (!pkg || !resume || !intent || companionStarting) return;
    setCompanionStarting(true);
    setError(null);
    setCompanionNotice(null);

    // Open synchronously from the user's click so popup blockers do not prevent
    // the ATS navigation after the secure handoff is prepared.
    const target = window.open("about:blank", "_blank");
    if (!target) {
      setCompanionStarting(false);
      setError("Your browser blocked the ATS tab. Allow popups for ApplyForge and try again.");
      return;
    }
    target.document.title = "Preparing ApplyForge application…";

    try {
      const [handoff, pdfBase64] = await Promise.all([
        api.post<CompanionHandoff>(`/submission-intents/${intent.id}/companion-handoff`),
        downloadResumeBase64(resume.ID),
      ]);

      const detected = await sendCompanionHandoff({
        source: "applyforge-web-v1",
        type: HANDOFF_MESSAGE,
        payload: {
          apiBaseUrl: API_BASE_URL,
          intentId: handoff.intent_id,
          packageId: handoff.package_id,
          token: handoff.token,
          expiresAt: handoff.expires_at,
          destinationUrl: pkg.destination_url,
          destinationOrigin: pkg.destination_origin,
          resumeFilename: "applyforge-resume.pdf",
          resumePdfBase64: pdfBase64,
        },
      });

      if (detected) {
        setCompanionNotice("Browser companion received the approved package. It will offer to fill and submit on the ATS page.");
      } else {
        setCompanionNotice("Browser companion was not detected. The ATS page is opening for manual completion; ApplyForge will not mark it applied automatically.");
      }
      target.location.href = pkg.destination_url;
    } catch (e) {
      target.close();
      setError(e instanceof Error ? e.message : "Could not prepare the browser companion handoff.");
    } finally {
      setCompanionStarting(false);
    }
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
          <button type="button" onClick={onClose} className="rounded-md border px-3 py-1.5 text-sm dark:border-white/15">Close</button>
        </div>

        {!pkg && (
          <div className="space-y-5">
            <section className="rounded-lg border border-black/10 p-5 dark:border-white/15">
              <div className="flex flex-wrap items-start justify-between gap-3">
                <div>
                  <h3 className="font-medium">Resume for this application</h3>
                  <p className="mt-1 text-sm text-black/60 dark:text-white/60">A package can only be built from a resume version tailored to this exact job.</p>
                </div>
                <a href={`/jobs/${app.JobID}/tailor`} className="text-sm underline">Tailor again</a>
              </div>

              {resumeLoading && <p className="mt-4 text-sm text-black/60 dark:text-white/60">Loading linked resume…</p>}

              {!resumeLoading && !app.ResumeVersionID && (
                <div className="mt-4 rounded-md border border-amber-300 bg-amber-50 p-4 text-sm text-amber-950 dark:bg-amber-950/20 dark:text-amber-100">
                  <p className="font-medium">No job-scoped resume is attached.</p>
                  <p className="mt-1">Tailor a resume for this job and attach that generated version to the application before continuing.</p>
                  <a href={`/jobs/${app.JobID}/tailor`} className="mt-3 inline-block rounded-md bg-foreground px-4 py-2 text-background">Tailor resume for this job →</a>
                </div>
              )}

              {!resumeLoading && resumeReady && resume && (
                <div className="mt-4 rounded-md bg-black/[0.03] p-4 text-sm dark:bg-white/[0.05]">
                  <div className="flex flex-wrap items-start justify-between gap-4">
                    <div>
                      <p className="font-medium">Tailored resume v{resume.VersionNumber}</p>
                      <p className="mt-1 text-black/60 dark:text-white/60">Match {resume.MatchScore ?? "—"}% · created {new Date(resume.CreatedAt).toLocaleString()}</p>
                      <p className="mt-1 text-xs font-medium text-green-700 dark:text-green-300">Verified for this exact job</p>
                    </div>
                    <a href={`${API_BASE_URL}/resume-versions/${resume.ID}/download?format=pdf`} target="_blank" rel="noreferrer" className="text-sm underline">Preview PDF</a>
                  </div>

                  {!resumeConfirmed ? (
                    <button type="button" onClick={() => { setResumeConfirmed(true); setError(null); }} className="mt-4 rounded-md bg-foreground px-4 py-2 text-sm text-background">Use this resume for application</button>
                  ) : (
                    <p className="mt-4 rounded-md border border-green-300 bg-green-50 px-3 py-2 text-sm text-green-900 dark:bg-green-950/20 dark:text-green-100">This exact job-scoped resume version will be included in the application package.</p>
                  )}
                </div>
              )}
            </section>

            <section className="rounded-lg border border-black/10 p-5 dark:border-white/15">
              <h3 className="font-medium">Build exact application package</h3>
              <p className="mt-2 text-sm text-black/60 dark:text-white/60">ApplyForge will snapshot the confirmed job-scoped resume version, saved application answers, and stored ATS destination. Any later resume, answer, or destination change creates a different package and requires another approval.</p>
              <button type="button" onClick={() => buildReview.mutate()} disabled={!resumeReady || !resumeConfirmed || buildReview.isPending} className="mt-4 rounded-md bg-foreground px-4 py-2 text-sm text-background disabled:cursor-not-allowed disabled:opacity-50">{buildReview.isPending ? "Building review…" : "Build review package"}</button>
              {!resumeConfirmed && resumeReady && <p className="mt-2 text-xs text-black/50 dark:text-white/50">Confirm the resume above to enable package creation.</p>}
            </section>
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
                <p className="font-medium">Submission intent ready.</p>
                <p className="mt-1">Intent {intent.id.slice(0, 8)}… is bound to this package with idempotency key {intent.idempotency_key.slice(0, 12)}…</p>
                <div className="mt-4 flex flex-wrap gap-2">
                  <button type="button" onClick={() => void launchCompanion()} disabled={companionStarting} className="rounded-md bg-foreground px-4 py-2 text-background disabled:opacity-50">{companionStarting ? "Preparing companion…" : "Continue with browser companion →"}</button>
                  <a href={pkg.destination_url} target="_blank" rel="noreferrer" className="rounded-md border border-blue-300 px-4 py-2">Open manually</a>
                </div>
                <p className="mt-3 text-xs opacity-80">The companion can submit only this exact approved package. ApplyForge changes the application to Applied only after a fenced executor receipt is confirmed.</p>
              </section>
            )}
          </div>
        )}

        {approval && <p className="mt-4 text-xs text-black/50 dark:text-white/50">Approval expires {new Date(approval.expires_at).toLocaleString()}.</p>}
        {companionNotice && <p className="mt-4 rounded-md bg-blue-50 p-3 text-sm text-blue-800 dark:bg-blue-950/20 dark:text-blue-100">{companionNotice}</p>}
        {error && <p role="alert" className="mt-4 rounded-md bg-red-50 p-3 text-sm text-red-700">{error}</p>}
      </div>
    </div>
  );
}

async function downloadResumeBase64(resumeVersionID: string): Promise<string> {
  const response = await fetch(`${API_BASE_URL}/resume-versions/${resumeVersionID}/download?format=pdf`, {
    credentials: "include",
  });
  if (!response.ok) throw new Error("Could not load the approved resume PDF for the browser companion.");
  const bytes = new Uint8Array(await response.arrayBuffer());
  let binary = "";
  const chunkSize = 0x8000;
  for (let offset = 0; offset < bytes.length; offset += chunkSize) {
    binary += String.fromCharCode(...bytes.subarray(offset, offset + chunkSize));
  }
  return btoa(binary);
}

function sendCompanionHandoff(message: unknown): Promise<boolean> {
  return new Promise((resolve) => {
    let settled = false;
    const finish = (value: boolean) => {
      if (settled) return;
      settled = true;
      window.removeEventListener("message", onMessage);
      resolve(value);
    };
    const onMessage = (event: MessageEvent) => {
      if (event.source !== window || event.origin !== window.location.origin) return;
      const data = event.data as { source?: string; type?: string } | undefined;
      if (data?.source === "applyforge-companion-v1" && data.type === HANDOFF_ACK) finish(true);
    };
    window.addEventListener("message", onMessage);
    window.postMessage(message, window.location.origin);
    window.setTimeout(() => finish(false), 700);
  });
}
