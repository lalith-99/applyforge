const HANDOFF_KEY = "activeHandoff";
const WORKER_KEY = "workerId";

chrome.runtime.onMessage.addListener((message, _sender, sendResponse) => {
  handleMessage(message)
    .then((result) => sendResponse({ ok: true, result }))
    .catch((error) => sendResponse({ ok: false, error: error instanceof Error ? error.message : String(error) }));
  return true;
});

async function handleMessage(message) {
  switch (message?.type) {
    case "STORE_HANDOFF":
      return storeHandoff(message.payload);
    case "GET_HANDOFF":
      return getHandoff();
    case "GET_WORKER_ID":
      return getWorkerId();
    case "GET_BUNDLE":
      return companionRequest("", { method: "GET" });
    case "CLAIM": {
      const workerId = await getWorkerId();
      const claimed = await companionRequest("/claim", {
        method: "POST",
        body: { worker_id: workerId },
      });
      const handoff = await getHandoff();
      await chrome.storage.session.set({
        [HANDOFF_KEY]: {
          ...handoff,
          execution: {
            workerId,
            generation: claimed.lease_generation,
          },
        },
      });
      return claimed;
    }
    case "BEGIN": {
      const handoff = await requireHandoff();
      const execution = requireExecution(handoff);
      return companionRequest("/begin", {
        method: "POST",
        body: {
          worker_id: execution.workerId,
          lease_generation: execution.generation,
        },
      });
    }
    case "CONFIRM": {
      const handoff = await requireHandoff();
      const execution = requireExecution(handoff);
      const result = await companionRequest("/confirm", {
        method: "POST",
        body: {
          worker_id: execution.workerId,
          lease_generation: execution.generation,
          receipt: message.receipt,
        },
      });
      await clearHandoff();
      return result;
    }
    case "UNCERTAIN": {
      const handoff = await requireHandoff();
      const execution = requireExecution(handoff);
      const result = await companionRequest("/uncertain", {
        method: "POST",
        body: {
          worker_id: execution.workerId,
          lease_generation: execution.generation,
          message: String(message.message || "submission outcome could not be verified"),
        },
      });
      await clearHandoff();
      return result;
    }
    case "CLEAR_HANDOFF":
      await clearHandoff();
      return true;
    default:
      throw new Error("Unsupported companion message");
  }
}

async function storeHandoff(payload) {
  if (!payload?.intentId || !payload?.token || !payload?.apiBaseUrl || !payload?.destinationOrigin) {
    throw new Error("Invalid ApplyForge companion handoff");
  }
  const expiresAt = Date.parse(payload.expiresAt);
  if (!Number.isFinite(expiresAt) || expiresAt <= Date.now()) {
    throw new Error("ApplyForge companion handoff already expired");
  }
  const destinationOrigin = new URL(payload.destinationUrl).origin;
  if (destinationOrigin !== payload.destinationOrigin) {
    throw new Error("ApplyForge destination origin mismatch");
  }
  const handoff = {
    ...payload,
    apiBaseUrl: String(payload.apiBaseUrl).replace(/\/$/, ""),
    execution: null,
  };
  await chrome.storage.session.set({ [HANDOFF_KEY]: handoff });
  return { intentId: handoff.intentId, expiresAt: handoff.expiresAt };
}

async function getHandoff() {
  const stored = await chrome.storage.session.get(HANDOFF_KEY);
  const handoff = stored[HANDOFF_KEY] || null;
  if (!handoff) return null;
  if (Date.parse(handoff.expiresAt) <= Date.now()) {
    await clearHandoff();
    return null;
  }
  return handoff;
}

async function requireHandoff() {
  const handoff = await getHandoff();
  if (!handoff) throw new Error("No active ApplyForge handoff");
  return handoff;
}

function requireExecution(handoff) {
  if (!handoff.execution?.workerId || !handoff.execution?.generation) {
    throw new Error("ApplyForge submission has not been claimed");
  }
  return handoff.execution;
}

async function getWorkerId() {
  const stored = await chrome.storage.local.get(WORKER_KEY);
  if (stored[WORKER_KEY]) return stored[WORKER_KEY];
  const workerId = `browser-${crypto.randomUUID()}`;
  await chrome.storage.local.set({ [WORKER_KEY]: workerId });
  return workerId;
}

async function companionRequest(path, options) {
  const handoff = await requireHandoff();
  const url = `${handoff.apiBaseUrl}/companion/submissions/${encodeURIComponent(handoff.intentId)}${path}`;
  const response = await fetch(url, {
    method: options.method,
    headers: {
      "Content-Type": "application/json",
      "X-ApplyForge-Companion-Token": handoff.token,
    },
    body: options.body ? JSON.stringify(options.body) : undefined,
    cache: "no-store",
  });
  const text = await response.text();
  let body = null;
  if (text) {
    try {
      body = JSON.parse(text);
    } catch {
      body = { error: text };
    }
  }
  if (!response.ok) {
    throw new Error(body?.error || `ApplyForge API returned ${response.status}`);
  }
  return body;
}

async function clearHandoff() {
  await chrome.storage.session.remove(HANDOFF_KEY);
}
