// URL success evidence must be narrow enough that ordinary job URLs such as
// /customer-success-engineer cannot be mistaken for a submission receipt.
function hasReliableSuccessURL(rawURL) {
  let parsed;
  try {
    parsed = new URL(String(rawURL || ""));
  } catch {
    return false;
  }

  const markers = new Set([
    "thank-you",
    "thank_you",
    "thanks",
    "confirmation",
    "submitted",
    "success",
    "application-submitted",
    "application_submitted",
  ]);
  return parsed.pathname
    .toLowerCase()
    .split("/")
    .filter(Boolean)
    .some((segment) => markers.has(segment));
}

// Loaded after content.js so this intentionally replaces its broad URL check
// while retaining the conservative ATS confirmation text phrases.
function successSignal() {
  if (hasReliableSuccessURL(window.location.href)) return "success-url";
  const text = normalize(document.body?.innerText || "");
  const phrases = [
    "thank you for applying",
    "thanks for applying",
    "application submitted",
    "application has been submitted",
    "application received",
    "we have received your application",
    "we received your application",
  ];
  const phrase = phrases.find((candidate) => text.includes(candidate));
  return phrase ? `success-text:${phrase}` : null;
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = { hasReliableSuccessURL };
}
