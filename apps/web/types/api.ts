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

export interface TailoringSuggestion {
  ID: string;
  TailoringRunID: string;
  Section: string;
  OriginalText: string | null;
  SuggestedText: string;
  RequirementsAddressed: string[];
  SkillsAdded: string[];
  KeywordsAdded: string[];
  Source: "MASTER_RESUME" | "AI_SUGGESTED";
  Reason: string;
  Confidence: number;
  RiskLevel: "LOW" | "MEDIUM" | "HIGH";
  UserStatus: "PENDING" | "APPROVED" | "EDITED" | "REJECTED";
  EditedText: string | null;
}

export interface TailoringRun {
  id: string;
  job_id: string;
  resume_id: string;
  mode: "STRICT" | "GROWTH" | "MAX_MATCH";
  status: string;
  alignment_score_before: number | null;
  alignment_score_after: number | null;
  created_at: string;
  completed_at: string | null;
  suggestions: TailoringSuggestion[] | null;
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

export interface InterviewQuestion {
  question: string;
  concise_answer: string;
  deeper_explanation: string;
}

export interface QuickPrepModule {
  skill: string;
  what_it_is: string;
  why_it_matters: string;
  transferable_from: string[] | null;
  core_concepts: string[];
  screening_points: string[];
  interview_questions: InterviewQuestion[];
  common_mistakes: string[];
  architecture_questions: string[];
  example_code: string | null;
}

export interface DefendBulletResponse {
  questions: InterviewQuestion[];
}

export interface LearningPlanResult {
  JobID: string;
  Skills: string[];
  CurrentReadiness: number;
  TargetReadiness: number;
  Topics: string[];
  PracticeQuestions: InterviewQuestion[];
  Projects: string[];
  ArchitectureQuestions: string[];
  EstimatedEffortCategory: "QUICK_PREP" | "STANDARD_PREP" | "DEEPER_GAP";
}

export interface ReadinessComponents {
  CoreLanguage: number;
  BackendFundamentals: number;
  RequiredTechnology: number;
  SystemDesignDomain: number;
  ExperienceExamples: number;
  QuestionPreparedness: number;
}

export interface QualifiedResult {
  CurrentProfileMatch: number;
  TargetProfileMatch: number;
  HighValueGaps: string[] | null;
  LowValueGaps: string[] | null;
  RecommendedSkills: string[] | null;
  InterviewReadiness: number;
  ReadinessComponents: ReadinessComponents;
  LearningPlan: LearningPlanResult;
}

export interface ResumeVersion {
  ID: string;
  UserID: string;
  BaseResumeID: string;
  JobID: string | null;
  TailoringRunID: string | null;
  VersionNumber: number;
  MatchScore: number | null;
  AlignmentScore: number | null;
  TailoringMode: string | null;
  Content: ResumeVersionContent;
  PDFStorageKey: string | null;
  DocxStorageKey: string | null;
  CreatedAt: string;
}

export interface ResumeVersionContent {
  Contact: {
    Name: string | null;
    Email: string | null;
    Phone: string | null;
    Location: string | null;
  };
  Summary: string | null;
  Skills: string[];
  Experiences: {
    Company: string | null;
    Title: string | null;
    StartDate: string | null;
    EndDate: string | null;
    Location: string | null;
    Bullets: string[];
  }[];
  Education: string[];
  Certifications: string[];
}

export type ApplicationStatus =
  | "SAVED"
  | "READY_TO_APPLY"
  | "APPLIED"
  | "RECRUITER_SCREEN"
  | "ASSESSMENT"
  | "TECHNICAL_INTERVIEW"
  | "FINAL_INTERVIEW"
  | "OFFER"
  | "REJECTED"
  | "WITHDRAWN";

export interface Application {
  ID: string;
  UserID: string;
  JobID: string;
  ResumeVersionID: string | null;
  Status: ApplicationStatus;
  MatchScore: number | null;
  Notes: string | null;
  NextAction: string | null;
  AppliedAt: string | null;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface ApplicationWithJob extends Application {
  CompanyName: string;
  Title: string;
  NormalizedTitle: string;
  Source: string;
  FirstSeenAt: string;
  PostedAt: string | null;
}

export interface ApplicationEvent {
  ID: string;
  ApplicationID: string;
  EventType: string;
  FromStatus: string | null;
  ToStatus: string | null;
  Notes: string | null;
  CreatedAt: string;
}

export interface ApplicationAnswers {
  UserID: string;
  FullName: string | null;
  Phone: string | null;
  Email: string | null;
  Location: string | null;
  DesiredLocation: string | null;
  WorkAuthorization: string | null;
  Sponsorship: string | null;
  SalaryExpectation: string | null;
  NoticePeriod: string | null;
  LinkedinURL: string | null;
  GithubURL: string | null;
  PortfolioURL: string | null;
  CommonAnswers: Record<string, unknown>;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface StatusCount {
  Status: ApplicationStatus;
  Count: number;
}

export interface FunnelStage {
  Status: ApplicationStatus;
  Count: number;
}

export interface AnalyticsDashboard {
  JobsDiscovered: number;
  TotalApplications: number;
  TailoringRunsCount: number;
  HighMatchesCount: number;
  ApplicationsByStatus: StatusCount[] | null;
  Funnel: FunnelStage[];
  ResponseRatePercent: number;
  AverageMatchScore: number | null;
}
