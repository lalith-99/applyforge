const assert = require("node:assert/strict");
const fs = require("node:fs");
const path = require("node:path");
const {
  appendUnresolvedDetails,
  compactFieldLabel,
  normalizePanelText,
} = require("./panel_ux.js");

assert.equal(normalizePanelText("  Please Select… "), "please select");
assert.equal(compactFieldLabel("LinkedIn Profile"), "LinkedIn Profile");
assert.equal(
  appendUnresolvedDetails(
    "Complete 2 required fields that ApplyForge could not answer safely.",
    ["Work authorization", "Visa sponsorship"],
  ),
  "Complete 2 required fields that ApplyForge could not answer safely.\nNeeds your input: Work authorization; Visa sponsorship.",
);
assert.equal(
  appendUnresolvedDetails("Filled 8 known fields.", ["Work authorization"]),
  "Filled 8 known fields.",
);

// panel_ux.js is loaded after required_field_safety.js in the extension. It must
// never redeclare the canonical unresolvedRequiredFields() submit guard, or the
// later declaration silently replaces the stricter pre-submit safety checks.
const panelSource = fs.readFileSync(path.join(__dirname, "panel_ux.js"), "utf8");
assert.doesNotMatch(panelSource, /function\s+unresolvedRequiredFields\s*\(/);
assert.match(panelSource, /function\s+panelUnresolvedRequiredFields\s*\(/);

console.log("browser companion panel UX tests passed");
