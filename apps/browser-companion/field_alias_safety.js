// Tighten broad reusable-answer aliases after the field driver loads.
// A bare "name" token is unsafe on employer forms because it can match fields
// such as company name, preferred name, reference name, or school name.

function removeUnsafeExactAliases(aliasMap) {
  if (!aliasMap || typeof aliasMap !== "object") return aliasMap;

  if (Array.isArray(aliasMap.full_name)) {
    aliasMap.full_name = aliasMap.full_name.filter((alias) => normalizeAlias(alias) !== "name");
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
