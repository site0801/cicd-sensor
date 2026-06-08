package jobcontext

// Provider identifies the CI/CD provider, or — for runner environments that
// need a different listener route surface — a provider variant.
//
// ProviderGitHub and ProviderGitLab are the canonical values that appear in
// JobIdentity. ProviderGitHubARC is a listener-mode marker: it selects the
// ARC runner-environment route set, while the JobIdentity for jobs reaching
// that listener still carries Provider = ProviderGitHub.
type Provider string

const (
	ProviderGitHub    Provider = "github"
	ProviderGitHubARC Provider = "github-arc"
	ProviderGitLab    Provider = "gitlab"
)
