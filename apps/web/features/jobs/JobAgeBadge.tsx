export function JobAgeBadge({ postedAt, firstSeenAt }: { postedAt: string | null; firstSeenAt: string }) {
  const reference = postedAt ?? firstSeenAt;
  const label = formatRelativeAge(reference);
  return (
    <span className="rounded-full bg-black/5 px-2 py-0.5 dark:bg-white/10" title={postedAt ? "Posted by employer" : "First discovered by ApplyForge"}>
      {label}
    </span>
  );
}

function formatRelativeAge(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime();
  const minutes = Math.floor(diffMs / 60000);
  if (minutes < 60) return `${Math.max(minutes, 0)}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days === 1) return "Yesterday";
  if (days < 7) return `${days}d ago`;
  return `${Math.floor(days / 7)}w ago`;
}
