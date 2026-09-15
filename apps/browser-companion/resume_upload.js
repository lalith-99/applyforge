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

  // A single file control is a reasonable fallback on a job-application form.
  // With multiple controls, require positive resume/CV evidence so we do not
  // accidentally attach a resume to a cover-letter or portfolio uploader.
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

// This intentionally overrides the basic fillKnownFields implementation in
// content.js. resume_upload.js is loaded after content.js by the manifest, the
// same pattern already used above to override uploadResume with the safer
// resume-input scoring implementation.
function fillKnownFields(answers) {
  let filled = 0;
  for (const field of document.querySelectorAll("input, textarea, select")) {
    if (!(field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement || field instanceof HTMLSelectElement)) continue;
    if (!isCompanionVisible(field) || field.disabled) continue;
    if (field instanceof HTMLInputElement && ["hidden", "submit", "button", "reset", "file", "password", "search"].includes(field.type)) continue;

    const descriptor = buildCompanionFieldDescriptor(field);
    const key = matchApprovedAnswerKey(descriptor, answers);
    if (!key) continue;

    const rawValue = answers[key];
    if (rawValue == null || String(rawValue).trim() === "") continue;
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

    if (!String(field.value || "").trim()) {
      setCompanionNativeValue(field, String(value));
      filled += 1;
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
  ],
  work_authorization: [
    "work authorization",
    "authorized to work",
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

function normalizeApprovedAnswer(key, value) {
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
    const labelledBy = group.getAttribute?.("aria-labelledby");
    if (labelledBy) {
      for (const id of labelledBy.split(/\s+/).filter(Boolean)) {
        const label = document.getElementById(id);
        if (label?.textContent) parts.push(label.textContent);
      }
    }

    const question = group.querySelector?.("legend, [data-question-label], .question-label, .field-label, .label, label");
    if (question?.textContent) parts.push(question.textContent);
  }

  const labelledBy = field.getAttribute?.("aria-labelledby");
  if (labelledBy) {
    for (const id of labelledBy.split(/\s+/).filter(Boolean)) {
      const label = document.getElementById(id);
      if (label?.textContent) parts.push(label.textContent);
    }
  }

  return parts.join(" ").slice(0, 800);
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

  // Only handle checkboxes when the option itself communicates a boolean.
  // We intentionally avoid guessing agreement/consent checkboxes.
  if (input.type === "checkbox" && desiredBoolean && optionBoolean) {
    const shouldCheck = desiredBoolean === optionBoolean;
    if (input.checked !== shouldCheck) input.click();
    return true;
  }

  // Sensitive authorization/sponsorship checkboxes without explicit Yes/No
  // wording are left for the user rather than inferred from a sentence polarity.
  if (key === "sponsorship" || key === "work_authorization") return false;
  return false;
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
  };
}
