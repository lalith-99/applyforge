"use client";

import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQuery } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useForm } from "react-hook-form";
import { api, ApiError } from "@/lib/api";
import {
  jobPreferencesSchema,
  personalCareerSchema,
  type JobPreferencesInput,
  type JobPreferencesOutput,
  type PersonalCareerInput,
  type PersonalCareerOutput,
} from "@/lib/schemas/onboarding";
import type { JobPreferences, Profile } from "@/types/api";

const EMPLOYMENT_TYPES = [
  { value: "full_time", label: "Full-time" },
  { value: "contract", label: "Contract" },
  { value: "internship", label: "Internship" },
];

export default function OnboardingPage() {
  const router = useRouter();
  const [step, setStep] = useState<0 | 1>(0);
  const [profileData, setProfileData] = useState<PersonalCareerOutput | null>(null);

  const profileForm = useForm<PersonalCareerInput, unknown, PersonalCareerOutput>({
    resolver: zodResolver(personalCareerSchema),
    defaultValues: { desired_compensation_currency: "USD" },
  });

  const preferencesForm = useForm<JobPreferencesInput, unknown, JobPreferencesOutput>({
    resolver: zodResolver(jobPreferencesSchema),
    defaultValues: {
      remote: false,
      hybrid: false,
      onsite: false,
      willingness_to_relocate: false,
      employment_types: [],
      requires_h1b_transfer: false,
      requires_new_h1b_cap_sponsorship: false,
      requires_future_employment_sponsorship: false,
      green_card_support_preferred: false,
      green_card_support_required: false,
      perm_support_preferred: false,
    },
  });

  const profileQuery = useQuery({
    queryKey: ["profile"],
    queryFn: () => api.get<Profile>("/profile"),
  });

  const preferencesQuery = useQuery({
    queryKey: ["preferences"],
    queryFn: () => api.get<JobPreferences>("/preferences"),
  });

  useEffect(() => {
    const profile = profileQuery.data;
    if (!profile) return;
    if (profile.onboarding_completed_at) {
      router.replace("/dashboard");
      return;
    }
    profileForm.reset({
      first_name: profile.first_name ?? "",
      last_name: profile.last_name ?? "",
      city: profile.city ?? "",
      state: profile.state ?? "",
      country: profile.country ?? "",
      primary_target_titles: (profile.primary_target_titles ?? []).join(", "),
      alternative_target_titles: (profile.alternative_target_titles ?? []).join(", "),
      seniority: profile.seniority ?? "",
      years_experience: profile.years_experience ?? undefined,
      preferred_industries: (profile.preferred_industries ?? []).join(", "),
      preferred_technologies: (profile.preferred_technologies ?? []).join(", "),
      desired_compensation_min: profile.desired_compensation_min ?? undefined,
      desired_compensation_max: profile.desired_compensation_max ?? undefined,
      desired_compensation_currency: profile.desired_compensation_currency || "USD",
    });
  }, [profileQuery.data, profileForm, router]);

  useEffect(() => {
    const preferences = preferencesQuery.data;
    if (!preferences) return;
    preferencesForm.reset({
      remote: preferences.remote,
      hybrid: preferences.hybrid,
      onsite: preferences.onsite,
      preferred_locations: (preferences.preferred_locations ?? []).join(", "),
      willingness_to_relocate: preferences.willingness_to_relocate,
      employment_types: preferences.employment_types ?? [],
      minimum_salary: preferences.minimum_salary ?? undefined,
      excluded_companies: (preferences.excluded_companies ?? []).join(", "),
      excluded_locations: (preferences.excluded_locations ?? []).join(", "),
      excluded_industries: (preferences.excluded_industries ?? []).join(", "),
      clearance_constraints: preferences.clearance_constraints ?? "",
      work_authorization: preferences.work_authorization ?? "",
      immigration_status: preferences.immigration_status ?? "",
      requires_h1b_transfer: preferences.requires_h1b_transfer,
      requires_new_h1b_cap_sponsorship: preferences.requires_new_h1b_cap_sponsorship,
      requires_future_employment_sponsorship: preferences.requires_future_employment_sponsorship,
      green_card_support_preferred: preferences.green_card_support_preferred,
      green_card_support_required: preferences.green_card_support_required,
      perm_support_preferred: preferences.perm_support_preferred,
      immigration_support_min_confidence: preferences.immigration_support_min_confidence ?? undefined,
    });
  }, [preferencesQuery.data, preferencesForm]);

  const saveProfileMutation = useMutation({
    mutationFn: (profile: PersonalCareerOutput) =>
      api.patch<Profile>("/profile", { ...profile, complete_onboarding: false }),
    onSuccess: (_saved, profile) => {
      setProfileData(profile);
      setStep(1);
    },
  });

  const submitMutation = useMutation({
    mutationFn: async (preferences: JobPreferencesOutput) => {
      if (!profileData) throw new Error("missing profile data");
      // Persist preferences before flipping the completion marker. If this
      // request fails the user remains safely incomplete and can retry.
      await api.patch("/preferences", preferences);
      await api.patch("/profile", { ...profileData, complete_onboarding: true });
    },
    onSuccess: () => router.push("/dashboard"),
  });

  const isHydrating = profileQuery.isLoading || preferencesQuery.isLoading;
  const hydrationFailed = profileQuery.isError || preferencesQuery.isError;

  if (isHydrating) {
    return <main className="flex flex-1 items-center justify-center p-8">Loading your saved job-search profile…</main>;
  }

  if (hydrationFailed) {
    return <main className="flex flex-1 items-center justify-center p-8 text-red-600">Could not load your saved onboarding data. Please refresh and try again.</main>;
  }

  return (
    <main className="mx-auto flex w-full max-w-2xl flex-1 flex-col gap-8 p-8">
      <div className="flex flex-col gap-1">
        <h1 className="text-2xl font-semibold">Tell us about your job search</h1>
        <p className="text-sm text-black/60 dark:text-white/60">
          Step {step + 1} of 2 — {step === 0 ? "Personal & career" : "Job preferences"}
        </p>
      </div>

      {step === 0 && (
        <form
          onSubmit={profileForm.handleSubmit((data) => saveProfileMutation.mutate(data))}
          className="flex flex-col gap-4"
        >
          <div className="grid grid-cols-2 gap-4">
            <Field label="First name" error={profileForm.formState.errors.first_name?.message}>
              <input {...profileForm.register("first_name")} className={inputClass} />
            </Field>
            <Field label="Last name" error={profileForm.formState.errors.last_name?.message}>
              <input {...profileForm.register("last_name")} className={inputClass} />
            </Field>
          </div>

          <div className="grid grid-cols-3 gap-4">
            <Field label="City"><input {...profileForm.register("city")} className={inputClass} /></Field>
            <Field label="State"><input {...profileForm.register("state")} className={inputClass} /></Field>
            <Field label="Country"><input {...profileForm.register("country")} className={inputClass} /></Field>
          </div>

          <Field label="Primary target titles (comma separated)">
            <input {...profileForm.register("primary_target_titles")} className={inputClass} placeholder="Backend Engineer, Software Engineer" />
          </Field>
          <Field label="Alternative target titles (comma separated)">
            <input {...profileForm.register("alternative_target_titles")} className={inputClass} />
          </Field>

          <div className="grid grid-cols-2 gap-4">
            <Field label="Seniority">
              <input {...profileForm.register("seniority")} className={inputClass} placeholder="e.g. Senior" />
            </Field>
            <Field label="Years of experience">
              <input type="number" {...profileForm.register("years_experience")} className={inputClass} />
            </Field>
          </div>

          <Field label="Preferred industries (comma separated)">
            <input {...profileForm.register("preferred_industries")} className={inputClass} />
          </Field>
          <Field label="Preferred technologies (comma separated)">
            <input {...profileForm.register("preferred_technologies")} className={inputClass} />
          </Field>

          <div className="grid grid-cols-2 gap-4">
            <Field label="Desired compensation min">
              <input type="number" {...profileForm.register("desired_compensation_min")} className={inputClass} />
            </Field>
            <Field label="Desired compensation max">
              <input type="number" {...profileForm.register("desired_compensation_max")} className={inputClass} />
            </Field>
          </div>

          {saveProfileMutation.isError && (
            <p className="text-sm text-red-600">
              {saveProfileMutation.error instanceof ApiError
                ? saveProfileMutation.error.message
                : "Could not save your profile. Please try again."}
            </p>
          )}
          <button type="submit" disabled={saveProfileMutation.isPending} className={buttonClass}>
            {saveProfileMutation.isPending ? "Saving…" : "Continue"}
          </button>
        </form>
      )}

      {step === 1 && (
        <form
          onSubmit={preferencesForm.handleSubmit((data) => submitMutation.mutate(data))}
          className="flex flex-col gap-6"
        >
          <section className="flex flex-col gap-3">
            <h2 className="text-lg font-medium">Work arrangement</h2>
            <div className="flex gap-6">
              <Checkbox label="Remote" {...preferencesForm.register("remote")} />
              <Checkbox label="Hybrid" {...preferencesForm.register("hybrid")} />
              <Checkbox label="Onsite" {...preferencesForm.register("onsite")} />
              <Checkbox label="Willing to relocate" {...preferencesForm.register("willingness_to_relocate")} />
            </div>
            <Field label="Preferred locations (comma separated)">
              <input {...preferencesForm.register("preferred_locations")} className={inputClass} />
            </Field>
            <div className="flex gap-6">
              {EMPLOYMENT_TYPES.map((type) => (
                <label key={type.value} className="flex items-center gap-2 text-sm">
                  <input
                    type="checkbox"
                    value={type.value}
                    {...preferencesForm.register("employment_types")}
                  />
                  {type.label}
                </label>
              ))}
            </div>
            <Field label="Minimum salary">
              <input type="number" {...preferencesForm.register("minimum_salary")} className={inputClass} />
            </Field>
          </section>

          <section className="flex flex-col gap-3">
            <h2 className="text-lg font-medium">Restrictions</h2>
            <Field label="Excluded companies (comma separated)">
              <input {...preferencesForm.register("excluded_companies")} className={inputClass} />
            </Field>
            <Field label="Excluded locations (comma separated)">
              <input {...preferencesForm.register("excluded_locations")} className={inputClass} />
            </Field>
            <Field label="Excluded industries (comma separated)">
              <input {...preferencesForm.register("excluded_industries")} className={inputClass} />
            </Field>
            <Field label="Clearance constraints">
              <input {...preferencesForm.register("clearance_constraints")} className={inputClass} />
            </Field>
            <Field label="Work authorization">
              <input {...preferencesForm.register("work_authorization")} className={inputClass} placeholder="e.g. H-1B, GC-EAD, US Citizen" />
            </Field>
          </section>

          <section className="flex flex-col gap-3 rounded-md border border-black/10 p-4 dark:border-white/15">
            <h2 className="text-lg font-medium">Immigration preferences</h2>
            <p className="text-xs text-black/60 dark:text-white/60">
              We treat H-1B transfer, new H-1B sponsorship, and green-card/PERM support as
              separate signals — never a single yes/no.
            </p>
            <Field label="Current immigration status">
              <input {...preferencesForm.register("immigration_status")} className={inputClass} placeholder="e.g. H1B" />
            </Field>
            <Checkbox label="Requires H-1B transfer support" {...preferencesForm.register("requires_h1b_transfer")} />
            <Checkbox label="Requires new H-1B cap sponsorship" {...preferencesForm.register("requires_new_h1b_cap_sponsorship")} />
            <Checkbox label="Requires future employment-based sponsorship" {...preferencesForm.register("requires_future_employment_sponsorship")} />
            <Checkbox label="Prefer green-card sponsorship support" {...preferencesForm.register("green_card_support_preferred")} />
            <Checkbox label="Require green-card sponsorship support" {...preferencesForm.register("green_card_support_required")} />
            <Checkbox label="Prefer PERM sponsorship pathway" {...preferencesForm.register("perm_support_preferred")} />
          </section>

          {submitMutation.isError && (
            <p className="text-sm text-red-600">
              {submitMutation.error instanceof ApiError
                ? submitMutation.error.message
                : "Something went wrong. Please try again."}
            </p>
          )}

          <div className="flex gap-3">
            <button type="button" onClick={() => setStep(0)} className={secondaryButtonClass}>
              Back
            </button>
            <button type="submit" disabled={submitMutation.isPending} className={buttonClass}>
              {submitMutation.isPending ? "Saving…" : "Finish onboarding"}
            </button>
          </div>
        </form>
      )}
    </main>
  );
}

const inputClass =
  "rounded-md border border-black/10 bg-transparent px-3 py-2 text-sm outline-none focus:border-black/30 dark:border-white/15 dark:focus:border-white/30";
const buttonClass =
  "rounded-md bg-foreground px-4 py-2 text-sm font-medium text-background disabled:opacity-60";
const secondaryButtonClass =
  "rounded-md border border-black/10 px-4 py-2 text-sm font-medium dark:border-white/15";

function Field({
  label,
  error,
  children,
}: {
  label: string;
  error?: string;
  children: React.ReactNode;
}) {
  return (
    <label className="flex flex-col gap-1 text-sm">
      <span className="font-medium">{label}</span>
      {children}
      {error && <span className="text-xs text-red-600">{error}</span>}
    </label>
  );
}

function Checkbox({
  label,
  ...props
}: React.InputHTMLAttributes<HTMLInputElement> & { label: string }) {
  return (
    <label className="flex items-center gap-2 text-sm">
      <input type="checkbox" {...props} />
      {label}
    </label>
  );
}
