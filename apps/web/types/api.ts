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

export interface JobSummary {
  id: string;
  source: string;
  company_name: string;
  title: string;
  normalized_title: string;
  location_text: string | null;
  remote_type: string | null;
  employment_type: string | null;
  salary_min: number | null;
  salary_max: number | null;
  salary_currency: string | null;
  apply_url: string | null;
  posted_at: string | null;
  first_seen_at: string;
}

export interface JobDetail extends JobSummary {
  description: string;
  source_url: string | null;
  status: string;
  requirements?: {
    RoleFamily: string | null;
    NormalizedTitle: string | null;
    Seniority: string | null;
    RequiredSkills: { normalized_name: string; importance: string }[];
    PreferredSkills: { normalized_name: string; importance: string }[];
    RequiredExperienceYears: number | null;
    Responsibilities: string[];
    Keywords: string[];
    ClearanceRequirements: string | null;
    WorkAuthorizationRequirements: string | null;
  };
}

export interface JobsListResponse {
  items: JobSummary[];
  total: number;
  limit: number;
  offset: number;
}

export interface ComponentScores {
  MustHaveSkillCoverage: number;
  ResponsibilityAlignment: number;
  RoleSeniority: number;
  PreferredSkills: number;
  DomainAlignment: number;
  LocationWorkArrangement: number;
  EducationCertifications: number;
  CandidatePreferences: number;
}

export interface TransferableMatch {
  SourceSkill: string;
  TargetSkill: string;
  Level: string;
  PrepClassification: string;
}

export interface MatchResult {
  TotalScore: number;
  Grade: string;
  Components: ComponentScores;
  MatchedSkills: string[];
  TransferableSkills: TransferableMatch[] | null;
  MissingRequiredSkills: string[] | null;
  MissingPreferredSkills: string[] | null;
  PositiveEvidence: string[] | null;
  Concerns: string[] | null;
  Explanation: string;
  OpportunityScore: number;
  CurrentProfileMatch: number;
  TargetProfileMatch: number;
  SuggestedTargetAdditions: string[] | null;
  Eligibility: {
    Eligible: boolean;
    HardFailures: string[] | null;
    Warnings: string[] | null;
  };
}
