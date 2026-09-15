"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { AppNav } from "@/components/AppNav";
import { api } from "@/lib/api";

type ApplicationAnswers = {
  UserID?: string;
  FullName?: string | null;
  Phone?: string | null;
  Email?: string | null;
  Location?: string | null;
  DesiredLocation?: string | null;
  WorkAuthorization?: string | null;
  Sponsorship?: string | null;
  SalaryExpectation?: string | null;
  NoticePeriod?: string | null;
  LinkedinURL?: string | null;
  GithubURL?: string | null;
  PortfolioURL?: string | null;
  CommonAnswers?: Record<string, unknown> | null;
  UpdatedAt?: string;
};

type FormState = {
  full_name: string;
  phone: string;
  email: string;
  location: string;
  desired_location: string;
  work_authorization: string;
  sponsorship: string;
  salary_expectation: string;
  notice_period: string;
  linkedin_url: string;
  github_url: string;
  portfolio_url: string;
};

const EMPTY: FormState = {
  full_name: "",
  phone: "",
  email: "",
  location: "",
  desired_location: "",
  work_authorization: "",
  sponsorship: "",
  salary_expectation: "",
  notice_period: "",
  linkedin_url: "",
  github_url: "",
  portfolio_url: "",
};

const FIELDS: { key: keyof FormState; label: string; hint?: string; type?: string }[] = [
  { key: "full_name", label: "Full name" },
  { key: "email", label: "Email", type: "email" },
  { key: "phone", label: "Phone" },
  { key: "location", label: "Current location" },
  { key: "desired_location", label: "Desired / preferred location" },
  { key: "salary_expectation", label: "Salary expectation", hint: "Leave blank if you prefer to answer job-by-job." },
  { key: "notice_period", label: "Notice period / availability" },
  { key: "linkedin_url", label: "LinkedIn URL", type: "url" },
  { key: "github_url", label: "GitHub URL", type: "url" },
  { key: "portfolio_url", label: "Portfolio URL", type: "url" },
];

const BINARY_ANSWERS: { key: "work_authorization" | "sponsorship"; label: string; question: string; hint: string }[] = [
  {
    key: "work_authorization",
    label: "US work authorization",
    question: "Are you currently authorized to work in the United States?",
    hint: "Choose the answer ApplyForge may reuse for equivalent work-authorization questions.",
  },
  {
    key: "sponsorship",
    label: "Visa sponsorship / support",
    question: "Do you now, or will you in the future, require employment visa sponsorship or support, including a transfer or renewal?",
    hint: "Choose the answer ApplyForge may reuse only for equivalent sponsorship/support questions.",
  },
];

