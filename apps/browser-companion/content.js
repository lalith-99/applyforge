const WEB_SOURCE = "applyforge-web-v1";
const COMPANION_SOURCE = "applyforge-companion-v1";
const HANDOFF_MESSAGE = "APPLYFORGE_COMPANION_HANDOFF";
const HANDOFF_ACK = "APPLYFORGE_COMPANION_ACK";
const PANEL_ID = "applyforge-companion-panel";

window.addEventListener("message", (event) => {
  if (event.source !== window || event.origin !== window.location.origin) return;
  const data = event.data;
  if (data?.source !== WEB_SOURCE || data?.type !== HANDOFF_MESSAGE || !data.payload) return;

  sendRuntime({ type: "STORE_HANDOFF", payload: data.payload })
    .then(() => {
      window.postMessage({ source: COMPANION_SOURCE, type: HANDOFF_ACK }, window.location.origin);
    })
    .catch(() => {
      // The web app times out and falls back to manual mode if storage fails.
    });
});

void initialize();

async function initialize() {
  const handoff = await safeRuntime({ type: "GET_HANDOFF" });
  if (!handoff) return;

  let currentOrigin;
  try {
    currentOrigin = window.location.origin;
  } catch {
    return;
  }
  if (currentOrigin !== handoff.destinationOrigin) return;

  const adapter = detectAdapter();
  renderPanel(adapter, handoff);

  if (handoff.execution) {
    setPanelStatus("A submission was already started. Checking the ATS outcome…", "working");
    await recoverAfterNavigation(adapter);
  }
}

function renderPanel(adapter, handoff) {
  if (document.getElementById(PANEL_ID)) return;
  const panel = document.createElement("aside");
  panel.id = PANEL_ID;
  panel.style.cssText = [
    "position:fixed",
    "right:20px",
    "bottom:20px",
    "z-index:2147483647",
    "width:320px",
    "padding:16px",
    "border-radius:12px",
    "background:#111827",
    "color:#fff",
    "box-shadow:0 12px 36px rgba(0,0,0,.35)",
    "font:13px/1.45 -apple-system,BlinkMacSystemFont,Segoe UI,sans-serif",
  ].join(";");

  const title = document.createElement("div");
  title.textContent = "ApplyForge Companion";
  title.style.cssText = "font-size:15px;font-weight:700;margin-bottom:4px";

  const meta = document.createElement("div");
  meta.textContent = `${adapterLabel(adapter)} · approved intent ${String(handoff.intentId).slice(0, 8)}…`;
  meta.style.cssText = "opacity:.72;font-size:11px;margin-bottom:10px";

  const status = document.createElement("div");
  status.dataset.applyforgeStatus = "true";
  status.textContent = adapter === "generic"
    ? "This site is not yet auto-submit supported. ApplyForge can fill known fields and stop before submission."
    : "Ready to fill the exact approved application package.";
  status.style.cssText = "margin-bottom:12px";

  const button = document.createElement("button");
  button.type = "button";
  button.dataset.applyforgeAction = "true";
  button.textContent = adapter === "generic" ? "Fill approved answers" : "Fill & submit approved application";
  button.style.cssText = "width:100%;border:0;border-radius:8px;padding:10px 12px;background:#fff;color:#111827;font-weight:700;cursor:pointer";
  button.addEventListener("click", () => void runApprovedApplication(adapter));

  const clear = document.createElement("button");
  clear.type = "button";
  clear.textContent = "Cancel companion handoff";
  clear.style.cssText = "width:100%;margin-top:8px;border:1px solid rgba(255,255,255,.25);border-radius:8px;padding:8px;background:transparent;color:#fff;cursor:pointer";
  clear.addEventListener("click", async () => {
    await safeRuntime({ type: "CLEAR_HANDOFF" });
    panel.remove();
  });

  panel.append(title, meta, status, button, clear);
  document.documentElement.appendChild(panel);
}

