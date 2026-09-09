package jobs

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	defaultFreeSourceDirectoryBaseURL = "https://raw.githubusercontent.com/kalil0321/ats-scrapers/main/ats-companies"
	defaultFreeSourceRefreshAfter     = 7 * 24 * time.Hour
)

type freeSourceInventory struct {
	Name          string
	SourceType    string
	AutoMonitor   bool
	RegistryToken string
}

var freeSourceInventories = []freeSourceInventory{
	{Name: "greenhouse", SourceType: "GREENHOUSE", AutoMonitor: true},
	{Name: "lever", SourceType: "LEVER", AutoMonitor: true},
	{Name: "ashby", SourceType: "ASHBY", AutoMonitor: true},
	{Name: "workday", SourceType: "WORKDAY", AutoMonitor: true},
	// SmartRecruiters' public directory is useful for discovery, but brand-name
	// collisions are more common there. Keep it registry-only until a later
	// verifier proves that the tenant belongs to the sponsor company.
	{Name: "smartrecruiters", SourceType: "SMARTRECRUITERS", AutoMonitor: false},
	{Name: "workable", SourceType: "WORKABLE", AutoMonitor: true},
	{Name: "icims", SourceType: "ICIMS", AutoMonitor: false},
	{Name: "oracle", SourceType: "ORACLE", AutoMonitor: false},
	{Name: "successfactors", SourceType: "CUSTOM", RegistryToken: "SUCCESSFACTORS"},
	{Name: "eightfold", SourceType: "CUSTOM", RegistryToken: "EIGHTFOLD"},
	{Name: "taleo", SourceType: "CUSTOM", RegistryToken: "TALEO"},
	{Name: "phenom", SourceType: "CUSTOM", RegistryToken: "PHENOM"},
	{Name: "avature", SourceType: "CUSTOM", RegistryToken: "AVATURE"},
	{Name: "adp", SourceType: "CUSTOM", RegistryToken: "ADP"},
	{Name: "cornerstone", SourceType: "CUSTOM", RegistryToken: "CORNERSTONE"},
	{Name: "ukg", SourceType: "CUSTOM", RegistryToken: "UKG"},
	{Name: "jobvite", SourceType: "CUSTOM", RegistryToken: "JOBVITE"},
}

// FreeSourceBootstrapConfig controls the local, API-key-free sponsor source
// bootstrap. Tests override BaseURL with httptest; normal callers use defaults.
type FreeSourceBootstrapConfig struct {
	BaseURL      string
	HTTPClient   *http.Client
	RefreshAfter time.Duration
}

// FreeSourceBootstrapResult summarizes one best-effort HOT+WARM sponsor pass.
type FreeSourceBootstrapResult struct {
	PriorityCompanies  int
	HotCompanies       int
	WarmCompanies      int
	DirectoryEntries   int
	MatchedCompanies   int
	ResolvedCompanies  int
	PartialCompanies   int
	EnabledJobSources  int
	RegistryCandidates int
	FetchFailures      int
}

type freeSourceCompany struct {
	ID                     uuid.UUID
	Name                   string
	EmployerNormalizedName string
	Tier                   string
}

type freeSourceEntry struct {
	Inventory  freeSourceInventory
	Name       string
	Slug       string
	URL        string
	CompanyKey string
	Score      int
}

var sourceCompanyParentheticalRE = regexp.MustCompile(`\([^)]*\)`)

var sourceCompanyLegalSuffixes = map[string]bool{
	"inc": true, "incorporated": true, "llc": true, "lp": true, "llp": true,
	"ltd": true, "limited": true, "corp": true, "corporation": true,
	"company": true, "co": true, "plc": true, "pbc": true, "na": true, "si": true,
}

