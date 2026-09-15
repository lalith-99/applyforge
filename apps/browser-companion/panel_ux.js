// Browser Companion panel UX and required-field diagnostics.
// Loaded after the field drivers so generic ATS handoffs explain exactly what
// still needs user input instead of showing only a count.

if (typeof document !== "undefined") {
  queueMicrotask(() => {
    const action = document.querySelector(`#${PANEL_ID} [data-applyforge-action="true"]`);
    if (action instanceof HTMLButtonElement && detectAdapter() === "generic") {
      action.textContent = "Fill saved answers";
      action.title = "Fill answers already approved in ApplyForge; unresolved questions stay for review.";
    }
  });
}

function unresolvedRequiredFields() {
  const unresolved = [];
  const selector = [
    "input[required]",
    "textarea[required]",
    "select[required]",
    "[aria-required='true']",
    "[role='combobox'][required]",
    "[role='combobox'][aria-required='true']",
    "[aria-haspopup='listbox'][required]",
    "[aria-haspopup='listbox'][aria-required='true']",
  ].join(", ");

  for (const field of document.querySelectorAll(selector)) {
    if (!(field instanceof HTMLElement)) continue;
    if (field.disabled === true) continue;
    if (typeof isCompanionVisible === "function" ? !isCompanionVisible(field) : !isVisible(field)) continue;

    if (field instanceof HTMLInputElement && ["hidden", "submit", "button", "reset", "file"].includes(field.type)) continue;

    if (field instanceof HTMLInputElement && (field.type === "radio" || field.type === "checkbox")) {
      if (field.name) {
        const group = [...document.querySelectorAll(`input[name="${cssEscape(field.name)}"]`)]
          .filter((candidate) => candidate instanceof HTMLInputElement);
        if (group.some((candidate) => candidate.checked)) continue;
      } else if (field.checked) {
        continue;
      }
    } else if (requiredFieldHasValue(field)) {
      continue;
    }

    unresolved.push(describeRequiredField(field));
  }

  return [...new Set(unresolved.filter(Boolean))];
}

function requiredFieldHasValue(field) {
  if (field instanceof HTMLSelectElement || field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement) {
    const value = String(field.value || "").trim();
    if (!value) return false;
    const normalized = normalizePanelText(value);
    return !["select", "select one", "choose", "choose one", "please select", "required"].includes(normalized);
  }

  const ariaValue = field.getAttribute("aria-valuetext") || field.getAttribute("aria-valuenow") || "";
  if (String(ariaValue).trim()) return true;

  const selected = field.querySelector?.("[aria-selected='true'], [data-selected='true']");
  if (selected && String(selected.textContent || "").trim()) return true;

  const text = normalizePanelText(field.textContent || "");
  return Boolean(text && !["select", "select one", "choose", "choose one", "please select", "required"].includes(text));
}

function describeRequiredField(field) {
  const label = typeof labelText === "function" ? String(labelText(field) || "").trim() : "";
  if (label) return compactFieldLabel(label);

  const aria = String(field.getAttribute?.("aria-label") || "").trim();
  if (aria) return compactFieldLabel(aria);

  if (typeof companionQuestionText === "function") {
    const question = String(companionQuestionText(field) || "").trim();
    if (question) return compactFieldLabel(question);
  }

  return compactFieldLabel(field.getAttribute?.("name") || field.id || "required field");
}

function compactFieldLabel(value) {
  const text = String(value || "").replace(/\s+/g, " ").trim();
  if (!text) return "required field";
  return text.length > 140 ? `${text.slice(0, 137)}…` : text;
}

function normalizePanelText(value) {
  return String(value || "").toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
}

function appendUnresolvedDetails(message, unresolved) {
  if (!Array.isArray(unresolved) || unresolved.length === 0) return message;
  if (!/could not answer safely/i.test(String(message || ""))) return message;
  const visible = unresolved.slice(0, 4);
  const suffix = unresolved.length > visible.length ? `; +${unresolved.length - visible.length} more` : "";
  return `${message}\nNeeds your input: ${visible.join("; ")}${suffix}.`;
}

function setPanelStatus(message, kind) {
  const status = document.querySelector(`#${PANEL_ID} [data-applyforge-status="true"]`);
  if (!(status instanceof HTMLElement)) return;

  const enriched = appendUnresolvedDetails(message, unresolvedRequiredFields());
  status.textContent = enriched;
  status.style.whiteSpace = "pre-line";
  status.style.color = kind === "error" ? "#fecaca" : kind === "success" ? "#bbf7d0" : kind === "attention" ? "#fde68a" : "#fff";
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = {
    appendUnresolvedDetails,
    compactFieldLabel,
    normalizePanelText,
  };
}
