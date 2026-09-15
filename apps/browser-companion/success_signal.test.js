const assert = require("node:assert/strict");
const { hasReliableSuccessURL } = require("./success_signal.js");

for (const url of [
  "https://boards.greenhouse.io/acme/thank-you",
  "https://jobs.lever.co/acme/confirmation",
  "https://jobs.lever.co/acme/application-submitted",
  "https://boards.greenhouse.io/acme/success/",
]) {
  assert.equal(hasReliableSuccessURL(url), true, `expected reliable success URL: ${url}`);
}

for (const url of [
  "https://jobs.lever.co/acme/customer-success-engineer",
  "https://boards.greenhouse.io/acme/jobs/123?team=customer-success",
  "https://boards.greenhouse.io/acme/submitted-applications-engineer",
  "https://example.com/jobs/confirmation-platform-engineer",
  "not a url",
]) {
  assert.equal(hasReliableSuccessURL(url), false, `expected ordinary URL to remain unconfirmed: ${url}`);
}

console.log("success URL signal tests passed");
