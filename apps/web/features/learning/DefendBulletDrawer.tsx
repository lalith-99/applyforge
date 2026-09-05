"use client";

import { useMutation } from "@tanstack/react-query";
import { useState } from "react";
import { api, ApiError } from "@/lib/api";
import type { DefendBulletResponse } from "@/types/api";

/**
 * "Defend This Bullet" trigger + drawer (MASTER_REQUIREMENTS.md §32).
 * Sends the bullet text and its associated skills directly, since the
 * caller already has both values on hand (tailoring suggestion or resume
 * experience row).
 */
export function DefendBulletDrawer({ bulletText, skills }: { bulletText: string; skills: string[] }) {
  const [open, setOpen] = useState(false);

  const mutation = useMutation({
    mutationFn: () => api.post<DefendBulletResponse>("/defend-bullet", { bullet_text: bulletText, skills }),
  });

  return (
    <>
      <button
        type="button"
        onClick={() => {
          setOpen(true);
          if (!mutation.data && !mutation.isPending) mutation.mutate();
        }}
        className="rounded-full border border-black/10 px-2 py-0.5 text-xs font-medium dark:border-white/15"
      >
        Defend This Bullet
      </button>

      {open && (
        <div className="fixed inset-0 z-50 flex justify-end bg-black/40" onClick={() => setOpen(false)}>
          <div
            className="flex h-full w-full max-w-md flex-col gap-4 overflow-y-auto bg-background p-6 shadow-xl"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between">
              <h2 className="text-lg font-semibold">Defend This Bullet</h2>
              <button type="button" onClick={() => setOpen(false)} className="text-sm text-black/50 dark:text-white/50">
                Close
              </button>
            </div>

            <p className="rounded-md border border-black/10 p-3 text-sm dark:border-white/15">{bulletText}</p>

            {mutation.isPending && <p className="text-sm text-black/60 dark:text-white/60">Loading…</p>}
            {mutation.isError && (
              <p className="text-sm text-red-600">
                {mutation.error instanceof ApiError ? mutation.error.message : "Could not load questions."}
              </p>
            )}

            {mutation.data && (
              <div className="flex flex-col gap-3">
                {mutation.data.questions.map((q) => (
                  <div key={q.question} className="rounded-md border border-black/10 p-3 text-sm dark:border-white/15">
                    <p className="font-medium">{q.question}</p>
                    <p className="mt-1 text-black/70 dark:text-white/70">{q.concise_answer}</p>
                    <p className="mt-1 text-xs text-black/50 dark:text-white/50">{q.deeper_explanation}</p>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </>
  );
}
