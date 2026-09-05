"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import type { QuickPrepModule } from "@/types/api";

/**
 * "Learn First" trigger + drawer for a single skill's Quick Prep module
 * (MASTER_REQUIREMENTS.md §31). Self-contained: renders its own trigger
 * button and manages its own open/closed state.
 */
export function QuickPrepDrawer({ skill }: { skill: string }) {
  const [open, setOpen] = useState(false);

  const query = useQuery({
    queryKey: ["quick-prep", skill],
    queryFn: () => api.get<QuickPrepModule>(`/skills/${encodeURIComponent(skill)}/quick-prep`),
    enabled: open,
  });

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="rounded-full bg-blue-100 px-2 py-0.5 text-xs font-medium text-blue-800"
      >
        Learn First: {skill}
      </button>

      {open && (
        <div className="fixed inset-0 z-50 flex justify-end bg-black/40" onClick={() => setOpen(false)}>
          <div
            className="flex h-full w-full max-w-md flex-col gap-4 overflow-y-auto bg-background p-6 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between">
              <h2 className="text-lg font-semibold">Quick Prep: {skill}</h2>
              <button type="button" onClick={() => setOpen(false)} className="text-sm text-black/50 dark:text-white/50">
                Close
              </button>
            </div>

            {query.isLoading && <p className="text-sm text-black/60 dark:text-white/60">Loading…</p>}
            {query.isError && (
              <p className="text-sm text-red-600">
                {query.error instanceof ApiError ? query.error.message : "Could not load Quick Prep."}
              </p>
            )}

            {query.data && (
              <div className="flex flex-col gap-5 text-sm">
                <section>
                  <h3 className="text-xs font-semibold uppercase tracking-wide text-black/50 dark:text-white/50">
                    What it is
                  </h3>
                  <p>{query.data.what_it_is}</p>
                </section>

                <section>
                  <h3 className="text-xs font-semibold uppercase tracking-wide text-black/50 dark:text-white/50">
                    Why it matters
                  </h3>
                  <p>{query.data.why_it_matters}</p>
                </section>

                {query.data.transferable_from && query.data.transferable_from.length > 0 && (
                  <section>
                    <h3 className="text-xs font-semibold uppercase tracking-wide text-black/50 dark:text-white/50">
                      What you already know that transfers
                    </h3>
                    <ul className="list-inside list-disc">
                      {query.data.transferable_from.map((s) => (
                        <li key={s}>{s}</li>
                      ))}
                    </ul>
                  </section>
                )}

                {query.data.core_concepts.length > 0 && (
                  <section>
                    <h3 className="text-xs font-semibold uppercase tracking-wide text-black/50 dark:text-white/50">
                      Core concepts
                    </h3>
                    <ul className="list-inside list-disc">
                      {query.data.core_concepts.map((c) => (
                        <li key={c}>{c}</li>
                      ))}
                    </ul>
                  </section>
                )}

                {query.data.screening_points.length > 0 && (
                  <section>
                    <h3 className="text-xs font-semibold uppercase tracking-wide text-black/50 dark:text-white/50">
                      What screeners look for
                    </h3>
                    <ul className="list-inside list-disc">
                      {query.data.screening_points.map((p) => (
                        <li key={p}>{p}</li>
                      ))}
                    </ul>
                  </section>
                )}

                {query.data.interview_questions.length > 0 && (
                  <section className="flex flex-col gap-3">
                    <h3 className="text-xs font-semibold uppercase tracking-wide text-black/50 dark:text-white/50">
                      Interview questions
                    </h3>
                    {query.data.interview_questions.map((q) => (
                      <div key={q.question} className="rounded-md border border-black/10 p-3 dark:border-white/15">
                        <p className="font-medium">{q.question}</p>
                        <p className="mt-1 text-black/70 dark:text-white/70">{q.concise_answer}</p>
                        <p className="mt-1 text-xs text-black/50 dark:text-white/50">{q.deeper_explanation}</p>
                      </div>
                    ))}
                  </section>
                )}

                {query.data.common_mistakes.length > 0 && (
                  <section>
                    <h3 className="text-xs font-semibold uppercase tracking-wide text-black/50 dark:text-white/50">
                      Common mistakes
                    </h3>
                    <ul className="list-inside list-disc">
                      {query.data.common_mistakes.map((m) => (
                        <li key={m}>{m}</li>
                      ))}
                    </ul>
                  </section>
                )}

                {query.data.architecture_questions.length > 0 && (
                  <section>
                    <h3 className="text-xs font-semibold uppercase tracking-wide text-black/50 dark:text-white/50">
                      Architecture / system-design questions
                    </h3>
                    <ul className="list-inside list-disc">
                      {query.data.architecture_questions.map((q) => (
                        <li key={q}>{q}</li>
                      ))}
                    </ul>
                  </section>
                )}

                {query.data.example_code && (
                  <section>
                    <h3 className="text-xs font-semibold uppercase tracking-wide text-black/50 dark:text-white/50">
                      Example
                    </h3>
                    <pre className="overflow-x-auto rounded-md bg-black/5 p-3 text-xs dark:bg-white/10">
                      {query.data.example_code}
                    </pre>
                  </section>
                )}
              </div>
            )}
          </div>
        </div>
      )}
    </>
  );
}
