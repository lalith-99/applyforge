const assert = require("node:assert/strict");

global.normalize = (value) => String(value || "").toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
const { isExplicitFinalSubmitLabel } = require("./final_submit_safety.js");

for (const label of [
  "Submit",
  "Submit Application",
  "Submit my application",
  "Send Application",
  "Send my application",
]) {
  assert.equal(isExplicitFinalSubmitLabel(label), true, `expected final submit label: ${label}`);
}

for (const label of [
  "Apply",
  "Apply now",
  "Apply for this job",
  "Continue",
  "Next",
  "Save and continue",
  "Submit profile",
  "Submit search",
  "Send",
]) {
  assert.equal(isExplicitFinalSubmitLabel(label), false, `expected non-final control: ${label}`);
}

console.log("final submit label safety tests passed");
