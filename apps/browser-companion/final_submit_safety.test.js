const assert = require("node:assert/strict");

global.normalize = (value) => String(value || "").toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
const { isExplicitFinalSubmitLabel, isEnabledFinalSubmitControl } = require("./final_submit_safety.js");

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

function control({ disabled = false, ariaDisabled = null } = {}) {
  return {
    disabled,
    getAttribute(name) {
      return name === "aria-disabled" ? ariaDisabled : null;
    },
  };
}

assert.equal(isEnabledFinalSubmitControl(control()), true, "enabled submit control should be eligible");
assert.equal(isEnabledFinalSubmitControl(control({ disabled: true })), false, "disabled submit control must not be eligible");
assert.equal(isEnabledFinalSubmitControl(control({ ariaDisabled: "true" })), false, "aria-disabled submit control must not be eligible");
assert.equal(isEnabledFinalSubmitControl(control({ ariaDisabled: "TRUE" })), false, "aria-disabled matching should be case insensitive");
assert.equal(isEnabledFinalSubmitControl(null), false, "missing submit control must not be eligible");

console.log("final submit label safety tests passed");
