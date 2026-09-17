const assert = require("node:assert/strict");
const { isRequiredAriaWidgetResolved } = require("./required_field_safety.js");

function ariaWidget(attributes = {}) {
  return {
    getAttribute(name) {
      return Object.prototype.hasOwnProperty.call(attributes, name) ? attributes[name] : null;
    },
  };
}

function ariaListbox(options = [], attributes = {}) {
  const listbox = ariaWidget({ role: "listbox", ...attributes });
  listbox.querySelectorAll = (selector) => selector === '[role="option"]' ? options : [];
  return listbox;
}

// A required custom listbox is resolved only by an explicitly selected option.
// Container metadata or option text must not be mistaken for committed state.
assert.equal(isRequiredAriaWidgetResolved(ariaListbox([
  ariaWidget({ "aria-selected": "true" }),
])), true);
assert.equal(isRequiredAriaWidgetResolved(ariaListbox([
  ariaWidget({ "aria-selected": " True " }),
])), true);
assert.equal(isRequiredAriaWidgetResolved(ariaListbox([
  ariaWidget({ "aria-selected": "false" }),
], { value: "placeholder" })), false);
assert.equal(isRequiredAriaWidgetResolved(ariaListbox([
  ariaWidget({ "aria-selected": "" }),
])), false);
assert.equal(isRequiredAriaWidgetResolved(ariaListbox([])), false);

console.log("required ARIA listbox safety tests passed");
