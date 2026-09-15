const assert = require("node:assert/strict");
const { matchAnswerKeyForTest, normalizeBooleanForTest } = require("./field_matching.js");

const answers = {
  work_authorization: "H-1B",
  sponsorship: "H-1B transfer required",
  linkedin_url: "https://www.linkedin.com/in/example",
};

assert.equal(matchAnswerKeyForTest("Are you legally authorized to work in the United States?", answers), "work_authorization");
assert.equal(matchAnswerKeyForTest("Will you now or in the future require sponsorship for employment visa status?", answers), "sponsorship");
assert.equal(matchAnswerKeyForTest("LinkedIn Profile", answers), "linkedin_url");
assert.equal(normalizeBooleanForTest("H-1B"), "yes");
assert.equal(normalizeBooleanForTest("H-1B transfer required"), "yes");
assert.equal(normalizeBooleanForTest("No"), "no");

console.log("field matching tests passed");
