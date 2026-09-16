const assert = require("node:assert/strict");
const { isRequiredChoiceResolved } = require("./required_field_safety.js");

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

console.log("required field safety tests passed");
