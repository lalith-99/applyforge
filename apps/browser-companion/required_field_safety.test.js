const assert = require("node:assert/strict");
const { isRequiredChoiceResolved, shouldValidateRequiredField, isRequiredAriaWidgetResolved } = require("./required_field_safety.js");

assert.equal(isRequiredChoiceResolved({ type: "checkbox", checked: true }), true);
assert.equal(isRequiredChoiceResolved({ type: "checkbox", checked: false }), false);

const applicationForm = { id: "application" };
const otherForm = { id: "newsletter" };
const checkedRadios = [
  { type: "radio", name: "work_authorization", checked: true, form: applicationForm },
];
assert.equal(isRequiredChoiceResolved({ type: "radio", name: "work_authorization", checked: false, form: applicationForm }, checkedRadios), true);
assert.equal(isRequiredChoiceResolved({ type: "radio", name: "sponsorship", checked: false, form: applicationForm }, checkedRadios), false);
assert.equal(isRequiredChoiceResolved({ type: "radio", name: "", checked: true, form: applicationForm }, checkedRadios), true);
assert.equal(isRequiredChoiceResolved({ type: "radio", name: "", checked: false, form: applicationForm }, checkedRadios), false);

// Regression: same-name radios in different forms are distinct HTML radio
// groups. A checked control elsewhere on the page cannot satisfy this form.
assert.equal(
  isRequiredChoiceResolved(
    { type: "radio", name: "work_authorization", checked: false, form: applicationForm },
    [{ type: "radio", name: "work_authorization", checked: true, form: otherForm }],
  ),
  false,
);

// Form-less radios can still satisfy each other when they share the same name.
assert.equal(
  isRequiredChoiceResolved(
    { type: "radio", name: "remote", checked: false, form: null },
    [{ type: "radio", name: "remote", checked: true, form: null }],
  ),
  true,
);

// Regression: another checked checkbox with the same name must never satisfy a
// required consent checkbox. Checkbox resolution deliberately ignores groups.
assert.equal(
  isRequiredChoiceResolved(
    { type: "checkbox", name: "consents", checked: false },
    [{ type: "checkbox", name: "consents", checked: true }],
  ),
  false,
);

// Native HTML constraint validation ignores disabled and readonly text-like
// controls. The companion should not create a stricter, impossible-to-resolve
// blocker for ATS-owned readonly fields.
assert.equal(shouldValidateRequiredField({ type: "text", disabled: true, readOnly: false }), false);
assert.equal(shouldValidateRequiredField({ type: "text", disabled: false, readOnly: true }), false);
assert.equal(shouldValidateRequiredField({ type: "email", disabled: false, readOnly: true }), false);
assert.equal(shouldValidateRequiredField({ type: "text", disabled: false, readOnly: false }), true);
// Keep choice/file controls conservative even if a synthetic page sets the
// readonly property; readonly has no native constraint-validation meaning for them.
assert.equal(shouldValidateRequiredField({ type: "checkbox", disabled: false, readOnly: true }), true);
assert.equal(shouldValidateRequiredField({ type: "radio", disabled: false, readOnly: true }), true);
assert.equal(shouldValidateRequiredField({ type: "file", disabled: false, readOnly: true }), true);

function ariaWidget(attributes = {}) {
  return {
    getAttribute(name) {
      return Object.prototype.hasOwnProperty.call(attributes, name) ? attributes[name] : null;
    },
  };
}

// ARIA-disabled custom widgets are not actionable application requirements and
// must not create an impossible pre-submit blocker. Only the explicit true
// state is skipped; enabled or missing states remain conservative.
assert.equal(shouldValidateRequiredField(ariaWidget({ "aria-disabled": "true" })), false);
assert.equal(shouldValidateRequiredField(ariaWidget({ "aria-disabled": "TRUE" })), false);
assert.equal(shouldValidateRequiredField(ariaWidget({ "aria-disabled": "false" })), true);
assert.equal(shouldValidateRequiredField(ariaWidget()), true);

// Non-native aria-required widgets must fail closed unless the ATS exposes a
// committed value. Placeholder/question text is intentionally not considered.
assert.equal(isRequiredAriaWidgetResolved(ariaWidget()), false);
assert.equal(isRequiredAriaWidgetResolved(ariaWidget({ "aria-valuetext": "Yes" })), true);
assert.equal(isRequiredAriaWidgetResolved(ariaWidget({ "data-value": "No" })), true);
assert.equal(isRequiredAriaWidgetResolved(ariaWidget({ value: "United States" })), true);
assert.equal(isRequiredAriaWidgetResolved(ariaWidget({ "aria-valuetext": "   " })), false);
assert.equal(isRequiredAriaWidgetResolved({ textContent: "Are you authorized to work?" }), false);

console.log("required field safety tests passed");
