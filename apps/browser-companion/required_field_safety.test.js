const assert = require("node:assert/strict");
const { isRequiredChoiceResolved } = require("./required_field_safety.js");

assert.equal(isRequiredChoiceResolved({ type: "checkbox", checked: true }), true);
assert.equal(isRequiredChoiceResolved({ type: "checkbox", checked: false }), false);

const checkedRadios = new Set(["work_authorization"]);
assert.equal(isRequiredChoiceResolved({ type: "radio", name: "work_authorization", checked: false }, checkedRadios), true);
assert.equal(isRequiredChoiceResolved({ type: "radio", name: "sponsorship", checked: false }, checkedRadios), false);
assert.equal(isRequiredChoiceResolved({ type: "radio", name: "", checked: true }, checkedRadios), true);
assert.equal(isRequiredChoiceResolved({ type: "radio", name: "", checked: false }, checkedRadios), false);

// Regression: another checked checkbox with the same name must never satisfy a
// required consent checkbox. Checkbox resolution deliberately ignores groups.
assert.equal(
  isRequiredChoiceResolved({ type: "checkbox", name: "consents", checked: false }, new Set(["consents"])),
  false,
);

console.log("required field safety tests passed");
