const assert = require("node:assert/strict");
const {
  scoreResumeDescriptor,
  matchApprovedAnswerKey,
  normalizeApprovedAnswer,
} = require("./resume_upload.js");

assert(scoreResumeDescriptor("Upload Resume PDF") > 0);
assert(scoreResumeDescriptor("Curriculum Vitae attachment") > 0);
assert(scoreResumeDescriptor("Cover Letter upload") < 0);
assert(scoreResumeDescriptor("Portfolio work sample") < 0);
assert(scoreResumeDescriptor("Candidate photo") < 0);
assert(scoreResumeDescriptor("Resume") > scoreResumeDescriptor("Upload attachment"));

const answers = {
  work_authorization: "H-1B",
  sponsorship: "H-1B transfer required",
  linkedin_url: "https://www.linkedin.com/in/example",
};

assert.equal(
  matchApprovedAnswerKey("are you legally authorized to work in the united states", answers),
  "work_authorization",
);
assert.equal(
  matchApprovedAnswerKey("are you authorized to lawfully work for roku in the country to which you are applying", answers),
  "work_authorization",
);
assert.equal(
  matchApprovedAnswerKey("will you now or in the future require visa sponsorship", answers),
  "sponsorship",
);
assert.equal(
  matchApprovedAnswerKey("do you now or will you in the future require employment visa sponsorship or support including renewals and transfers", answers),
  "sponsorship",
);
assert.equal(
  matchApprovedAnswerKey("linkedin profile", answers),
  "linkedin_url",
);
assert.equal(normalizeApprovedAnswer("work_authorization", "H-1B"), "yes");
assert.equal(normalizeApprovedAnswer("sponsorship", "H-1B transfer required"), "yes");
assert.equal(normalizeApprovedAnswer("sponsorship", "Do not require sponsorship"), "no");
assert.equal(normalizeApprovedAnswer("work_authorization", "Not authorized"), "no");

console.log("browser companion resume and approved-answer mapping tests passed");
