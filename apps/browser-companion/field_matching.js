function matchAnswerKey(descriptor, answers) {
  const normalizedDescriptor = normalize(descriptor);
  const aliases = {
    first_name: ["first name", "firstname", "given name"],
    last_name: ["last name", "lastname", "surname", "family name"],
    full_name: ["full name", "candidate name"],
    email: ["email", "email address"],
    phone: ["phone", "phone number", "mobile", "telephone"],
    location: ["current location", "home location", "city"],
    desired_location: ["desired location", "preferred location"],
    linkedin_url: ["linkedin", "linkedin url", "linkedin profile"],
    github_url: ["github", "github url", "github profile"],
    portfolio_url: ["portfolio", "website", "personal website"],
    salary_expectation: ["salary", "compensation", "desired salary", "salary expectation"],
    notice_period: ["notice period", "start date", "available to start", "availability"],
    work_authorization: [
      "work authorization", "authorized to work", "legally authorized", "eligible to work",
      "authorization to work", "right to work", "work eligibility",
    ],
    sponsorship: [
      "sponsorship", "visa sponsorship", "require sponsorship", "requires sponsorship",
      "need sponsorship", "immigration sponsorship", "employment visa", "work visa",
      "now or in the future", "future sponsorship",
    ],
  };

  // Sponsorship questions often contain both "authorized" and "sponsorship".
  // Prefer the more specific sponsorship answer before work authorization.
  const priority = ["sponsorship", "work_authorization"];
  for (const key of priority) {
    if (!(key in answers)) continue;
    if (aliases[key].some((alias) => normalizedDescriptor.includes(normalize(alias)))) return key;
  }
  for (const [key, keyAliases] of Object.entries(aliases)) {
    if (priority.includes(key) || !(key in answers)) continue;
    if (keyAliases.some((alias) => normalizedDescriptor.includes(normalize(alias)))) return key;
  }
  for (const key of Object.keys(answers)) {
    const words = normalize(key.replaceAll("_", " "));
    if (words.length >= 4 && normalizedDescriptor.includes(words)) return key;
  }
  return null;
}

function labelText(field) {
  const pieces = [];
  if (field.labels?.length) pieces.push(...[...field.labels].map((label) => label.textContent || ""));
  const wrapping = field.closest?.("label");
  if (wrapping) pieces.push(wrapping.textContent || "");
  const id = field.id;
  if (id) {
    const label = document.querySelector(`label[for="${cssEscape(id)}"]`);
    if (label) pieces.push(label.textContent || "");
  }
  const labelledBy = field.getAttribute?.("aria-labelledby");
  if (labelledBy) {
    for (const labelID of labelledBy.split(/\s+/)) {
      const label = document.getElementById(labelID);
      if (label) pieces.push(label.textContent || "");
    }
  }
  const container = field.closest?.("fieldset, [role='group'], [role='radiogroup'], .field, .form-field, .application-question");
  if (container) {
    const legend = container.querySelector?.("legend");
    if (legend) pieces.push(legend.textContent || "");
    const nearbyLabel = container.querySelector?.("label, [class*='label'], [class*='question']");
    if (nearbyLabel) pieces.push(nearbyLabel.textContent || "");
  }
  return [...new Set(pieces.map((value) => String(value).trim()).filter(Boolean))].join(" ");
}

function normalizeBoolean(value) {
  const text = normalize(value);
  if (!text) return null;
  if (["yes", "true", "y", "1"].includes(text) || text.startsWith("yes ")) return "yes";
  if (["no", "false", "n", "0"].includes(text) || text.startsWith("no ")) return "no";
  // Saved immigration values are commonly descriptive rather than literal yes/no.
  if (/\bh ?1b\b|\bh1 b\b|\bvisa\b|\bsponsorship\b|\btransfer\b/.test(text)) return "yes";
  if (/citizen|permanent resident|green card|no sponsorship|does not require sponsorship/.test(text)) return "no";
  return null;
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = {
    matchAnswerKeyForTest(descriptor, answers) {
      global.normalize = (value) => String(value || "").toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
      return matchAnswerKey(descriptor, answers);
    },
    normalizeBooleanForTest(value) {
      global.normalize = (item) => String(item || "").toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
      return normalizeBoolean(value);
    },
  };
}
