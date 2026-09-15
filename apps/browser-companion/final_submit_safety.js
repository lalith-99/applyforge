// A supported ATS may contain multiple submit-like controls across its flow.
// Only controls whose visible label explicitly describes final application
// submission are eligible to cross ApplyForge's approved-submit boundary.
function isExplicitFinalSubmitLabel(rawText) {
  const text = normalize(String(rawText || ""));
  if (!text) return false;
  if (text.includes("save") || text.includes("next") || text.includes("continue")) return false;

  return text === "submit"
    || text === "submit application"
    || text === "submit my application"
    || text === "send application"
    || text === "send my application";
}

// Loaded after content.js so this intentionally replaces the permissive
// fallback that accepted any button containing the word "apply".
function findFinalSubmitButton() {
  const controls = [...document.querySelectorAll('button[type="submit"], input[type="submit"], button')];
  return controls.find((control) => {
    if (!(control instanceof HTMLElement) || !isVisible(control)) return false;
    const text = control instanceof HTMLInputElement ? control.value : control.textContent || "";
    return isExplicitFinalSubmitLabel(text);
  }) || null;
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = { isExplicitFinalSubmitLabel };
}
