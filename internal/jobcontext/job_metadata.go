package jobcontext

// JobMetadata carries optional non-identity context attached to a Job.
// JobIdentity owns attribution; metadata is for logs, reports, and search.
type JobMetadata struct {
	// Shared across providers.
	CommitSHA string `json:"commit_sha,omitempty"`
	RefName   string `json:"ref_name,omitempty"`
	Trigger   string `json:"trigger,omitempty"`
	ActorName string `json:"actor_name,omitempty"`
	ActorID   string `json:"actor_id,omitempty"`

	// GitHub only.
	GitHubWorkflow    string `json:"github_workflow,omitempty"`
	GitHubWorkflowRef string `json:"github_workflow_ref,omitempty"`
	GitHubWorkflowSHA string `json:"github_workflow_sha,omitempty"`

	// GitLab only.
	GitLabJobName      string `json:"gitlab_job_name,omitempty"`
	GitLabConfigRefURI string `json:"gitlab_config_ref_uri,omitempty"`
}
