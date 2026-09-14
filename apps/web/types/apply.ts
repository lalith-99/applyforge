import type { ResumeVersion } from "@/types/api";

export interface ApplicationPackage {
  id: string;
  application_id: string;
  job_id: string;
  resume_version_id: string;
  destination_url: string;
  destination_origin: string;
  adapter: string;
  form_version: string;
  answers_json: Record<string, unknown>;
  required_fields: unknown[];
  evidence_refs: unknown[];
  resume_content_hash: string;
  answers_hash: string;
  package_hash: string;
  created_at: string;
}

export interface ApplicationApproval {
  id: string;
  package_id: string;
  package_hash: string;
  action_scope: "SUBMIT_ONCE";
  confirmation_version: string;
  confirmation_text: string;
  approved_at: string;
  expires_at: string;
  revoked_at?: string | null;
}

export type SubmissionIntentStatus =
  | "PENDING"
  | "CLAIMED"
  | "SUBMITTING"
  | "CONFIRMED"
  | "UNCERTAIN"
  | "FAILED"
  | "CANCELLED";

export interface SubmissionIntent {
  id: string;
  application_id: string;
  package_id: string;
  approval_id: string;
  package_hash: string;
  idempotency_key: string;
  status: SubmissionIntentStatus;
  lease_generation: number;
  lease_owner?: string | null;
  lease_expires_at?: string | null;
  attempt_count: number;
  last_error?: string | null;
  created_at: string;
  updated_at: string;
  completed_at?: string | null;
}

export interface CompanionHandoff {
  intent_id: string;
  package_id: string;
  token: string;
  expires_at: string;
}

export interface ApplyReviewData {
  package: ApplicationPackage;
  resume: ResumeVersion;
}
