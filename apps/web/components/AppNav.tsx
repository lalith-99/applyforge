import Link from "next/link";

const NAV_LINKS = [
  { href: "/dashboard", label: "Dashboard" },
  { href: "/jobs", label: "Jobs" },
  { href: "/applications", label: "Applications" },
  { href: "/resume", label: "Resume" },
];

export function AppNav() {
  return (
    <nav className="flex items-center gap-6 border-b border-black/10 px-8 py-4 text-sm dark:border-white/15">
      <span className="font-semibold">ApplyForge</span>
      {NAV_LINKS.map((link) => (
        <Link key={link.href} href={link.href} className="text-black/70 hover:text-black dark:text-white/70 dark:hover:text-white">
          {link.label}
        </Link>
      ))}
    </nav>
  );
}
