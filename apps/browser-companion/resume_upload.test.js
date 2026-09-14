const assert = require("node:assert/strict");
const { scoreResumeDescriptor } = require("./resume_upload.js");

assert(scoreResumeDescriptor("Upload Resume PDF") > 0);
assert(scoreResumeDescriptor("Curriculum Vitae attachment") > 0);
assert(scoreResumeDescriptor("Cover Letter upload") < 0);
assert(scoreResumeDescriptor("Portfolio work sample") < 0);
assert(scoreResumeDescriptor("Candidate photo") < 0);
assert(scoreResumeDescriptor("Resume") > scoreResumeDescriptor("Upload attachment"));

console.log("resume upload scoring tests passed");
