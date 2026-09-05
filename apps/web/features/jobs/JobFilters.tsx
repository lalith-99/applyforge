"use client";

export interface JobFiltersState {
  search: string;
  remoteType: string;
  employmentType: string;
  postedWithin: string;
  sort: string;
}

export const DEFAULT_FILTERS: JobFiltersState = {
  search: "",
  remoteType: "",
  employmentType: "",
  postedWithin: "",
  sort: "newest",
};

export function JobFilters({
  value,
  onChange,
}: {
  value: JobFiltersState;
  onChange: (next: JobFiltersState) => void;
}) {
  return (
    <div className="flex flex-wrap items-center gap-3">
      <input
        placeholder="Search title or company"
        value={value.search}
        onChange={(e) => onChange({ ...value, search: e.target.value })}
        className="rounded-md border border-black/10 bg-transparent px-3 py-2 text-sm dark:border-white/15"
      />
      <select
        value={value.remoteType}
        onChange={(e) => onChange({ ...value, remoteType: e.target.value })}
        className="rounded-md border border-black/10 bg-transparent px-3 py-2 text-sm dark:border-white/15"
      >
        <option value="">Any work arrangement</option>
        <option value="remote">Remote</option>
        <option value="hybrid">Hybrid</option>
        <option value="onsite">Onsite</option>
      </select>
      <select
        value={value.employmentType}
        onChange={(e) => onChange({ ...value, employmentType: e.target.value })}
        className="rounded-md border border-black/10 bg-transparent px-3 py-2 text-sm dark:border-white/15"
      >
        <option value="">Any employment type</option>
        <option value="FullTime">Full-time</option>
        <option value="Contract">Contract</option>
        <option value="Internship">Internship</option>
      </select>
      <select
        value={value.postedWithin}
        onChange={(e) => onChange({ ...value, postedWithin: e.target.value })}
        className="rounded-md border border-black/10 bg-transparent px-3 py-2 text-sm dark:border-white/15"
      >
        <option value="">Any time posted</option>
        <option value="1h">Last 1 hour</option>
        <option value="3h">Last 3 hours</option>
        <option value="6h">Last 6 hours</option>
        <option value="12h">Last 12 hours</option>
        <option value="24h">Last 24 hours</option>
        <option value="72h">Last 3 days</option>
        <option value="168h">Last 7 days</option>
      </select>
      <select
        value={value.sort}
        onChange={(e) => onChange({ ...value, sort: e.target.value })}
        className="rounded-md border border-black/10 bg-transparent px-3 py-2 text-sm dark:border-white/15"
      >
        <option value="newest">Newest</option>
        <option value="salary">Highest Salary</option>
      </select>
    </div>
  );
}