async function runApprovedApplication(adapter) {
  disableAction(true);
  try {
    setPanelStatus("Loading the immutable approved package…", "working");
    const [bundle, handoff] = await Promise.all([
      sendRuntime({ type: "GET_BUNDLE" }),
      sendRuntime({ type: "GET_HANDOFF" }),
    ]);
    if (!handoff) throw new Error("The ApplyForge handoff expired.");

    const answers = flattenAnswers(bundle.package?.answers_json || {});
    const result = fillKnownFields(answers);
    const resumeUploaded = uploadResume(handoff.resumePdfBase64, handoff.resumeFilename || "applyforge-resume.pdf");

    const barriers = detectBarriers();
    const unresolved = unresolvedRequiredFields();
    if (barriers.length || unresolved.length) {
      const messages = [];
      if (barriers.length) messages.push(`Needs attention: ${barriers.join(", ")}.`);
      if (unresolved.length) messages.push(`Complete ${unresolved.length} required field${unresolved.length === 1 ? "" : "s"} that ApplyForge could not answer safely.`);
      setPanelStatus(`${messages.join(" ")} Filled ${result.filled} known fields${resumeUploaded ? " and attached the approved resume" : ""}.`, "attention");
      return;
    }

    if (adapter === "generic") {
      setPanelStatus(`Filled ${result.filled} known fields${resumeUploaded ? " and attached the approved resume" : ""}. Final submit is manual on this unsupported ATS; ApplyForge will not guess.`, "attention");
      return;
    }

    const submit = findFinalSubmitButton();
    if (!submit) {
      setPanelStatus("Known fields are filled, but ApplyForge could not identify a safe final submit control. Submit manually or leave this application pending.", "attention");
      return;
    }

    setPanelStatus("Claiming the one-time fenced submission lease…", "working");
    const claimed = await sendRuntime({ type: "CLAIM" });
    if (!claimed?.lease_generation) throw new Error("ApplyForge did not return a submission lease.");

    setPanelStatus("Crossing the final submission boundary…", "working");
    await sendRuntime({ type: "BEGIN" });

    submit.click();
    setPanelStatus("Submitted. Verifying the ATS confirmation before marking this application Applied…", "working");

    const signal = await waitForSuccessSignal(20000);
    if (signal) {
      await confirmSuccess(adapter, signal);
      setPanelStatus("Application confirmed and recorded as Applied in ApplyForge.", "success");
      disableAction(true);
      return;
    }

    await sendRuntime({
      type: "UNCERTAIN",
      message: "final submit was triggered but no reliable ATS success signal was observed within 20 seconds",
    });
    setPanelStatus("The final submit was triggered, but the ATS outcome could not be verified. ApplyForge marked this as Submission uncertain and will not retry automatically.", "attention");
  } catch (error) {
    setPanelStatus(error instanceof Error ? error.message : String(error), "error");
  } finally {
    const handoff = await safeRuntime({ type: "GET_HANDOFF" });
    if (handoff && !handoff.execution) disableAction(false);
  }
}

async function recoverAfterNavigation(adapter) {
  const signal = await waitForSuccessSignal(8000);
  if (signal) {
    try {
      await confirmSuccess(adapter, signal);
      setPanelStatus("Application confirmation recovered after navigation and recorded as Applied.", "success");
      disableAction(true);
      return;
    } catch (error) {
      setPanelStatus(error instanceof Error ? error.message : String(error), "error");
      return;
    }
  }

  try {
    await sendRuntime({
      type: "UNCERTAIN",
      message: "browser navigated after final submit but the destination page did not expose a reliable success signal",
    });
    setPanelStatus("ApplyForge could not verify whether the ATS accepted the submission. It is marked Submission uncertain and will not be retried automatically.", "attention");
  } catch (error) {
    setPanelStatus(error instanceof Error ? error.message : String(error), "error");
  }
}

async function confirmSuccess(adapter, signal) {
  return sendRuntime({
    type: "CONFIRM",
    receipt: {
      adapter,
      signal,
      url: window.location.href,
      page_title: document.title,
      confirmed_at: new Date().toISOString(),
    },
  });
}

function fillKnownFields(answers) {
  let filled = 0;
  for (const field of document.querySelectorAll("input, textarea, select")) {
    if (!(field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement || field instanceof HTMLSelectElement)) continue;
    if (!isVisible(field) || field.disabled) continue;
    if (field instanceof HTMLInputElement && ["hidden", "submit", "button", "reset", "file", "password", "search"].includes(field.type)) continue;

    const descriptor = normalize(`${field.name || ""} ${field.id || ""} ${field.getAttribute("aria-label") || ""} ${field.getAttribute("placeholder") || ""} ${labelText(field)}`);
    const key = matchAnswerKey(descriptor, answers);
    if (!key) continue;
    const value = answers[key];
    if (value == null || String(value).trim() === "") continue;

    if (field instanceof HTMLSelectElement) {
      if (setSelect(field, String(value))) filled += 1;
      continue;
    }
    if (field instanceof HTMLInputElement && (field.type === "radio" || field.type === "checkbox")) {
      if (setChoice(field, String(value))) filled += 1;
      continue;
    }
    if (!String(field.value || "").trim()) {
      setNativeValue(field, String(value));
      filled += 1;
    }
  }
  return { filled };
}

