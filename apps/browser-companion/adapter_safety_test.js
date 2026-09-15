const assert = require("node:assert/strict");
const { detectSupportedAdapterHost } = require("./adapter_safety.js");

assert.equal(detectSupportedAdapterHost("boards.greenhouse.io"), "greenhouse");
assert.equal(detectSupportedAdapterHost("job-boards.greenhouse.io"), "greenhouse");
assert.equal(detectSupportedAdapterHost("jobs.lever.co"), "lever");

// A hostname containing a vendor name is not evidence that ApplyForge knows
// that employer-controlled site's final-submit semantics.
assert.equal(detectSupportedAdapterHost("greenhouse.io.example.com"), "generic");
assert.equal(detectSupportedAdapterHost("careers-greenhouse.io.example.com"), "generic");
assert.equal(detectSupportedAdapterHost("lever.co.example.com"), "generic");
assert.equal(detectSupportedAdapterHost("evilgreenhouse.io"), "generic");
assert.equal(detectSupportedAdapterHost("notlever.co"), "generic");
assert.equal(detectSupportedAdapterHost("careers.example.com"), "generic");

console.log("adapter host safety tests passed");
