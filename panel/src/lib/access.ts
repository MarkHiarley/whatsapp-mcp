function parseList(value?: string): string[] {
  return (value ?? "")
    .split(",")
    .map((entry) => entry.trim().toLowerCase())
    .filter(Boolean);
}

/**
 * Allowed = admins + PANEL_ALLOWED_EMAILS. If neither is configured, allow
 * (bootstrap); once any list exists, unknown emails are denied.
 */
export function isEmailAllowed(email: string): boolean {
  const allowed = [
    ...new Set([
      ...parseList(process.env.PRIMER_ADMIN),
      ...parseList(process.env.PANEL_ADMIN_EMAILS),
      ...parseList(process.env.PANEL_ALLOWED_EMAILS),
    ]),
  ];
  if (allowed.length === 0) {
    return true;
  }
  return allowed.includes(email.trim().toLowerCase());
}
