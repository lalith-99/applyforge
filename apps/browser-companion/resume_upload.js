function uploadResume(base64, filename) {
  if (!base64) return false;

  const inputs = [...document.querySelectorAll('input[type="file"]')]
    .filter((candidate) => candidate instanceof HTMLInputElement && !candidate.disabled);
  const input = chooseResumeFileInput(inputs);
  if (!input) return false;

  try {
    const binary = atob(base64);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i);

    const file = new File([bytes], filename, { type: "application/pdf" });
    const transfer = new DataTransfer();
    transfer.items.add(file);
    input.files = transfer.files;
    input.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
    input.dispatchEvent(new Event("change", { bubbles: true, composed: true }));

    return input.files?.length === 1 && input.files[0]?.name === filename;
  } catch {
    return false;
  }
}

function chooseResumeFileInput(inputs) {
  if (!Array.isArray(inputs) || inputs.length === 0) return null;

  const ranked = inputs
    .map((input, index) => ({ input, index, score: scoreResumeFileInput(input) }))
    .sort((left, right) => right.score - left.score || left.index - right.index);

  const best = ranked[0];
  if (!best) return null;

  if (inputs.length === 1) return best.input;
  return best.score > 0 ? best.input : null;
}

function scoreResumeFileInput(input) {
  const descriptor = normalizeResumeDescriptor([
    input.name,
    input.id,
    input.getAttribute?.("aria-label"),
    input.getAttribute?.("data-testid"),
    input.getAttribute?.("data-qa"),
    input.getAttribute?.("accept"),
    typeof labelText === "function" ? labelText(input) : "",
  ].filter(Boolean).join(" "));

  let score = scoreResumeDescriptor(descriptor);
  const accept = String(input.getAttribute?.("accept") || "").toLowerCase();
  if (accept.includes("pdf") || accept.includes("application/pdf")) score += 2;
  if (input.multiple) score -= 1;
  return score;
}

function scoreResumeDescriptor(descriptor) {
  const text = normalizeResumeDescriptor(descriptor);
  let score = 0;

  if (/\bresume\b/.test(text)) score += 10;
  if (/\bcv\b/.test(text) || /curriculum vitae/.test(text)) score += 9;
  if (/\battach(ment)?\b/.test(text) || /\bupload\b/.test(text)) score += 1;

  if (/cover letter/.test(text)) score -= 12;
  if (/portfolio|work sample|writing sample/.test(text)) score -= 10;
  if (/photo|image|avatar/.test(text)) score -= 10;
  if (/transcript|certificate|certification/.test(text)) score -= 8;

  return score;
}

function normalizeResumeDescriptor(value) {
  return String(value || "").toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
}

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

const APPROVED_FIELD_ALIASES = {
  sponsorship: [
    "sponsorship",
    "visa sponsorship",
    "require sponsorship",
    "requires sponsorship",
    "need sponsorship",
    "future sponsorship",
    "now or in the future require",
    "immigration sponsorship",
    "employment sponsorship",
    "employment visa sponsorship",
    "work visa support",
  ],
  work_authorization: [
    "work authorization",
    "authorized to work",
    "authorized to lawfully work",
    "lawfully work",
    "legally authorized",
    "eligible to work",
    "permission to work",
    "right to work",
  ],
  first_name: ["first name", "firstname", "given name"],
  last_name: ["last name", "lastname", "surname", "family name"],
  full_name: ["full name", "name"],
  email: ["email", "email address"],
  phone: ["phone", "phone number", "mobile", "telephone"],
  location: ["current location", "location", "city"],
  desired_location: ["desired location", "preferred location"],
  linkedin_url: ["linkedin", "linkedin url", "linkedin profile"],
  github_url: ["github", "github url", "github profile"],
  portfolio_url: ["portfolio", "website", "personal website"],
  salary_expectation: ["salary", "compensation", "desired salary", "salary expectation"],
  notice_period: ["notice period", "start date", "available to start", "availability"],
};

const REJECTED_ANSWER_TOKENS = new Set([
  "required",
  "required field",
  "optional",
  "select",
  "select one",
  "please select",
  "choose",
  "choose one",
  "please choose",
  "not provided",
  "not set",
  "unknown",
  "placeholder",
]);

function matchApprovedAnswerKey(descriptor, answers) {
  const normalizedDescriptor = normalizeCompanionText(descriptor);
  for (const [key, aliases] of Object.entries(APPROVED_FIELD_ALIASES)) {
    if (!(key in (answers || {}))) continue;
    if (aliases.some((alias) => normalizedDescriptor.includes(normalizeCompanionText(alias)))) return key;
  }

  for (const key of Object.keys(answers || {})) {
    const words = normalizeCompanionText(key.replaceAll("_", " "));
    if (words.length >= 4 && normalizedDescriptor.includes(words)) return key;
  }
  return null;
}