var sourceCompanyBrandOverrides = map[string]string{
	"american express travel related services": "american express",
	"capital one national association":         "capital one",
	"capital one services":                     "capital one",
	"barclays capital":                         "barclays",
	"barclays services":                        "barclays",
	"bofa securities":                          "bank of america",
	"db global technology":                     "deutsche bank",
	"db usa core":                              "deutsche bank",
	"deutsche bank securities":                 "deutsche bank",
	"dell products":                            "dell",
	"dell usa":                                 "dell",
	"deloitte consulting":                      "deloitte",
	"deloitte services":                        "deloitte",
	"deloitte tax":                             "deloitte",
	"deloitte and touche":                      "deloitte",
	"goldman sachs bank usa":                   "goldman sachs",
	"goldman sachs and co":                     "goldman sachs",
	"goldman sachs services":                   "goldman sachs",
	"moodys analytics":                         "moodys",
	"moodys investors service":                 "moodys",
	"morgan stanley and co":                    "morgan stanley",
	"morgan stanley services group":            "morgan stanley",
	"ntt data americas":                        "ntt data",
	"ntt data services":                        "ntt data",
	"openai opco":                              "openai",
	"pricewaterhousecoopers advisory services": "pwc",
	"pricewaterhousecoopers":                   "pwc",
	"pwc us tax":                               "pwc",
	"robinhood markets":                        "robinhood",
	"sap america":                              "sap",
	"sap labs":                                 "sap",
	"tmobile usa":                              "t mobile",
	"visa technology and operations":           "visa",
	"visa usa":                                 "visa",
	"western digital technologies":             "western digital",
	"zoom communications":                      "zoom",
}

var workdayReviewOnlyTokens = []string{
	"internal", "campus", "student", "contractor", "restricted", "redeployment",
	"apac", "india", "global_in", "emea", "europe", "exteu",
	"university", "early_career", "early-career",
}

// BootstrapFreePriorityCompanySources uses the public MIT-licensed ats-scrapers
// company inventories as a discovery accelerator for HOT and WARM H-1B sponsors.
//
// The directory is not treated as authoritative identity evidence: only exact
// normalized company-name matches are accepted automatically. Supported ATS
// types can become job_sources, while unsupported portals are retained in
// company_source_registry for future connectors. The operation is best-effort
// and should never gate API startup.
func (r *Repository) BootstrapFreePriorityCompanySources(ctx context.Context, cfg FreeSourceBootstrapConfig) (FreeSourceBootstrapResult, error) {
	if r.pool == nil {
		return FreeSourceBootstrapResult{}, errors.New("free source bootstrap requires a repository backed by a database pool")
	}
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultFreeSourceDirectoryBaseURL
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 25 * time.Second}
	}
	if cfg.RefreshAfter <= 0 {
		cfg.RefreshAfter = defaultFreeSourceRefreshAfter
	}

	companies, err := r.priorityCompaniesDueForFreeSourceBootstrap(ctx, cfg.RefreshAfter)
	if err != nil {
		return FreeSourceBootstrapResult{}, err
	}
	result := FreeSourceBootstrapResult{PriorityCompanies: len(companies)}
	for _, company := range companies {
		switch company.Tier {
		case "HOT":
			result.HotCompanies++
		case "WARM":
			result.WarmCompanies++
		}
	}
	if len(companies) == 0 {
		return result, nil
	}

	entries, failures := fetchFreeSourceInventories(ctx, cfg)
	result.FetchFailures = failures
	result.DirectoryEntries = len(entries)
	if len(entries) == 0 {
		return result, errors.New("free source bootstrap could not load any source inventories")
	}

	index := make(map[string][]freeSourceEntry)
	for _, entry := range entries {
		if entry.CompanyKey == "" {
			continue
		}
		index[entry.CompanyKey] = append(index[entry.CompanyKey], entry)
	}
	for key := range index {
		sort.SliceStable(index[key], func(i, j int) bool {
			return index[key][i].Score > index[key][j].Score
		})
	}

	for _, company := range companies {
		keys := sourceCompanyExactKeys(company.Name, company.EmployerNormalizedName)
		candidates := make([]freeSourceEntry, 0)
		seen := map[string]bool{}
		for _, key := range keys {
			for _, candidate := range index[key] {
				identity := candidate.Inventory.Name + "|" + candidate.Slug + "|" + candidate.URL
				if seen[identity] {
					continue
				}
				seen[identity] = true
				candidates = append(candidates, candidate)
			}
		}

		if portal, ok := builtInOfficialCareerPortal(company.Name, company.EmployerNormalizedName); ok {
			candidates = append(candidates, freeSourceEntry{
				Inventory: freeSourceInventory{Name: "official", SourceType: "CUSTOM", RegistryToken: "OFFICIAL"},
				Name:      company.Name,
				Slug:      "OFFICIAL",
				URL:       portal,
				Score:     85,
			})
		}

		if len(candidates) == 0 {
			continue
		}
		result.MatchedCompanies++
		sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })

		discoveries := make([]DiscoveredCompanySource, 0, 4)
		monitorableChosen := false
		for _, candidate := range candidates {
			if len(discoveries) >= 4 {
				break
			}
			discovery, ok := candidate.toDiscoveredSource()
			if !ok {
				continue
			}

			// Poll at most one public ATS per sponsor during the MVP. Keep
			// additional exact portals in the registry for later verification.
			if discovery.Monitorable {
				if monitorableChosen {
					discovery.Monitorable = false
					discovery.Confidence = minFloat32(discovery.Confidence, 0.90)
				} else {
					monitorableChosen = true
					result.EnabledJobSources++
				}
			}
			discoveries = append(discoveries, discovery)
		}
		if len(discoveries) == 0 {
			continue
		}
		result.RegistryCandidates += len(discoveries)

		if err := r.RecordDiscoveredCompanySources(ctx, company.ID, discoveries); err != nil {
			slog.Warn("free sponsor source candidate could not be recorded",
				"company", company.Name,
				"company_id", company.ID,
				"error", err,
			)
			continue
		}
		if monitorableChosen {
			result.ResolvedCompanies++
		} else {
			result.PartialCompanies++
		}
	}

	return result, nil
}