export default function ApplicationAnswersPage() {
  const queryClient = useQueryClient();
  const [form, setForm] = useState<FormState>(EMPTY);
  const [savedMessage, setSavedMessage] = useState<string | null>(null);

  const answersQuery = useQuery({
    queryKey: ["application-answers"],
    queryFn: () => api.get<ApplicationAnswers>("/application-answers"),
  });

  useEffect(() => {
    const data = answersQuery.data;
    if (!data) return;
    // Hydrate this editable draft only when the server query value changes.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setForm({
      full_name: data.FullName ?? "",
      phone: data.Phone ?? "",
      email: data.Email ?? "",
      location: data.Location ?? "",
      desired_location: data.DesiredLocation ?? "",
      work_authorization: normalizeBinarySavedAnswer(data.WorkAuthorization),
      sponsorship: normalizeBinarySavedAnswer(data.Sponsorship),
      salary_expectation: data.SalaryExpectation ?? "",
      notice_period: data.NoticePeriod ?? "",
      linkedin_url: data.LinkedinURL ?? "",
      github_url: data.GithubURL ?? "",
      portfolio_url: data.PortfolioURL ?? "",
    });
  }, [answersQuery.data]);

  const save = useMutation({
    mutationFn: () => api.patch<ApplicationAnswers>("/application-answers", {
      ...Object.fromEntries(Object.entries(form).map(([key, value]) => [key, value.trim() || null])),
      common_answers: answersQuery.data?.CommonAnswers ?? {},
    }),
    onSuccess: (data) => {
      queryClient.setQueryData(["application-answers"], data);
      setSavedMessage("Application answers saved. New application packages will snapshot these exact values.");
      window.setTimeout(() => setSavedMessage(null), 5000);
    },
  });

  return (
    <>
      <AppNav />
      <main className="mx-auto w-full max-w-4xl p-8">
        <div className="mb-6">
          <h1 className="text-2xl font-semibold">Application answers</h1>
          <p className="mt-2 text-sm text-black/60 dark:text-white/60">
            These reusable answers are copied into the immutable package you review before each application. ApplyForge does not invent missing answers; leave a field blank when you want to answer it manually on the employer site.
          </p>
        </div>

        {answersQuery.isLoading ? (
          <p className="text-sm text-black/60 dark:text-white/60">Loading…</p>
        ) : answersQuery.isError ? (
          <p className="rounded-md bg-red-50 p-3 text-sm text-red-700">Could not load application answers.</p>
        ) : (
          <form
            className="grid gap-5 rounded-xl border border-black/10 p-6 dark:border-white/15 md:grid-cols-2"
            onSubmit={(event) => {
              event.preventDefault();
              setSavedMessage(null);
              save.mutate();
            }}
          >
            {FIELDS.map((field) => (
              <label key={field.key} className="flex flex-col gap-1.5">
                <span className="text-sm font-medium">{field.label}</span>
                <input
                  type={field.type ?? "text"}
                  value={form[field.key]}
                  onChange={(event) => setForm((current) => ({ ...current, [field.key]: event.target.value }))}
                  className="rounded-md border border-black/15 bg-transparent px-3 py-2 text-sm outline-none focus:border-black/40 dark:border-white/20 dark:focus:border-white/50"
                />
                {field.hint && <span className="text-xs text-black/50 dark:text-white/50">{field.hint}</span>}
              </label>
            ))}

            <div className="grid gap-5 border-t border-black/10 pt-5 md:col-span-2 md:grid-cols-2 dark:border-white/15">
              {BINARY_ANSWERS.map((field) => (
                <fieldset key={field.key} className="rounded-lg border border-black/10 p-4 dark:border-white/15">
                  <legend className="px-1 text-sm font-medium">{field.label}</legend>
                  <p className="mt-1 text-sm">{field.question}</p>
                  <div className="mt-3 flex flex-wrap gap-4">
                    {["yes", "no"].map((answer) => (
                      <label key={answer} className="flex items-center gap-2 text-sm">
                        <input
                          type="radio"
                          name={field.key}
                          value={answer}
                          checked={form[field.key] === answer}
                          onChange={(event) => setForm((current) => ({ ...current, [field.key]: event.target.value }))}
                        />
                        {answer === "yes" ? "Yes" : "No"}
                      </label>
                    ))}
                    <button
                      type="button"
                      className="text-xs underline underline-offset-2 opacity-70"
                      onClick={() => setForm((current) => ({ ...current, [field.key]: "" }))}
                    >
                      Clear / answer manually
                    </button>
                  </div>
                  <p className="mt-2 text-xs text-black/50 dark:text-white/50">{field.hint}</p>
                </fieldset>
              ))}
            </div>

            <div className="flex items-center gap-3 border-t border-black/10 pt-5 md:col-span-2 dark:border-white/15">
              <button type="submit" disabled={save.isPending} className="rounded-md bg-foreground px-4 py-2 text-sm font-medium text-background disabled:opacity-50">
                {save.isPending ? "Saving…" : "Save application answers"}
              </button>
              {savedMessage && <p className="text-sm text-green-700 dark:text-green-300">{savedMessage}</p>}
              {save.isError && <p className="text-sm text-red-700">{save.error instanceof Error ? save.error.message : "Could not save answers."}</p>}
            </div>
          </form>
        )}

        <section className="mt-6 rounded-lg bg-black/[0.03] p-4 text-sm text-black/65 dark:bg-white/[0.05] dark:text-white/65">
          <p className="font-medium text-black/80 dark:text-white/80">How this is used</p>
          <p className="mt-1">When you choose Review & apply, ApplyForge snapshots these values into that package. Editing this page later does not silently change an already-approved package; you must build and approve a new package for changed answers.</p>
        </section>
      </main>
    </>
  );
}

export function normalizeBinarySavedAnswer(value?: string | null): string {
  const normalized = String(value ?? "").trim().toLowerCase();
  if (["yes", "true", "y", "1"].includes(normalized)) return "yes";
  if (["no", "false", "n", "0"].includes(normalized)) return "no";
  return "";
}
