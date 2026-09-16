const assert = require("node:assert/strict");
const { isRequiredAriaWidgetResolved } = require("./required_field_safety.js");

function radiogroup(selected) {
  return {
    getAttribute(name) {
      return name === "role" ? "radiogroup" : null;
    },
    querySelector(selector) {
      assert.equal(selector, '[role="radio"][aria-checked="true" i]');
      return selected ? { role: "radio" } : null;
    },
  };
}

// A required ARIA radiogroup is resolved only by an explicitly checked child.
// Labels, values, and the existence of unchecked options must not be inferred
// as an answer.
assert.equal(isRequiredAriaWidgetResolved(radiogroup(true)), true);
assert.equal(isRequiredAriaWidgetResolved(radiogroup(false)), false);
assert.equal(
  isRequiredAriaWidgetResolved({
    getAttribute(name) {
      return name === "role" ? "radiogroup" : name === "value" ? "Yes" : null;
    },
  }),
  false,
);

console.log("required radiogroup safety tests passed");