func (r *Repository) priorityCompaniesDueForFreeSourceBootstrap(ctx context.Context, refreshAfter time.Duration) ([]freeSourceCompany, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT c.id, c.name, w.employer_normalized_name, w.tier
		FROM company_sponsor_watchlist w
		JOIN companies c ON c.id = w.company_id
		WHERE w.tier IN ('HOT', 'WARM')
		  AND (
		      w.last_source_discovery_at IS NULL
		      OR w.source_discovery_status IN ('PENDING', 'PARTIAL', 'FAILED')
		      OR w.last_source_discovery_at < now() - ($1::bigint * interval '1 second')
		  )
		ORDER BY
			CASE w.tier WHEN 'HOT' THEN 0 ELSE 1 END,
			w.watchlist_rank
	`, int64(refreshAfter.Seconds()))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var companies []freeSourceCompany
	for rows.Next() {
		var item freeSourceCompany
		if err := rows.Scan(&item.ID, &item.Name, &item.EmployerNormalizedName, &item.Tier); err != nil {
			return nil, err
		}
		companies = append(companies, item)
	}
	return companies, rows.Err()
}

func fetchFreeSourceInventories(ctx context.Context, cfg FreeSourceBootstrapConfig) ([]freeSourceEntry, int) {
	type fetchResult struct {
		entries []freeSourceEntry
		err     error
	}
	results := make(chan fetchResult, len(freeSourceInventories))
	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup

	for _, inventory := range freeSourceInventories {
		inventory := inventory
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results <- fetchResult{err: ctx.Err()}
				return
			}
			entries, err := fetchFreeSourceInventory(ctx, cfg, inventory)
			results <- fetchResult{entries: entries, err: err}
		}()
	}
	wg.Wait()
	close(results)

	var all []freeSourceEntry
	failures := 0
	for result := range results {
		if result.err != nil {
			failures++
			slog.Warn("free source inventory fetch failed", "error", result.err)
			continue
		}
		all = append(all, result.entries...)
	}
	return all, failures
}

func fetchFreeSourceInventory(ctx context.Context, cfg FreeSourceBootstrapConfig, inventory freeSourceInventory) ([]freeSourceEntry, error) {
	endpoint := strings.TrimRight(cfg.BaseURL, "/") + "/" + inventory.Name + ".csv"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/csv")
	req.Header.Set("User-Agent", "ApplyForge/1.0 (+free sponsor source bootstrap)")

	resp, err := cfg.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s inventory: %w", inventory.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s inventory returned %s", inventory.Name, resp.Status)
	}

	reader := csv.NewReader(io.LimitReader(resp.Body, 2<<20))
	reader.FieldsPerRecord = -1
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read %s inventory header: %w", inventory.Name, err)
	}
	nameIdx, slugIdx, urlIdx := -1, -1, -1
	for i, field := range header {
		switch strings.ToLower(strings.TrimSpace(field)) {
		case "name", "company_name":
			if nameIdx < 0 {
				nameIdx = i
			}
		case "slug":
			slugIdx = i
		case "url":
			urlIdx = i
		}
	}
	if nameIdx < 0 || slugIdx < 0 || urlIdx < 0 {
		return nil, fmt.Errorf("%s inventory missing name/slug/url columns", inventory.Name)
	}

	var entries []freeSourceEntry
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse %s inventory: %w", inventory.Name, err)
		}
		if nameIdx >= len(record) || slugIdx >= len(record) || urlIdx >= len(record) {
			continue
		}
		name := strings.TrimSpace(record[nameIdx])
		slug := strings.TrimSpace(record[slugIdx])
		sourceURL := strings.TrimSpace(record[urlIdx])
		if name == "" || sourceURL == "" {
			continue
		}
		key := sourceCompanyKey(name)
		if key == "" {
			continue
		}
		entries = append(entries, freeSourceEntry{
			Inventory:  inventory,
			Name:       name,
			Slug:       slug,
			URL:        sourceURL,
			CompanyKey: key,
			Score:      freeSourceEntryScore(inventory, name, sourceURL),
		})
	}
	return entries, nil
}

func freeSourceEntryScore(inventory freeSourceInventory, companyName, sourceURL string) int {
	score := 50
	switch inventory.Name {
	case "greenhouse":
		score = 100
	case "lever", "ashby":
		score = 98
	case "workday":
		score = 94
	case "workable":
		score = 90
	case "smartrecruiters":
		score = 78
	case "icims", "oracle":
		score = 70
	default:
		score = 60
	}
	lowerURL := strings.ToLower(sourceURL)
	if inventory.Name == "workday" {
		// Parenthesized Workday directory names typically represent a
		// secondary site (campus, regional, subsidiary, etc.), not the
		// employer's complete external board.
		if strings.Contains(companyName, "(") {
			score -= 30
		}
		for _, token := range workdayReviewOnlyTokens {
			if strings.Contains(lowerURL, token) {
				score -= 35
				break
			}
		}
		if strings.Contains(lowerURL, "/external") || strings.Contains(lowerURL, "/careers") ||
			strings.Contains(lowerURL, "_external") {
			score += 5
		}
	}
	return score
}

func (entry freeSourceEntry) toDiscoveredSource() (DiscoveredCompanySource, bool) {
	sourceURL := strings.TrimSpace(entry.URL)
	if sourceURL == "" {
		return DiscoveredCompanySource{}, false
	}
	confidence := float32(0.94)
	monitorable := entry.Inventory.AutoMonitor && entry.Score >= 80
	boardToken := strings.TrimSpace(entry.Slug)
	sourceType := entry.Inventory.SourceType

	switch entry.Inventory.Name {
	case "workday":
		discovered, ok := parseWorkdayCareerURL(sourceURL)
		if !ok {
			return DiscoveredCompanySource{
				SourceType: "CUSTOM", BoardToken: "WORKDAY|" + boardToken,
				SourceURL: sourceURL, DiscoveryMethod: "MANUAL",
				Confidence: 0.80, Monitorable: false,
			}, true
		}
		discovered.DiscoveryMethod = "MANUAL"
		discovered.Confidence = confidence
		discovered.Monitorable = monitorable
		return discovered, true
	case "icims":
		if parsed, err := url.Parse(sourceURL); err == nil && parsed.Hostname() != "" {
			boardToken = parsed.Hostname()
		}
	case "oracle":
		if parsed, err := url.Parse(sourceURL); err == nil && parsed.Hostname() != "" {
			boardToken = parsed.Hostname()
		}
	default:
		if sourceType == "CUSTOM" {
			prefix := entry.Inventory.RegistryToken
			if prefix == "" {
				prefix = strings.ToUpper(entry.Inventory.Name)
			}
			boardToken = prefix + "|" + boardToken
		}
	}

	if sourceType == "SMARTRECRUITERS" {
		// Candidate only until a dedicated tenant verifier is added.
		monitorable = false
		confidence = 0.88
	}
	return DiscoveredCompanySource{
		SourceType:      sourceType,
		BoardToken:      boardToken,
		SourceURL:       sourceURL,
		DiscoveryMethod: "MANUAL",
		Confidence:      confidence,
		Monitorable:     monitorable,
	}, true
}

func sourceCompanyExactKeys(values ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		key := sourceCompanyKey(value)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, key)
		if strings.HasPrefix(key, "the ") {
			withoutThe := strings.TrimSpace(strings.TrimPrefix(key, "the "))
			if withoutThe != "" && !seen[withoutThe] {
				seen[withoutThe] = true
				out = append(out, withoutThe)
			}
		}
		if brand, ok := sourceCompanyBrandOverrides[key]; ok && !seen[brand] {
			seen[brand] = true
			out = append(out, brand)
		}
	}
	return out
}

func sourceCompanyKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return ""
	}
	if idx := strings.Index(value, "formerly known as"); idx >= 0 {
		value = strings.TrimSpace(value[:idx])
	}
	value = strings.NewReplacer(
		"n.a.", "na",
		"n.a", "na",
		"l.p.", "lp",
		"l.l.c.", "llc",
		"s.i.", "si",
		"&", " and ",
	).Replace(value)
	value = sourceCompanyParentheticalRE.ReplaceAllString(value, " ")
	value = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= '0' && r <= '9':
			return r
		default:
			return ' '
		}
	}, value)
	fields := strings.Fields(value)
	for len(fields) > 0 && sourceCompanyLegalSuffixes[fields[len(fields)-1]] {
		fields = fields[:len(fields)-1]
	}
	return strings.Join(fields, " ")
}

func builtInOfficialCareerPortal(values ...string) (string, bool) {
	for _, value := range values {
		key := sourceCompanyKey(value)
		for brand, portal := range map[string]string{
			"amazon advertising":            "https://www.amazon.jobs/en",
			"amazon com services":           "https://www.amazon.jobs/en",
			"amazon data services":          "https://www.amazon.jobs/en",
			"amazon development center u s": "https://www.amazon.jobs/en",
			"amazon web services":           "https://www.amazon.jobs/en",
			"annapurna labs u s":            "https://www.amazon.jobs/en",
			"apple":                         "https://jobs.apple.com/en-us/search",
			"google":                        "https://careers.google.com/jobs",
			"microsoft":                     "https://careers.microsoft.com/v2/global/en/home.html",
			"meta platforms":                "https://www.metacareers.com/jobs",
			"netflix":                       "https://jobs.netflix.com/",
			"salesforce":                    "https://careers.salesforce.com/en/jobs/",
			"starbucks coffee":              "https://apply.starbucks.com/careers",
			"tesla":                         "https://www.tesla.com/careers/search",
			"wal mart associates":           "https://careers.walmart.com/us/en",
		} {
			if key == brand {
				return portal, true
			}
		}
	}
	return "", false
}

func minFloat32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
