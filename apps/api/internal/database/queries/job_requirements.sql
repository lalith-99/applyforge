-- name: UpsertJobRequirements :one
INSERT INTO job_requirements (
    job_id, content_hash, role_family, normalized_title, seniority, required_skills, preferred_skills,
    required_experience_years, responsibilities, domains, education_requirements, certifications,
    location_requirements, employment_type, clearance_requirements, work_authorization_requirements, keywords
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
)
ON CONFLICT (job_id) DO UPDATE SET
    content_hash = EXCLUDED.content_hash,
    role_family = EXCLUDED.role_family,
    normalized_title = EXCLUDED.normalized_title,
    seniority = EXCLUDED.seniority,
    required_skills = EXCLUDED.required_skills,
    preferred_skills = EXCLUDED.preferred_skills,
    required_experience_years = EXCLUDED.required_experience_years,
    responsibilities = EXCLUDED.responsibilities,
    domains = EXCLUDED.domains,
    education_requirements = EXCLUDED.education_requirements,
    certifications = EXCLUDED.certifications,
    location_requirements = EXCLUDED.location_requirements,
    employment_type = EXCLUDED.employment_type,
    clearance_requirements = EXCLUDED.clearance_requirements,
    work_authorization_requirements = EXCLUDED.work_authorization_requirements,
    keywords = EXCLUDED.keywords,
    parsed_at = now()
RETURNING *;

-- name: GetJobRequirementsByJobID :one
SELECT * FROM job_requirements WHERE job_id = $1;
