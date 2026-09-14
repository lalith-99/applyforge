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

if (typeof module !== "undefined" && module.exports) {
  module.exports = { scoreResumeDescriptor };
}
