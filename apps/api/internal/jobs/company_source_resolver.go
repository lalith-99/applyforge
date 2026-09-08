package jobs

import "context"

type CompanySourceResolution struct {
	Sources           []DiscoveredCompanySource
	ProviderRequestID string
}

type CompanySourceResolver interface {
	Provider() string
	Operation() string
	Resolve(ctx context.Context, companyName string) (CompanySourceResolution, error)
}
