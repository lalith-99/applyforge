// Tighten broad reusable-answer aliases after the field driver loads.
// Bare tokens such as "name", "location", "city", and "phone" are unsafe on
// employer forms because substring matching can target unrelated questions
// such as company name, job location, preferred location, relocation, school
// city, or a phone country/area-code field that must not receive a full number.

const UNSAFE_EXACT_ALIASES = {
  full_name: new Set(["name"]),
  phone: new Set(["phone"]),
  location: new Set(["location", "city"]),
};

function removeUnsafeExactAliases(aliasMap) {
  if (!aliasMap || typeof aliasMap !== "object") return aliasMap;

  for (const [key, unsafeAliases] of Object.entries(UNSAFE_EXACT_ALIASES)) {
    if (!Array.isArray(aliasMap[key])) continue;
    aliasMap[key] = aliasMap[key].filter((alias) => !unsafeAliases.has(normalizeAlias(alias)));
  }

  return aliasMap;
}

function normalizeAlias(value) {
  return String(value || "").toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
}

// content.js and resume_upload.js are classic content scripts in the same
// isolated world, so their top-level lexical alias maps are visible here.
// Harden both drivers: runApprovedApplication() uses FIELD_ALIASES while the
// richer resume/combobox driver uses APPROVED_FIELD_ALIASES.
if (typeof FIELD_ALIASES !== "undefined") {
  removeUnsafeExactAliases(FIELD_ALIASES);
}

if (typeof APPROVED_FIELD_ALIASES !== "undefined") {
  removeUnsafeExactAliases(APPROVED_FIELD_ALIASES);
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = { removeUnsafeExactAliases };
}
