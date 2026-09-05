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

export interface ResumeSummary {
  id: string;
  original_filename: string;
  mime_type: string;
  size_bytes: number;
  status: "UPLOADED" | "PARSING" | "PARSED" | "FAILED";
  parse_error: string | null;
  parsed_at: string | null;
  created_at: string;
  updated_at: string;
}

export interface ResumeExperience {
  Company: string | null;
  Title: string | null;
  StartDate: string | null;
  EndDate: string | null;
  Location: string | null;
  Bullets: string[];
  DetectedSkills: string[];
  Technologies: string[];
}

export interface ResumeParsedProfile {
  contact: { name: string | null; email: string | null; phone: string | null; location: string | null };
  summary: string | null;
  skills: string[];
  experiences: {
    company: string | null;
    title: string | null;
    start_date: string | null;
    end_date: string | null;
    bullets: string[];
    detected_skills: string[];
  }[];
  education: string[];
  certifications: string[];
}

export interface ResumeDetail extends ResumeSummary {
  parsed_profile: ResumeParsedProfile | null;
  experiences: ResumeExperience[];
}
