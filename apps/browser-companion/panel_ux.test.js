const assert = require("node:assert/strict");
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

console.log("browser companion panel UX tests passed");