function isUsableApprovedAnswer(key, value) {
  if (value == null || typeof value === "object") return false;
  const text = String(value).trim();
  if (!text) return false;
  const normalized = normalizeCompanionText(text);
  if (!normalized || REJECTED_ANSWER_TOKENS.has(normalized)) return false;

  if (["linkedin_url", "github_url", "portfolio_url"].includes(key)) {
    try {
      const parsed = new URL(text);
      if (parsed.protocol !== "https:" && parsed.protocol !== "http:") return false;
      if (key === "linkedin_url" && !parsed.hostname.toLowerCase().includes("linkedin.com")) return false;
      if (key === "github_url" && !parsed.hostname.toLowerCase().includes("github.com")) return false;
    } catch {
      return false;
    }
  }

  return true;
}

function normalizeApprovedAnswer(key, value) {
  if (!isUsableApprovedAnswer(key, value)) return null;
  const text = normalizeCompanionText(value);
  const booleanValue = normalizeCompanionBoolean(value);
  if (booleanValue) return booleanValue;

  if (key === "sponsorship") {
    if (/\b(no sponsorship|do not require|does not require|not require|without sponsorship|no visa sponsorship)\b/.test(text)) return "no";
    if (/\b(h ?1b|h1 b|visa|sponsor|sponsorship|transfer|change of employer|immigration support)\b/.test(text)) return "yes";
  }

  if (key === "work_authorization") {
    if (/\b(not authorized|not eligible|no work authorization|not permitted)\b/.test(text)) return "no";
    if (/\b(h ?1b|h1 b|authorized|eligible|citizen|permanent resident|green card|ead|work permit|employment authorization|work visa)\b/.test(text)) return "yes";
  }

  return value;
}

function buildCompanionFieldDescriptor(field) {
  const parts = [
    field.name,
    field.id,
    field.getAttribute?.("aria-label"),
    field.getAttribute?.("placeholder"),
    field.getAttribute?.("data-label"),
    field.getAttribute?.("data-question"),
    typeof labelText === "function" ? labelText(field) : "",
    companionQuestionText(field),
  ];
  return normalizeCompanionText(parts.filter(Boolean).join(" "));
}

function companionQuestionText(field) {
  const parts = [];

  const fieldset = field.closest?.("fieldset");
  const legend = fieldset?.querySelector?.("legend");
  if (legend?.textContent) parts.push(legend.textContent);

  const group = field.closest?.('[role="group"], [role="radiogroup"], [data-question], [data-field], .application-question, .form-field, .field');
  if (group) {
    collectLabelledByText(group, parts);
    collectQuestionLikeText(group, parts);
  }

  collectLabelledByText(field, parts);
  collectDescribedByText(field, parts);

  let node = field;
  for (let depth = 0; depth < 7; depth += 1) {
    const parent = node.parentElement;
    if (!parent || parent === document.body || parent.tagName === "FORM") break;

    collectLabelledByText(parent, parts);
    collectDescribedByText(parent, parts);
    collectQuestionLikeText(parent, parts);

    const previous = node.previousElementSibling;
    if (previous) addCompactText(previous, parts);

    node = parent;
  }

  return [...new Set(parts.map((value) => String(value).trim()).filter(Boolean))].join(" ").slice(0, 1800);
}

function collectLabelledByText(element, parts) {
  const labelledBy = element.getAttribute?.("aria-labelledby");
  if (!labelledBy) return;
  for (const id of labelledBy.split(/\s+/).filter(Boolean)) {
    const label = document.getElementById(id);
    if (label?.textContent) parts.push(label.textContent);
  }
}

function collectDescribedByText(element, parts) {
  const describedBy = element.getAttribute?.("aria-describedby");
  if (!describedBy) return;
  for (const id of describedBy.split(/\s+/).filter(Boolean)) {
    const description = document.getElementById(id);
    if (description?.textContent) parts.push(description.textContent);
  }
}

function collectQuestionLikeText(container, parts) {
  const candidates = container.querySelectorAll?.(
    ":scope > legend, :scope > label, :scope > [data-question-label], :scope > .question-label, :scope > .field-label, :scope > [class*='question'], :scope > [class*='label']",
  );
  if (!candidates) return;
  for (const candidate of candidates) addCompactText(candidate, parts);
}

function addCompactText(element, parts) {
  const text = String(element?.textContent || "").replace(/\s+/g, " ").trim();
  if (text && text.length <= 600) parts.push(text);
}

function setApprovedSelect(select, desired) {
  const normalizedDesired = normalizeCompanionText(desired);
  const yesNo = normalizeCompanionBoolean(desired);
  const option = [...select.options].find((candidate) => {
    const optionText = normalizeCompanionText(`${candidate.text} ${candidate.value}`);
    return optionText === normalizedDesired
      || optionText.includes(normalizedDesired)
      || (yesNo && normalizeCompanionBoolean(optionText) === yesNo);
  });
  if (!option) return false;
  if (select.value === option.value) return false;
  select.value = option.value;
  select.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
  select.dispatchEvent(new Event("change", { bubbles: true, composed: true }));
  select.dispatchEvent(new Event("blur", { bubbles: true, composed: true }));
  return true;
}

