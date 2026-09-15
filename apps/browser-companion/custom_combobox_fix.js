// Final Browser Companion field-driver override for custom ATS widgets.
// Loaded after resume_upload.js so we can keep generic text/native controls while
// handling custom listbox/menu controls conservatively and synchronously.

function fillKnownFields(answers) {
  let filled = 0;
  const fields = document.querySelectorAll(
    "input, textarea, select, [role='combobox'], [aria-haspopup='listbox'], [aria-haspopup='menu']",
  );

  for (const field of fields) {
    const isNative = field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement || field instanceof HTMLSelectElement;
    const isCombobox = field instanceof HTMLElement && (
      field.getAttribute("role") === "combobox"
      || field.getAttribute("aria-haspopup") === "listbox"
      || field.getAttribute("aria-haspopup") === "menu"
    );
    if (!isNative && !isCombobox) continue;
    if (!isCompanionVisible(field) || field.disabled === true) continue;
    if (field instanceof HTMLInputElement && ["hidden", "submit", "button", "reset", "file", "password", "search"].includes(field.type)) continue;

    const descriptor = buildCompanionFieldDescriptor(field);
    const key = matchApprovedAnswerKey(descriptor, answers);
    if (!key) continue;

    clearRejectedPlaceholderValue(field);

    const rawValue = answers[key];
    if (!isUsableApprovedAnswer(key, rawValue)) continue;
    const value = normalizeApprovedAnswer(key, rawValue);
    if (value == null || String(value).trim() === "") continue;

    if (field instanceof HTMLSelectElement) {
      if (setApprovedSelect(field, String(value))) filled += 1;
      continue;
    }

    if (field instanceof HTMLInputElement && (field.type === "radio" || field.type === "checkbox")) {
      if (setApprovedChoice(field, String(value), key)) filled += 1;
      continue;
    }

    if (isCombobox) {
      // Custom location/autocomplete widgets are intentionally left alone unless
      // they are a high-confidence binary question. Opening several asynchronous
      // widgets in one pass can make one ATS control steal another control's
      // option list (the Roku location-vs-authorization bug).
      if (!shouldDriveCustomCombobox(key, value)) continue;
      if (setApprovedCombobox(field, String(value))) filled += 1;
      continue;
    }

    if (field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement) {
      if (!String(field.value || "").trim()) {
        setCompanionNativeValue(field, String(value));
        filled += 1;
      }
    }
  }

  return { filled };
}

function shouldDriveCustomCombobox(key, value) {
  if (key !== "work_authorization" && key !== "sponsorship") return false;
  return normalizeCustomBoolean(value) !== null;
}

function normalizeCustomBoolean(value) {
  const text = String(value || "").toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
  if (["yes", "true", "y", "1"].includes(text) || text.startsWith("yes ")) return "yes";
  if (["no", "false", "n", "0"].includes(text) || text.startsWith("no ")) return "no";
  return null;
}

function setApprovedCombobox(field, desired) {
  const desiredBoolean = normalizeCustomBoolean(desired);
  if (!desiredBoolean || !(field instanceof HTMLElement)) return false;

  const normalizedDesired = normalizeCompanionText(desiredBoolean);
  field.focus?.();
  field.click();
  field.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true, composed: true }));

  let option = findMatchingComboboxOption(field, normalizedDesired, desiredBoolean);
  if (option) {
    option.click();
    return true;
  }

  // Some employer-hosted ATS controls expose a text input as the combobox and
  // only commit the selection after a typed value + Enter. Do this only for the
  // already-normalized Yes/No answers, never for arbitrary strings.
  if (field instanceof HTMLInputElement) {
    const answer = desiredBoolean === "yes" ? "Yes" : "No";
    setCompanionNativeValue(field, answer);
    field.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true, composed: true }));

    option = findMatchingComboboxOption(field, normalizedDesired, desiredBoolean);
    if (option) {
      option.click();
      return true;
    }

    field.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true, composed: true }));
    field.dispatchEvent(new Event("change", { bubbles: true, composed: true }));
    if (normalizeCustomBoolean(field.value) === desiredBoolean) return true;
  }

  // Do not leave an unrelated/stale ATS menu open after an unsuccessful match.
  field.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true, composed: true }));
  field.blur?.();
  return false;
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = {
    shouldDriveCustomCombobox,
  };
}
