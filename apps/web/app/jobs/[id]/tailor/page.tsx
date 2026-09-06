"use client";

import { useMutation, useQuery } from "@tanstack/react-query";
import { use, useEffect, useState, type ReactNode } from "react";
import { AppNav } from "@/components/AppNav";
import { api, ApiError, API_BASE_URL } from "@/lib/api";
import { DefendBulletDrawer } from "@/features/learning/DefendBulletDrawer";
import { QuickPrepDrawer } from "@/features/learning/QuickPrepDrawer";
import type { ResumeSummary, ResumeVersion, ResumeVersionContent, TailoringRun, TailoringSuggestion } from "@/types/api";

const MODES = ["STRICT", "GROWTH", "MAX_MATCH"] as const;

export default function TailorResumePage({ params }: { params: Promise<{ id: string }> }) {
  const { id: jobId } = use(params);
  const [resumeId, setResumeId] = useState<string>("");
  const [mode, setMode] = useState<(typeof MODES)[number]>("GROWTH");
  const [run, setRun] = useState<TailoringRun | null>(null);
  const [previewOpen, setPreviewOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [editedContent, setEditedContent] = useState<ResumeVersionContent | null>(null);
  const [latestVersion, setLatestVersion] = useState<ResumeVersion | null>(null);

  const resumesQuery = useQuery({
    queryKey: ["resumes"],
    queryFn: () => api.get<ResumeSummary[]>("/resumes"),
  });

  const tailorMutation = useMutation({
    mutationFn: () => api.post<TailoringRun>(`/jobs/${jobId}/tailor`, { resume_id: resumeId, mode }),
    onSuccess: (data) => setRun(data),
  });

  const isProcessing = !!run && run.status !== "COMPLETED" && run.status !== "FAILED";

  useEffect(() => {
    if (!isProcessing || !run) return;
    const interval = setInterval(async () => {
      const latest = await api.get<TailoringRun>(`/tailoring/${run.id}`);
      setRun(latest);
    }, 2000);
    return () => clearInterval(interval);
  }, [isProcessing, run]);

  const updateSuggestion = useMutation({
    mutationFn: ({ suggestionId, status }: { suggestionId: string; status: string }) =>
      api.patch<TailoringSuggestion>(`/tailoring/${run!.id}/suggestions/${suggestionId}`, { status }),
    onSuccess: (updated) => {
      setRun((prev) =>
        prev
          ? { ...prev, suggestions: (prev.suggestions ?? []).map((s) => (s.ID === updated.ID ? updated : s)) }
          : prev,
      );
    },
  });

  const approveAll = useMutation({
    mutationFn: () => api.post<TailoringSuggestion[]>(`/tailoring/${run!.id}/approve-all`),
    onSuccess: (updated) => setRun((prev) => (prev ? { ...prev, suggestions: updated } : prev)),
  });

  const generateResume = useMutation({
    mutationFn: () =>
      api.post<ResumeVersion>(`/resumes/${run!.resume_id}/versions`, {
        job_id: jobId,
        tailoring_run_id: run!.id,
        ...(editedContent ? { content: serializeResumeContent(editedContent) } : {}),
      }),
    onSuccess: (version) => {
      setLatestVersion(version);
      setEditedContent(normalizeResumeContent(version.Content));
      setPreviewOpen(true);
    },
  });

  const parsedResumes = resumesQuery.data?.filter((r) => r.status === "PARSED") ?? [];

  return (
    <>
      <AppNav />
      <main className="mx-auto flex w-full max-w-3xl flex-1 flex-col gap-6 p-8">
        <h1 className="text-2xl font-semibold">Tailor Resume</h1>

        {!run && (
          <div className="flex flex-col gap-4 rounded-md border border-black/10 p-6 dark:border-white/15">
            <label className="flex flex-col gap-1 text-sm">
              <span className="font-medium">Resume</span>
              <select
                value={resumeId}
                onChange={(e) => setResumeId(e.target.value)}
                className="rounded-md border border-black/10 bg-transparent px-3 py-2 dark:border-white/15"
              >
                <option value="">Select a parsed resume…</option>
                {parsedResumes.map((r) => (
                  <option key={r.id} value={r.id}>
                    {r.original_filename}
                  </option>
                ))}
              </select>
            </label>

            <label className="flex flex-col gap-1 text-sm">
              <span className="font-medium">Tailoring mode</span>
              <select
                value={mode}
                onChange={(e) => setMode(e.target.value as (typeof MODES)[number])}
                className="rounded-md border border-black/10 bg-transparent px-3 py-2 dark:border-white/15"
              >
                {MODES.map((m) => (
                  <option key={m} value={m}>
                    {m}
                  </option>
                ))}
              </select>
            </label>

            {tailorMutation.isError && (
              <p className="text-sm text-red-600">
                {tailorMutation.error instanceof ApiError ? tailorMutation.error.message : "Tailoring failed."}
              </p>
            )}

            <button
              type="button"
              disabled={!resumeId || tailorMutation.isPending}
              onClick={() => tailorMutation.mutate()}
              className="rounded-md bg-foreground px-4 py-2 text-sm font-medium text-background disabled:opacity-60"
            >
              {tailorMutation.isPending ? "Tailoring…" : "Tailor Resume"}
            </button>
          </div>
        )}

        {run && (
          <div className="flex flex-col gap-6">
            <div className="flex items-center justify-between rounded-md border border-black/10 p-4 dark:border-white/15">
              <div>
                <p className="text-sm text-black/60 dark:text-white/60">Resume Alignment</p>
                <p className="text-2xl font-semibold">
                  {run.alignment_score_before}% → {run.alignment_score_after}%
                </p>
              </div>
              <button
                type="button"
                onClick={() => approveAll.mutate()}
                disabled={isProcessing}
                className="rounded-md border border-black/10 px-4 py-2 text-sm dark:border-white/15 disabled:opacity-60"
              >
                Approve All Selected
              </button>
            </div>

            {!isProcessing && run.alignment_score_after !== null && (
              <TailoringScore run={run} />
            )}

            {isProcessing && (
              <p className="text-sm text-black/60 dark:text-white/60">
                Tailoring in progress ({run.status.toLowerCase()})…
              </p>
            )}

            <div className="flex flex-col gap-4">
              {(run.suggestions ?? []).map((s) => (
                <SuggestionCard
                  key={s.ID}
                  suggestion={s}
                  onApprove={() => updateSuggestion.mutate({ suggestionId: s.ID, status: "APPROVED" })}
                  onReject={() => updateSuggestion.mutate({ suggestionId: s.ID, status: "REJECTED" })}
                />
              ))}
            </div>

            <div className="flex flex-col gap-3 rounded-md border border-black/10 p-4 dark:border-white/15">
              <div className="flex items-center justify-between">
                <p className="text-sm font-medium">Generate Tailored Resume</p>
                <button
                  type="button"
                  onClick={() => generateResume.mutate()}
                  disabled={generateResume.isPending}
                  className="rounded-md bg-foreground px-4 py-2 text-sm font-medium text-background disabled:opacity-60"
                >
                  {generateResume.isPending ? "Generating…" : "Generate PDF/DOCX"}
                </button>
              </div>
              {generateResume.isError && (
                <p className="text-sm text-red-600">
                  {generateResume.error instanceof ApiError ? generateResume.error.message : "Could not generate resume documents."}
                </p>
              )}
              {generateResume.data && (
                <div className="flex flex-col gap-4 text-sm">
                  <button
                    type="button"
                    onClick={() => setPreviewOpen((open) => !open)}
                    className="self-start rounded-md border border-black/10 px-3 py-1.5 dark:border-white/15"
                  >
                    {previewOpen ? "Hide Preview" : "Preview Resume"}
                  </button>
                  {previewOpen && <ResumePreview content={editedContent ?? latestVersion?.Content ?? generateResume.data.Content} />}
                  <button
                    type="button"
                    onClick={() => setEditOpen((open) => !open)}
                    className="self-start rounded-md border border-black/10 px-3 py-1.5 dark:border-white/15"
                  >
                    {editOpen ? "Hide Editor" : "Edit Resume"}
                  </button>
                  {editOpen && editedContent && (
                    <ResumeEditor content={editedContent} onChange={setEditedContent} />
                  )}
                  <div className="flex gap-4">
                  <a
                    href={`${API_BASE_URL}/resume-versions/${latestVersion?.ID ?? generateResume.data.ID}/download?format=pdf`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="rounded-md border border-black/10 px-3 py-1.5 dark:border-white/15"
                  >
                    Download PDF
                  </a>
                  <a
                    href={`${API_BASE_URL}/resume-versions/${latestVersion?.ID ?? generateResume.data.ID}/download?format=docx`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="rounded-md border border-black/10 px-3 py-1.5 dark:border-white/15"
                  >
                    Download DOCX
                  </a>
                  </div>
                </div>
              )}
            </div>
          </div>
        )}
      </main>
    </>
  );
}

function TailoringScore({ run }: { run: TailoringRun }) {
  const alignment = run.alignment_score_after ?? 0;
  const ats = run.critic_result?.ats_score ?? Math.round((run.keyword_coverage?.after ?? 0) * 100);
  const overall = Math.round((alignment + ats) / 2);

  return (
    <section className="rounded-md border border-black/10 p-4 dark:border-white/15">
      <div className="flex items-center justify-between gap-4">
        <div>
          <p className="text-sm text-black/60 dark:text-white/60">Tailoring Score</p>
          <p className="text-3xl font-semibold">{overall}/100</p>
        </div>
        <div className="grid grid-cols-3 gap-4 text-right text-sm">
          <ScoreMetric label="Alignment" value={alignment} />
          <ScoreMetric label="ATS" value={ats} />
          <ScoreMetric label="Keywords" value={Math.round((run.keyword_coverage?.after ?? 0) * 100)} />
        </div>
      </div>
      <p className="mt-3 text-xs text-black/60 dark:text-white/60">
        Combines job alignment and ATS keyword coverage. Review every suggestion before downloading.
      </p>
    </section>
  );
}

function ScoreMetric({ label, value }: { label: string; value: number }) {
  return <div><p className="text-black/60 dark:text-white/60">{label}</p><p className="text-lg font-semibold">{value}%</p></div>;
}

function ResumeEditor({ content, onChange }: { content: ResumeVersionContent; onChange: (content: ResumeVersionContent) => void }) {
  function updateExperience(index: number, field: "Company" | "Title" | "Location" | "StartDate" | "EndDate", value: string) {
    const experiences = content.Experiences.map((experience, experienceIndex) =>
      experienceIndex === index ? { ...experience, [field]: value || null } : experience,
    );
    onChange({ ...content, Experiences: experiences });
  }

  function updateBullet(experienceIndex: number, bulletIndex: number, value: string) {
    const experiences = content.Experiences.map((experience, index) =>
      index === experienceIndex
        ? { ...experience, Bullets: experience.Bullets.map((bullet, currentIndex) => currentIndex === bulletIndex ? value : bullet) }
        : experience,
    );
    onChange({ ...content, Experiences: experiences });
  }

  return (
    <div className="flex flex-col gap-4 rounded-md border border-black/10 p-4 dark:border-white/15">
      <label className="flex flex-col gap-1 text-sm"><span className="font-medium">Summary</span><textarea value={content.Summary ?? ""} onChange={(e) => onChange({ ...content, Summary: e.target.value || null })} rows={4} className="rounded-md border border-black/10 p-2 dark:border-white/15" /></label>
      <label className="flex flex-col gap-1 text-sm"><span className="font-medium">Skills (one per line)</span><textarea value={content.Skills.join("\n")} onChange={(e) => onChange({ ...content, Skills: e.target.value.split("\n").map((item) => item.trim()).filter(Boolean) })} rows={4} className="rounded-md border border-black/10 p-2 dark:border-white/15" /></label>
      {content.Experiences.map((experience, experienceIndex) => (
        <fieldset key={experienceIndex} className="flex flex-col gap-2 rounded-md border border-black/10 p-3 dark:border-white/15">
          <legend className="px-1 text-sm font-medium">Experience {experienceIndex + 1}</legend>
          {(["Title", "Company", "Location", "StartDate", "EndDate"] as const).map((field) => (
            <input key={field} value={experience[field] ?? ""} placeholder={field} onChange={(e) => updateExperience(experienceIndex, field, e.target.value)} className="rounded-md border border-black/10 p-2 text-sm dark:border-white/15" />
          ))}
          {experience.Bullets.map((bullet, bulletIndex) => <textarea key={bulletIndex} value={bullet} onChange={(e) => updateBullet(experienceIndex, bulletIndex, e.target.value)} rows={3} className="rounded-md border border-black/10 p-2 text-sm dark:border-white/15" />)}
        </fieldset>
      ))}
      <p className="text-xs text-black/60 dark:text-white/60">Edit the content, then click Generate PDF/DOCX to create a new downloadable version.</p>
    </div>
  );
}

function ResumePreview({ content }: { content: ResumeVersion["Content"] }) {
  const normalized = normalizeResumeContent(content);
  const contact = normalized.Contact;

  return (
    <article className="max-h-[70vh] overflow-y-auto rounded-md border border-black/10 bg-white p-6 text-black shadow-sm dark:border-white/15 dark:bg-white dark:text-black">
      <header className="border-b border-black/15 pb-4 text-center">
        {contact.Name && <h2 className="text-2xl font-bold">{contact.Name}</h2>}
        <p className="mt-1 text-xs text-black/65">
          {[contact.Email, contact.Phone, contact.Location].filter(Boolean).join(" | ")}
        </p>
      </header>

      {normalized.Summary && <PreviewSection title="Summary"><p>{normalized.Summary}</p></PreviewSection>}
      {normalized.Skills.length > 0 && (
        <PreviewSection title="Skills">
          <p>{normalized.Skills.join(" • ")}</p>
        </PreviewSection>
      )}
      {normalized.Experiences.length > 0 && (
        <PreviewSection title="Experience">
          {normalized.Experiences.map((experience, index) => (
            <div key={`${experience.Company}-${experience.Title}-${index}`} className="mb-4 last:mb-0">
              <div className="flex justify-between gap-4 font-semibold">
                <span>{[experience.Title, experience.Company].filter(Boolean).join(", ")}</span>
                <span className="shrink-0 text-xs font-normal">
                  {[experience.StartDate, experience.EndDate].filter(Boolean).join(" - ")}
                </span>
              </div>
              {experience.Location && <p className="text-xs text-black/65">{experience.Location}</p>}
              <ul className="mt-1 list-inside list-disc">
                {experience.Bullets.map((bullet, bulletIndex) => <li key={bulletIndex}>{bullet}</li>)}
              </ul>
            </div>
          ))}
        </PreviewSection>
      )}
      {normalized.Education.length > 0 && <PreviewSection title="Education"><ul className="list-inside list-disc">{normalized.Education.map((item) => <li key={item}>{item}</li>)}</ul></PreviewSection>}
      {normalized.Certifications.length > 0 && <PreviewSection title="Certifications"><ul className="list-inside list-disc">{normalized.Certifications.map((item) => <li key={item}>{item}</li>)}</ul></PreviewSection>}
    </article>
  );
}

function normalizeResumeContent(content: ResumeVersionContent | null | undefined): ResumeVersionContent {
  const raw = asRecord(content);
  const rawContact = asRecord(raw.Contact ?? raw.contact);
  const rawExperiences = raw.Experiences ?? raw.experiences;

  return {
    Contact: {
      Name: stringValue(rawContact.Name ?? rawContact.name),
      Email: stringValue(rawContact.Email ?? rawContact.email),
      Phone: stringValue(rawContact.Phone ?? rawContact.phone),
      Location: stringValue(rawContact.Location ?? rawContact.location),
    },
    Summary: stringValue(raw.Summary ?? raw.summary),
    Skills: stringArray(raw.Skills ?? raw.skills),
    Experiences: Array.isArray(rawExperiences)
      ? rawExperiences.map((experience) => {
          const item = asRecord(experience);
          return {
            Company: stringValue(item.Company ?? item.company),
            Title: stringValue(item.Title ?? item.title),
            StartDate: stringValue(item.StartDate ?? item.start_date),
            EndDate: stringValue(item.EndDate ?? item.end_date),
            Location: stringValue(item.Location ?? item.location),
            Bullets: stringArray(item.Bullets ?? item.bullets),
          };
        })
      : [],
    Education: stringArray(raw.Education ?? raw.education),
    Certifications: stringArray(raw.Certifications ?? raw.certifications),
  };
}

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value) ? (value as Record<string, unknown>) : {};
}

function stringValue(value: unknown): string | null {
  return typeof value === "string" && value.trim() ? value : null;
}

function stringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === "string" && item.trim().length > 0) : [];
}

