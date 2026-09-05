export interface User {
  id: string;
  email: string;
  email_verified_at: string | null;
  has_password: boolean;
  has_google: boolean;
  created_at: string;
}

export interface Profile {
  user_id: string;
  first_name: string | null;
  last_name: string | null;
  city: string | null;
  state: string | null;
  country: string | null;
  primary_target_titles: string[];
  alternative_target_titles: string[];
  seniority: string | null;
  years_experience: number | null;
  preferred_industries: string[];
  preferred_technologies: string[];
  desired_compensation_min: number | null;
  desired_compensation_max: number | null;
  desired_compensation_currency: string;
  onboarding_completed_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface JobPreferences {
  user_id: string;
  remote: boolean;
  hybrid: boolean;
  onsite: boolean;
  preferred_locations: string[];
  willingness_to_relocate: boolean;
  employment_types: string[];
  minimum_salary: number | null;
  excluded_companies: string[];
  excluded_locations: string[];
  excluded_industries: string[];
  clearance_constraints: string | null;
  work_authorization: string | null;
  immigration_status: string | null;
  requires_h1b_transfer: boolean;
  requires_new_h1b_cap_sponsorship: boolean;
  requires_future_employment_sponsorship: boolean;
  green_card_support_preferred: boolean;
  green_card_support_required: boolean;
  perm_support_preferred: boolean;
  immigration_support_min_confidence: string | null;
  created_at: string;
  updated_at: string;
}
