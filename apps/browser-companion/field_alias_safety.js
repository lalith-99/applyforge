// Tighten broad reusable-answer aliases after the field driver loads.
// Bare tokens such as "name", "location", and "city" are unsafe on employer
// forms because substring matching can target unrelated questions such as
// company name, job location, preferred location, relocation, or school city.

const UNSAFE_EXACT_ALIASES = {
  full_name: new Set(["name"]),
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

if (typeof APPROVED_FIELD_ALIASES !== "undefined") {
  removeUnsafeExactAliases(APPROVED_FIELD_ALIASES);
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = { removeUnsafeExactAliases };
}