function serializeResumeContent(content: ResumeVersionContent) {
  return {
    contact: {
      name: content.Contact.Name,
      email: content.Contact.Email,
      phone: content.Contact.Phone,
      location: content.Contact.Location,
    },
    summary: content.Summary,
    skills: content.Skills,
    experiences: content.Experiences.map((experience) => ({
      company: experience.Company,
      title: experience.Title,
      start_date: experience.StartDate,
      end_date: experience.EndDate,
      location: experience.Location,
      bullets: experience.Bullets,
      detected_skills: [],
      technologies: [],
    })),
    education: content.Education,
    certifications: content.Certifications,
  };
}

function PreviewSection({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="mt-5 first:mt-0">
      <h3 className="mb-2 border-b border-black/15 pb-1 text-sm font-bold uppercase tracking-wide">{title}</h3>
      <div className="text-sm leading-relaxed">{children}</div>
    </section>
  );
}

function SuggestionCard({
  suggestion,
  onApprove,
  onReject,
}: {
  suggestion: TailoringSuggestion;
  onApprove: () => void;
  onReject: () => void;
}) {
  return (
    <div className="flex flex-col gap-3 rounded-md border border-black/10 p-4 dark:border-white/15">
      <div className="flex items-center justify-between">
        <span className="text-xs font-semibold uppercase tracking-wide text-black/60 dark:text-white/60">
          {suggestion.Section}
        </span>
        <div className="flex items-center gap-2">
          {suggestion.Source === "AI_SUGGESTED" && (
            <span className="rounded-full bg-purple-100 px-2 py-0.5 text-xs font-medium text-purple-800">
              AI Suggested
            </span>
          )}
          <span
            className={`rounded-full px-2 py-0.5 text-xs font-medium ${
              suggestion.UserStatus === "APPROVED"
                ? "bg-green-100 text-green-800"
                : suggestion.UserStatus === "REJECTED"
                  ? "bg-red-100 text-red-800"
                  : "bg-black/5 dark:bg-white/10"
            }`}
          >
            {suggestion.UserStatus}
          </span>
        </div>
      </div>

      {suggestion.OriginalText && (
        <div>
          <p className="text-xs font-medium text-black/50 dark:text-white/50">ORIGINAL</p>
          <p className="text-sm">{suggestion.OriginalText}</p>
        </div>
      )}
      <div>
        <p className="text-xs font-medium text-black/50 dark:text-white/50">SUGGESTED</p>
        <p className="text-sm">{suggestion.SuggestedText}</p>
      </div>
      <p className="text-xs text-black/60 dark:text-white/60">Why: {suggestion.Reason}</p>

      {suggestion.SkillsAdded.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {suggestion.SkillsAdded.map((skill) => (
            <QuickPrepDrawer key={skill} skill={skill} />
          ))}
        </div>
      )}

      {suggestion.Section === "experience" && (
        <div>
          <DefendBulletDrawer bulletText={suggestion.SuggestedText} skills={suggestion.SkillsAdded} />
        </div>
      )}

      {suggestion.UserStatus === "PENDING" && (
        <div className="flex gap-2">
          <button onClick={onApprove} className="rounded-md bg-foreground px-3 py-1.5 text-sm text-background">
            Approve
          </button>
          <button onClick={onReject} className="rounded-md border border-black/10 px-3 py-1.5 text-sm dark:border-white/15">
            Reject
          </button>
        </div>
      )}
    </div>
  );
}
