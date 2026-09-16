// Required radio buttons are satisfied by one checked member of their HTML
// radio group, but that group is scoped by both name and form owner. A checked
// radio in another form must never satisfy a required field in the application
// form. Required checkboxes remain independent controls.
function isRequiredChoiceResolved(field, checkedRadios = []) {
  if (!field) return false;
  const type = String(field.type || "").toLowerCase();
  if (type === "checkbox") return field.checked === true;
  if (type === "radio") {
    if (field.checked === true) return true;
    const name = String(field.name || "");
    if (!name) return false;
    return checkedRadios.some((candidate) =>
      candidate &&
      candidate.checked === true &&
      String(candidate.name || "") === name &&
      candidate.form === field.form
    );
  }
  return false;
}

// Native constraint validation excludes disabled controls and readonly text
// controls. Keep the companion's pre-submit guard aligned with the browser so
// an ATS-owned readonly/required field cannot permanently block a valid form.
function shouldValidateRequiredField(field) {
  if (!field || field.disabled === true) return false;
  const type = String(field.type || "").toLowerCase();
  if (field.readOnly === true && !["radio", "checkbox", "file"].includes(type)) return false;
  return true;
}

// Custom ATS comboboxes and listbox-backed controls are often non-native
// elements marked only with aria-required. They are outside native constraint
// validation, so fail closed unless the widget exposes an explicit selected
// value. Do not infer a selection from textContent because it commonly contains
// the question/placeholder label rather than a committed answer.
function isRequiredAriaWidgetResolved(field) {
  if (!field || typeof field.getAttribute !== "function") return false;
  const explicitValue = [
    field.getAttribute("aria-valuetext"),
    field.getAttribute("data-value"),
    field.getAttribute("value"),
  ];
  return explicitValue.some((value) => String(value || "").trim() !== "");
}

function unresolvedRequiredFields() {
  const unresolved = [];
  const required = [...document.querySelectorAll("input[required], textarea[required], select[required], [aria-required='true']")];
  const checkedRadios = [...document.querySelectorAll('input[type="radio"]:checked')];

  for (const field of required) {
    const isNative = field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement || field instanceof HTMLSelectElement;
    if (!isVisible(field) || !shouldValidateRequiredField(field)) continue;

    if (!isNative) {
      if (isRequiredAriaWidgetResolved(field)) continue;
      unresolved.push(labelText(field) || field.getAttribute("aria-label") || field.getAttribute("name") || field.id || "required field");
      continue;
    }

    if (field instanceof HTMLInputElement && ["hidden", "submit", "button"].includes(field.type)) continue;

    if (field instanceof HTMLInputElement && (field.type === "radio" || field.type === "checkbox")) {
      if (isRequiredChoiceResolved(field, checkedRadios)) continue;
    } else if (String(field.value || "").trim()) {
      continue;
    }
    unresolved.push(labelText(field) || field.getAttribute("aria-label") || field.name || field.id || "required field");
  }
  return [...new Set(unresolved)];
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = { isRequiredChoiceResolved, shouldValidateRequiredField, isRequiredAriaWidgetResolved };
}
