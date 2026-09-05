import { z } from "zod";

const csvList = z
  .string()
  .optional()
  .transform((value) =>
    (value ?? "")
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean),
  );

export const personalCareerSchema = z.object({
  first_name: z.string().min(1, "First name is required"),
  last_name: z.string().min(1, "Last name is required"),
  city: z.string().optional(),
  state: z.string().optional(),
  country: z.string().optional(),
  primary_target_titles: csvList,
  alternative_target_titles: csvList,
  seniority: z.string().optional(),
  years_experience: z.coerce.number().int().min(0).max(60).optional(),
  preferred_industries: csvList,
  preferred_technologies: csvList,
  desired_compensation_min: z.coerce.number().int().min(0).optional(),
  desired_compensation_max: z.coerce.number().int().min(0).optional(),
  desired_compensation_currency: z.string().default("USD"),
});

export type PersonalCareerInput = z.input<typeof personalCareerSchema>;
export type PersonalCareerOutput = z.output<typeof personalCareerSchema>;

export const jobPreferencesSchema = z.object({
  remote: z.boolean().default(false),
  hybrid: z.boolean().default(false),
  onsite: z.boolean().default(false),
  preferred_locations: csvList,
  willingness_to_relocate: z.boolean().default(false),
  employment_types: z.array(z.string()).default([]),
  minimum_salary: z.coerce.number().int().min(0).optional(),
  excluded_companies: csvList,
  excluded_locations: csvList,
  excluded_industries: csvList,
  clearance_constraints: z.string().optional(),
  work_authorization: z.string().optional(),
  immigration_status: z.string().optional(),
  requires_h1b_transfer: z.boolean().default(false),
  requires_new_h1b_cap_sponsorship: z.boolean().default(false),
  requires_future_employment_sponsorship: z.boolean().default(false),
  green_card_support_preferred: z.boolean().default(false),
  green_card_support_required: z.boolean().default(false),
  perm_support_preferred: z.boolean().default(false),
  immigration_support_min_confidence: z.string().optional(),
});

export type JobPreferencesInput = z.input<typeof jobPreferencesSchema>;
export type JobPreferencesOutput = z.output<typeof jobPreferencesSchema>;
