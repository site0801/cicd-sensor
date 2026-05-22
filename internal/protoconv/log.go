package protoconv

import (
	"net/url"
	"strings"

	"github.com/cicd-sensor/cicd-sensor/internal/jobcontext"
	logv1 "github.com/cicd-sensor/cicd-sensor/internal/proto/cicd_sensor/log/v1"
)

// ToJobLogContext populates only provider-relevant fields so the unrelated
// provider's keys stay zero-valued and drop out of marshalled JSON.
func ToJobLogContext(identity jobcontext.JobIdentity, metadata jobcontext.JobMetadata, runnerKind string) *logv1.JobLogContext {
	out := &logv1.JobLogContext{
		Provider:     string(identity.Provider),
		ProviderHost: identity.ProviderHost,
		ProjectPath:  identity.ProjectPath,
		RunnerKind:   runnerKind,
		JobLink:      logJobLink(identity),
		CommitSha:    metadata.CommitSHA,
		RefName:      metadata.RefName,
		Trigger:      metadata.Trigger,
		ActorName:    metadata.ActorName,
		ActorId:      metadata.ActorID,
	}
	switch identity.Provider {
	case jobcontext.ProviderGitHub:
		out.GithubRunId = identity.GitHubRunID
		out.GithubJob = identity.GitHubJob
		out.GithubRunAttempt = identity.GitHubRunAttempt
		out.GithubRunnerTrackingId = identity.GitHubRunnerTrackingID
		out.GithubWorkflow = metadata.GitHubWorkflow
		out.GithubWorkflowRef = metadata.GitHubWorkflowRef
		out.GithubWorkflowSha = metadata.GitHubWorkflowSHA
	case jobcontext.ProviderGitLab:
		out.GitlabJobId = identity.GitLabJobID
		out.GitlabJobName = metadata.GitLabJobName
		out.GitlabConfigRefUri = metadata.GitLabConfigRefURI
	}
	return out
}

func logJobLink(identity jobcontext.JobIdentity) string {
	if identity.ProviderHost == "" || identity.ProjectPath == "" {
		return ""
	}
	u := url.URL{Scheme: "https", Host: identity.ProviderHost}
	switch identity.Provider {
	case jobcontext.ProviderGitHub:
		if identity.GitHubRunID == "" {
			return ""
		}
		u.Path = strings.TrimSuffix("/"+identity.ProjectPath, "/") + "/actions/runs/" + identity.GitHubRunID
	case jobcontext.ProviderGitLab:
		if identity.GitLabJobID == "" {
			return ""
		}
		u.Path = strings.TrimSuffix("/"+identity.ProjectPath, "/") + "/-/jobs/" + identity.GitLabJobID
	default:
		return ""
	}
	return u.String()
}
