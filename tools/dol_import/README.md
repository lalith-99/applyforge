# DOL immigration evidence importer

This utility aggregates official U.S. Department of Labor OFLC disclosure
workbooks into employer-level historical evidence for ApplyForge.

It intentionally does **not** store raw case rows. LCA data is filtered to
H-1B when a `VISA_CLASS` column is present, then LCA/PERM cases are grouped by
normalized employer name.

Historical DOL evidence means an employer has used the relevant labor
application/certification pathway. It is **not** proof that a particular
current job will sponsor. ApplyForge's role-level job-description evidence
always takes precedence.

## Usage

```bash
python -m venv .venv
. .venv/bin/activate
pip install -r tools/dol_import/requirements.txt

python tools/dol_import/import_dol.py \
  --lca-url "<official DOL LCA XLSX URL>" \
  --perm-url "<official DOL PERM XLSX URL>" \
  --fiscal-year 2026 \
  --source-release FY2026_Q3 \
  --api-base http://localhost:8080/api/v1 \
  --admin-token "$ADMIN_SYNC_TOKEN"
```

Use `--dry-run` first to inspect aggregate counts without calling ApplyForge.

The current DOL release URLs should be copied from the OFLC Performance Data
page rather than hard-coded here, because quarterly filenames/releases change.