function setApprovedChoice(input, desired, key) {
  const desiredBoolean = normalizeCompanionBoolean(desired);
  const optionDescriptor = normalizeCompanionText(`${input.value || ""} ${typeof labelText === "function" ? labelText(input) : ""}`);
  const optionBoolean = normalizeCompanionBoolean(optionDescriptor);

  if (input.type === "radio") {
    const matches = normalizeCompanionText(desired) === normalizeCompanionText(input.value)
      || optionDescriptor.includes(normalizeCompanionText(desired))
      || (desiredBoolean && optionBoolean === desiredBoolean);
    if (!matches) return false;
    if (!input.checked) input.click();
    return true;
  }

  if (input.type === "checkbox" && desiredBoolean && optionBoolean) {
    const shouldCheck = desiredBoolean === optionBoolean;
    if (input.checked !== shouldCheck) input.click();
    return true;
  }

  if (key === "sponsorship" || key === "work_authorization") return false;
  return false;
}

function setApprovedCombobox(field, desired) {
  const desiredBoolean = normalizeCompanionBoolean(desired);
  const normalizedDesired = normalizeCompanionText(desired);
  const priorValue = field instanceof HTMLInputElement ? String(field.value || "") : "";

  if (field instanceof HTMLElement) {
    field.focus?.();
    field.click();
    field.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true, composed: true }));
  }

  const tryPick = () => {
    const option = findMatchingComboboxOption(field, normalizedDesired, desiredBoolean);
    if (!option) return false;
    option.click();
    return true;
  };

  if (tryPick()) return true;

  setTimeout(() => {
    if (tryPick()) return;
    if (field instanceof HTMLInputElement && desiredBoolean) {
      setCompanionNativeValue(field, desiredBoolean === "yes" ? "Yes" : "No");
      field.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", bubbles: true, composed: true }));
    }
  }, 40);

  setTimeout(() => {
    if (tryPick()) return;
    if (field instanceof HTMLInputElement && !String(field.value || "").trim() && priorValue) {
      setCompanionNativeValue(field, priorValue);
    }
  }, 180);

  return false;
}

function findMatchingComboboxOption(field, normalizedDesired, desiredBoolean) {
  const roots = [];
  const controls = field.getAttribute?.("aria-controls");
  if (controls) {
    for (const id of controls.split(/\s+/).filter(Boolean)) {
      const controlled = document.getElementById(id);
      if (controlled) roots.push(controlled);
    }
  }

  const popupRoots = document.querySelectorAll(
    "[role='listbox'], [role='menu'], [role='dialog'], [class*='dropdown'], [class*='menu'], [class*='popover']",
  );
  for (const popup of popupRoots) {
    if (popup instanceof HTMLElement && isCompanionVisible(popup)) roots.push(popup);
  }
  roots.push(document);

  const seen = new Set();
  for (const root of roots) {
    if (seen.has(root)) continue;
    seen.add(root);
    const options = root.querySelectorAll?.(
      "[role='option'], [role='menuitemradio'], [role='menuitem'], [role='radio'], option, li[data-value], [data-value][role], [class*='option']",
    ) || [];
    for (const option of options) {
      if (!(option instanceof HTMLElement) || !isCompanionVisible(option)) continue;
      const text = normalizeCompanionText(`${option.textContent || ""} ${option.getAttribute?.("data-value") || ""} ${option.getAttribute?.("value") || ""}`);
      if (!text) continue;
      if (text === normalizedDesired || text.includes(normalizedDesired)) return option;
      if (desiredBoolean && normalizeCompanionBoolean(text) === desiredBoolean) return option;
    }
  }
  return null;
}

function clearRejectedPlaceholderValue(field) {
  if (!(field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement)) return;
  const current = String(field.value || "").trim();
  if (!current || !REJECTED_ANSWER_TOKENS.has(normalizeCompanionText(current))) return;
  setCompanionNativeValue(field, "");
}

function setCompanionNativeValue(field, value) {
  const prototype = field instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(prototype, "value")?.set;
  if (setter) setter.call(field, value);
  else field.value = value;
  field.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
  field.dispatchEvent(new Event("change", { bubbles: true, composed: true }));
  field.dispatchEvent(new Event("blur", { bubbles: true, composed: true }));
}

function isCompanionVisible(element) {
  if (!(element instanceof Element)) return false;
  const style = window.getComputedStyle(element);
  if (style.display === "none" || style.visibility === "hidden" || Number(style.opacity) === 0) return false;
  const rect = element.getBoundingClientRect();
  return rect.width > 0 && rect.height > 0;
}

function normalizeCompanionText(value) {
  return String(value || "").toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
}

function normalizeCompanionBoolean(value) {
  const text = normalizeCompanionText(value);
  if (["yes", "true", "y", "1"].includes(text) || text.startsWith("yes ")) return "yes";
  if (["no", "false", "n", "0"].includes(text) || text.startsWith("no ")) return "no";
  return null;
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = {
    scoreResumeDescriptor,
    matchApprovedAnswerKey,
    normalizeApprovedAnswer,
    isUsableApprovedAnswer,
  };
}