function flattenAnswers(snapshot) {
  const result = {};
  for (const [key, value] of Object.entries(snapshot || {})) {
    if (key === "common_answers" && value && typeof value === "object") {
      for (const [commonKey, commonValue] of Object.entries(value)) result[normalizeKey(commonKey)] = commonValue;
    } else {
      result[normalizeKey(key)] = value;
    }
  }

  const fullName = String(result.full_name || "").trim();
  if (fullName) {
    const parts = fullName.split(/\s+/);
    result.first_name ||= parts[0];
    result.last_name ||= parts.length > 1 ? parts.slice(1).join(" ") : parts[0];
  }
  return result;
}

const FIELD_ALIASES = {
  first_name: ["first name", "firstname", "given name"],
  last_name: ["last name", "lastname", "surname", "family name"],
  full_name: ["full name", "name"],
  email: ["email", "email address"],
  phone: ["phone", "phone number", "mobile", "telephone"],
  location: ["current location", "location", "city"],
  desired_location: ["desired location", "preferred location"],
  linkedin_url: ["linkedin", "linkedin url", "linkedin profile"],
  github_url: ["github", "github url", "github profile"],
  portfolio_url: ["portfolio", "website", "personal website"],
  salary_expectation: ["salary", "compensation", "desired salary", "salary expectation"],
  notice_period: ["notice period", "start date", "available to start", "availability"],
  work_authorization: ["work authorization", "authorized to work", "legally authorized", "eligible to work"],
  sponsorship: ["sponsorship", "visa sponsorship", "require sponsorship", "immigration sponsorship"],
};

function matchAnswerKey(descriptor, answers) {
  for (const [key, aliases] of Object.entries(FIELD_ALIASES)) {
    if (!(key in answers)) continue;
    if (aliases.some((alias) => descriptor.includes(normalize(alias)))) return key;
  }
  for (const key of Object.keys(answers)) {
    const words = normalize(key.replaceAll("_", " "));
    if (words.length >= 4 && descriptor.includes(words)) return key;
  }
  return null;
}

function uploadResume(base64, filename) {
  if (!base64) return false;
  const input = [...document.querySelectorAll('input[type="file"]')].find((candidate) => isVisible(candidate) || candidate.offsetParent !== null);
  if (!(input instanceof HTMLInputElement)) return false;
  try {
    const binary = atob(base64);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i += 1) bytes[i] = binary.charCodeAt(i);
    const file = new File([bytes], filename, { type: "application/pdf" });
    const transfer = new DataTransfer();
    transfer.items.add(file);
    input.files = transfer.files;
    input.dispatchEvent(new Event("input", { bubbles: true }));
    input.dispatchEvent(new Event("change", { bubbles: true }));
    return true;
  } catch {
    return false;
  }
}

function detectBarriers() {
  const barriers = [];
  if (document.querySelector('iframe[src*="recaptcha" i], iframe[src*="hcaptcha" i], [class*="captcha" i], [data-sitekey]')) barriers.push("CAPTCHA");
  if ([...document.querySelectorAll('input[type="password"]')].some(isVisible)) barriers.push("login/account step");
  return barriers;
}

function unresolvedRequiredFields() {
  const unresolved = [];
  for (const field of document.querySelectorAll("input[required], textarea[required], select[required], [aria-required='true']")) {
    if (!(field instanceof HTMLInputElement || field instanceof HTMLTextAreaElement || field instanceof HTMLSelectElement)) continue;
    if (!isVisible(field) || field.disabled) continue;
    if (field instanceof HTMLInputElement && ["hidden", "submit", "button"].includes(field.type)) continue;
    if (field instanceof HTMLInputElement && (field.type === "radio" || field.type === "checkbox")) {
      if (field.name && document.querySelector(`input[name="${cssEscape(field.name)}"]:checked`)) continue;
      if (field.checked) continue;
    } else if (String(field.value || "").trim()) {
      continue;
    }
    unresolved.push(labelText(field) || field.getAttribute("aria-label") || field.name || field.id || "required field");
  }
  return [...new Set(unresolved)];
}

function detectAdapter() {
  const host = window.location.hostname.toLowerCase();
  if (host.includes("greenhouse.io")) return "greenhouse";
  if (host === "jobs.lever.co" || host.endsWith(".lever.co")) return "lever";
  return "generic";
}

function adapterLabel(adapter) {
  if (adapter === "greenhouse") return "Greenhouse";
  if (adapter === "lever") return "Lever";
  return "Generic ATS";
}

function findFinalSubmitButton() {
  const controls = [...document.querySelectorAll('button[type="submit"], input[type="submit"], button')];
  return controls.find((control) => {
    if (!(control instanceof HTMLElement) || !isVisible(control)) return false;
    const text = normalize(control instanceof HTMLInputElement ? control.value : control.textContent || "");
    if (!text || text.includes("save") || text.includes("next") || text.includes("continue")) return false;
    return text.includes("submit") || text.includes("apply") || text.includes("send application");
  }) || null;
}

