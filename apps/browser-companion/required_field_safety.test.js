const assert = require("node:assert/strict");
const { isRequiredChoiceResolved, shouldValidateRequiredField, isAriaRequired, isRequiredAriaWidgetResolved } = require("./required_field_safety.js");

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

assert.equal(
  isRequiredChoiceResolved(
    { type: "radio", name: "work_authorization", checked: false, form: applicationForm },
    [{ type: "radio", name: "work_authorization", checked: true, form: otherForm }],
  ),
  false,
);
assert.equal(
  isRequiredChoiceResolved(
    { type: "radio", name: "remote", checked: false, form: null },
    [{ type: "radio", name: "remote", checked: true, form: null }],
  ),
  true,
);
assert.equal(
  isRequiredChoiceResolved(
    { type: "checkbox", name: "consents", checked: false },
    [{ type: "checkbox", name: "consents", checked: true }],
  ),
  false,
);

assert.equal(shouldValidateRequiredField({ type: "text", disabled: true, readOnly: false }), false);
assert.equal(shouldValidateRequiredField({ type: "text", disabled: false, readOnly: true }), false);
assert.equal(shouldValidateRequiredField({ type: "email", disabled: false, readOnly: true }), false);
assert.equal(shouldValidateRequiredField({ type: "text", disabled: false, readOnly: false }), true);
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

// ARIA-required token values are case-insensitive. An ATS emitting uppercase or
// padded true must not bypass the fail-closed required-field guard.
assert.equal(isAriaRequired(ariaWidget({ "aria-required": "true" })), true);
assert.equal(isAriaRequired(ariaWidget({ "aria-required": "TRUE" })), true);
assert.equal(isAriaRequired(ariaWidget({ "aria-required": " True " })), true);
assert.equal(isAriaRequired(ariaWidget({ "aria-required": "false" })), false);
assert.equal(isAriaRequired(ariaWidget({ "aria-required": "" })), false);
assert.equal(isAriaRequired(ariaWidget()), false);

assert.equal(shouldValidateRequiredField(ariaWidget({ "aria-disabled": "true" })), false);
assert.equal(shouldValidateRequiredField(ariaWidget({ "aria-disabled": "TRUE" })), false);
assert.equal(shouldValidateRequiredField(ariaWidget({ "aria-disabled": "false" })), true);
assert.equal(shouldValidateRequiredField(ariaWidget()), true);

assert.equal(shouldValidateRequiredField(ariaWidget({ "aria-hidden": "true" })), false);
assert.equal(shouldValidateRequiredField(ariaWidget({ "aria-hidden": "TRUE" })), false);
assert.equal(shouldValidateRequiredField(ariaWidget({ "aria-hidden": "false" })), true);
assert.equal(shouldValidateRequiredField(ariaWidget({ "aria-hidden": "" })), true);

assert.equal(isRequiredAriaWidgetResolved(ariaWidget()), false);
assert.equal(isRequiredAriaWidgetResolved(ariaWidget({ "aria-valuetext": "Yes" })), true);
assert.equal(isRequiredAriaWidgetResolved(ariaWidget({ "data-value": "No" })), true);
assert.equal(isRequiredAriaWidgetResolved(ariaWidget({ value: "United States" })), true);
assert.equal(isRequiredAriaWidgetResolved(ariaWidget({ "aria-valuetext": "   " })), false);
assert.equal(isRequiredAriaWidgetResolved({ textContent: "Are you authorized to work?" }), false);

console.log("required field safety tests passed");
