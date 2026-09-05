"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useRef, useState } from "react";
import { AppNav } from "@/components/AppNav";
import { api, ApiError } from "@/lib/api";
import type { ResumeDetail, ResumeSummary } from "@/types/api";

export default function ResumePage() {
  const queryClient = useQueryClient();
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const resumesQuery = useQuery({
    queryKey: ["resumes"],
    queryFn: () => api.get<ResumeSummary[]>("/resumes"),
    refetchInterval: (query) =>
      query.state.data?.some((r) => r.status === "UPLOADED" || r.status === "PARSING") ? 2000 : false,
  });

  const detailQuery = useQuery({
    queryKey: ["resumes", selectedId],
    queryFn: () => api.get<ResumeDetail>(`/resumes/${selectedId}`),
    enabled: !!selectedId,
    refetchInterval: (query) =>
      query.state.data && (query.state.data.status === "UPLOADED" || query.state.data.status === "PARSING")
        ? 2000
        : false,
  });

  const uploadMutation = useMutation({
    mutationFn: (file: File) => api.upload<ResumeSummary>("/resumes", file),
    onSuccess: (resume) => {
      queryClient.invalidateQueries({ queryKey: ["resumes"] });
      setSelectedId(resume.id);
    },
  });

  return (
    <>
      <AppNav />
      <main className="mx-auto flex w-full max-w-4xl flex-1 flex-col gap-8 p-8">
      <div className="flex items-center justify-between">
        <h1 className="text-2xl font-semibold">Master Resume</h1>
        <div className="flex flex-col items-end gap-1">
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            disabled={uploadMutation.isPending}
            className="rounded-md bg-foreground px-4 py-2 text-sm font-medium text-background disabled:opacity-60"
          >
            {uploadMutation.isPending ? "Uploading…" : "Upload resume (PDF or DOCX)"}
          </button>
          <input
            ref={fileInputRef}
            type="file"
            accept=".pdf,.docx"
            className="hidden"
            onChange={(e) => {
              const file = e.target.files?.[0];
              if (file) uploadMutation.mutate(file);
              e.target.value = "";
            }}
          />
          {uploadMutation.isError && (
            <p className="text-xs text-red-600">
              {uploadMutation.error instanceof ApiError ? uploadMutation.error.message : "Upload failed"}
            </p>
          )}
        </div>
      </div>

      <section className="flex flex-col gap-2">
        {resumesQuery.data?.length === 0 && (
          <p className="text-sm text-black/60 dark:text-white/60">No resumes uploaded yet.</p>
        )}
        {resumesQuery.data?.map((r) => (
          <button
            key={r.id}
            onClick={() => setSelectedId(r.id)}
            className={`flex items-center justify-between rounded-md border px-4 py-3 text-left text-sm ${
              selectedId === r.id ? "border-black/30 dark:border-white/30" : "border-black/10 dark:border-white/15"
            }`}
          >
            <span>{r.original_filename}</span>
            <StatusBadge status={r.status} />
          </button>
        ))}
      </section>

      {selectedId && detailQuery.data && <ResumeReview resume={detailQuery.data} />}
      </main>
    </>
  );
}

function StatusBadge({ status }: { status: ResumeSummary["status"] }) {
  const styles: Record<ResumeSummary["status"], string> = {
    UPLOADED: "bg-blue-100 text-blue-800",
    PARSING: "bg-amber-100 text-amber-800",
    PARSED: "bg-green-100 text-green-800",
    FAILED: "bg-red-100 text-red-800",
  };
  return <span className={`rounded-full px-2 py-0.5 text-xs font-medium ${styles[status]}`}>{status}</span>;
}

function ResumeReview({ resume }: { resume: ResumeDetail }) {
  if (resume.status === "FAILED") {
    return (
      <section className="rounded-md border border-red-200 p-4 text-sm text-red-700">
        Parsing failed: {resume.parse_error}
      </section>
    );
  }

  if (!resume.parsed_profile) {
    return <p className="text-sm text-black/60 dark:text-white/60">Parsing in progress…</p>;
  }

  const { parsed_profile: profile } = resume;

  return (
    <section className="flex flex-col gap-6 rounded-md border border-black/10 p-6 dark:border-white/15">
      <div>
        <h2 className="text-lg font-medium">{profile.contact.name ?? "Candidate"}</h2>
        <p className="text-sm text-black/60 dark:text-white/60">
          {profile.contact.email} {profile.contact.phone ? `· ${profile.contact.phone}` : ""}
        </p>
      </div>

      {profile.summary && <p className="text-sm">{profile.summary}</p>}

      <div>
        <h3 className="mb-2 text-sm font-semibold uppercase tracking-wide text-black/60 dark:text-white/60">
          Skills
        </h3>
        <div className="flex flex-wrap gap-2">
          {profile.skills.map((skill) => (
            <span key={skill} className="rounded-full bg-black/5 px-3 py-1 text-xs dark:bg-white/10">
              {skill}
            </span>
          ))}
        </div>
      </div>

      <div>
        <h3 className="mb-2 text-sm font-semibold uppercase tracking-wide text-black/60 dark:text-white/60">
          Experience
        </h3>
        <div className="flex flex-col gap-4">
          {profile.experiences.map((exp, i) => (
            <div key={i}>
              <p className="text-sm font-medium">
                {exp.title} {exp.company ? `— ${exp.company}` : ""}
              </p>
              <p className="text-xs text-black/60 dark:text-white/60">
                {exp.start_date} – {exp.end_date}
              </p>
              <ul className="mt-1 list-inside list-disc text-sm">
                {exp.bullets.map((b, j) => (
                  <li key={j}>{b}</li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </div>

      {profile.education.length > 0 && (
        <div>
          <h3 className="mb-2 text-sm font-semibold uppercase tracking-wide text-black/60 dark:text-white/60">
            Education
          </h3>
          <ul className="list-inside list-disc text-sm">
            {profile.education.map((e, i) => (
              <li key={i}>{e}</li>
            ))}
          </ul>
        </div>
      )}
    </section>
  );
}