async function waitForSuccessSignal(timeoutMs) {
  const started = Date.now();
  while (Date.now() - started < timeoutMs) {
    const signal = successSignal();
    if (signal) return signal;
    await sleep(500);
  }
  return null;
}

function successSignal() {
  const url = window.location.href.toLowerCase();
  if (/thank|confirmation|submitted|success/.test(url)) return "success-url";
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

function setSelect(select, desired) {
  const normalizedDesired = normalize(desired);
  const yesNo = normalizeBoolean(desired);
  const option = [...select.options].find((candidate) => {
    const optionText = normalize(`${candidate.text} ${candidate.value}`);
    return optionText === normalizedDesired || optionText.includes(normalizedDesired) || (yesNo && normalizeBoolean(optionText) === yesNo);
  });
  if (!option) return false;
  select.value = option.value;
  select.dispatchEvent(new Event("input", { bubbles: true }));
  select.dispatchEvent(new Event("change", { bubbles: true }));
  return true;
}

function setChoice(input, desired) {
  const desiredBoolean = normalizeBoolean(desired);
  const descriptor = normalize(`${input.value || ""} ${labelText(input)}`);
  const inputBoolean = normalizeBoolean(descriptor);
  if (input.type === "checkbox") {
    if (desiredBoolean === "yes" && !input.checked) input.click();
    else if (desiredBoolean === "no" && input.checked) input.click();
    else return desiredBoolean != null;
    return true;
  }
  if (input.type === "radio" && (normalize(desired) === normalize(input.value) || descriptor.includes(normalize(desired)) || (desiredBoolean && inputBoolean === desiredBoolean))) {
    if (!input.checked) input.click();
    return true;
  }
  return false;
}

function setNativeValue(field, value) {
  const prototype = field instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  const setter = Object.getOwnPropertyDescriptor(prototype, "value")?.set;
  if (setter) setter.call(field, value);
  else field.value = value;
  field.dispatchEvent(new Event("input", { bubbles: true }));
  field.dispatchEvent(new Event("change", { bubbles: true }));
  field.dispatchEvent(new Event("blur", { bubbles: true }));
}

function labelText(field) {
  if (field.labels?.length) return [...field.labels].map((label) => label.textContent || "").join(" ");
  const wrapping = field.closest("label");
  if (wrapping) return wrapping.textContent || "";
  const id = field.id;
  if (id) {
    const label = document.querySelector(`label[for="${cssEscape(id)}"]`);
    if (label) return label.textContent || "";
  }
  return "";
}

function isVisible(element) {
  if (!(element instanceof Element)) return false;
  const style = window.getComputedStyle(element);
  if (style.display === "none" || style.visibility === "hidden" || Number(style.opacity) === 0) return false;
  const rect = element.getBoundingClientRect();
  return rect.width > 0 && rect.height > 0;
}

function normalize(value) {
  return String(value || "").toLowerCase().replace(/[^a-z0-9]+/g, " ").trim();
}

function normalizeKey(value) {
  return normalize(value).replaceAll(" ", "_");
}

function normalizeBoolean(value) {
  const text = normalize(value);
  if (["yes", "true", "y", "1"].includes(text) || text.startsWith("yes ")) return "yes";
  if (["no", "false", "n", "0"].includes(text) || text.startsWith("no ")) return "no";
  return null;
}

function cssEscape(value) {
  return globalThis.CSS?.escape ? CSS.escape(value) : String(value).replace(/["\\]/g, "\\$&");
}

function setPanelStatus(message, kind) {
  const status = document.querySelector(`#${PANEL_ID} [data-applyforge-status="true"]`);
  if (!(status instanceof HTMLElement)) return;
  status.textContent = message;
  status.style.color = kind === "error" ? "#fecaca" : kind === "success" ? "#bbf7d0" : kind === "attention" ? "#fde68a" : "#fff";
}

function disableAction(disabled) {
  const button = document.querySelector(`#${PANEL_ID} [data-applyforge-action="true"]`);
  if (!(button instanceof HTMLButtonElement)) return;
  button.disabled = disabled;
  button.style.opacity = disabled ? ".55" : "1";
  button.style.cursor = disabled ? "default" : "pointer";
}

function sleep(ms) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function sendRuntime(message) {
  const response = await chrome.runtime.sendMessage(message);
  if (!response?.ok) throw new Error(response?.error || "ApplyForge companion request failed");
  return response.result;
}

async function safeRuntime(message) {
  try {
    return await sendRuntime(message);
  } catch {
    return null;
  }
}
