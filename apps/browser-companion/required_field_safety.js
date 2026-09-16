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

// Native constraint validation excludes disabled and readonly text controls.
// Custom ATS widgets can also remain mounted while explicitly removed from the
// accessibility tree. Skip only explicit aria-disabled/aria-hidden true states
// so stale conditional widgets cannot create an impossible submit blocker.
function shouldValidateRequiredField(field) {
  if (!field || field.disabled === true) return false;
  if (typeof field.getAttribute === "function") {
    if (String(field.getAttribute("aria-disabled") || "").trim().toLowerCase() === "true") return false;
    if (String(field.getAttribute("aria-hidden") || "").trim().toLowerCase() === "true") return false;
  }
  const type = String(field.type || "").toLowerCase();
  if (field.readOnly === true && !["radio", "checkbox", "file"].includes(type)) return false;
  return true;
}

// ARIA token values are case-insensitive. Query aria-required by presence and
// normalize the value in JavaScript so an ATS emitting TRUE cannot bypass the
// fail-closed required-field guard. Explicit false/empty states are not required.
function isAriaRequired(field) {
  if (!field || typeof field.getAttribute !== "function") return false;
  return String(field.getAttribute("aria-required") || "").trim().toLowerCase() === "true";
}

// Custom ATS comboboxes, listboxes, checkboxes, radios, and radiogroups are
// often non-native elements marked only with ARIA attributes. They are outside
// native constraint validation, so fail closed unless the widget exposes an
// explicit committed state/value. Do not infer a selection from textContent
// because it commonly contains the question/placeholder label rather than a
// committed answer.
function isRequiredAriaWidgetResolved(field) {
  if (!field || typeof field.getAttribute !== "function") return false;
  const role = String(field.getAttribute("role") || "").trim().toLowerCase();
  if (["checkbox", "radio"].includes(role)) {
    return String(field.getAttribute("aria-checked") || "").trim().toLowerCase() === "true";
  }
  if (role === "radiogroup") {
    if (typeof field.querySelector !== "function") return false;
    return Boolean(field.querySelector('[role="radio"][aria-checked="true" i]'));
  }
  const explicitValue = [
    field.getAttribute("aria-valuetext"),
    field.getAttribute("data-value"),
    field.getAttribute("value"),
  ];
  return explicitValue.some((value) => String(value || "").trim() !== "");
}

function unresolvedRequiredFields() {
  const unresolved = [];
  const required = [...document.querySelectorAll("input[required], textarea[required], select[required], [aria-required]")];
  const checkedRadios = [...document.querySelectorAll('input[type="radio"]:checked')];

  for (const field of required) {
    const isNative = field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement || field instanceof HTMLSelectElement;
    const isNativeRequired = isNative && field.required === true;
    if (!isNativeRequired && !isAriaRequired(field)) continue;
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
  module.exports = { isRequiredChoiceResolved, shouldValidateRequiredField, isAriaRequired, isRequiredAriaWidgetResolved };
}
