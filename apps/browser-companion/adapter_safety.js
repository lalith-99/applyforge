// Keep final-submit support scoped to ATS hosts whose submission flow ApplyForge
// explicitly understands. Employer-controlled subdomains that merely contain an
// ATS vendor name must remain generic/manual until they have a dedicated adapter.
function detectSupportedAdapterHost(hostname) {
  const host = String(hostname || "").toLowerCase().replace(/\.$/, "");
  if (host === "greenhouse.io" || host.endsWith(".greenhouse.io")) return "greenhouse";
  if (host === "lever.co" || host.endsWith(".lever.co")) return "lever";
  return "generic";
}

if (typeof module !== "undefined" && module.exports) {
  module.exports = { detectSupportedAdapterHost };
}
