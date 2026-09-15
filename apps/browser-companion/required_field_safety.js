// Required radio buttons are satisfied by one checked member of their group,
// but HTML required checkboxes are independent controls. Treating another
// checkbox with the same name as satisfying a required checkbox can cross the
// final-submit boundary with an unchecked consent/acknowledgement control.
function isRequiredChoiceResolved(field, checkedRadioNames = new Set()) {
  if (!field) return false;
  const type = String(field.type || "").toLowerCase();
  if (type === "checkbox") return field.checked === true;
  if (type === "radio") {
    if (field.checked === true) return true;
    const name = String(field.name || "");
    return Boolean(name && checkedRadioNames.has(name));
  }
  return false;
}

function unresolvedRequiredFields() {
  const unresolved = [];
  const required = [...document.querySelectorAll("input[required], textarea[required], select[required], [aria-required='true']")];
  const checkedRadioNames = new Set(
    [...document.querySelectorAll('input[type="radio"]:checked')]
      .map((field) => String(field.name || ""))
      .filter(Boolean),
  );

  for (const field of required) {
    if (!(field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement || field instanceof HTMLSelectElement)) continue;
    if (!isVisible(field) || field.disabled) continue;
    if (field instanceof HTMLInputElement && ["hidden", "submit", "button"].includes(field.type)) continue;

    if (field instanceof HTMLInputElement && (field.type === "radio" || field.type === "checkbox")) {
      if (isRequiredChoiceResolved(field, checkedRadioNames)) continue;
    } else if (String(field.value || "").trim()) {
      continue;
    }
    unresolved.push(labelText(field) || field.getAttribute("aria-label") || field.name || field.id || "required field");
  }
  return [...new Set(unresolved)];
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = { isRequiredChoiceResolved };
}
