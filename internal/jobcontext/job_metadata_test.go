package jobcontext_test

import (
	"encoding/json"
	"testing"

	"github.com/cicd-sensor/cicd-sensor/internal/jobcontext"
)

func TestJobMetadata_OmitsEmptyOptionalFields(t *testing.T) {
	m := jobcontext.JobMetadata{}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, key := range []string{
		"commit_sha",
		"ref_name",
		"trigger",
		"actor_name",
		"actor_id",
		"github_workflow",
		"github_workflow_ref",
		"github_workflow_sha",
		"gitlab_job_name",
		"gitlab_config_ref_uri",
	} {
		if _, ok := raw[key]; ok {
			t.Errorf("expected key %q to be omitted, but it was present", key)
		}
	}
}

func TestJobMetadata_JSONRoundTrip(t *testing.T) {
	input := jobcontext.JobMetadata{
		CommitSHA:          "abc123",
		RefName:            "main",
		Trigger:            "push",
		ActorName:          "alice",
		ActorID:            "1001",
		GitHubWorkflow:     "build.yml",
		GitHubWorkflowRef:  "acme/example/.github/workflows/build.yml@refs/heads/main",
		GitHubWorkflowSHA:  "def456",
		GitLabJobName:      "jirojiro-smoke",
		GitLabConfigRefURI: "gitlab.com/acme/example//.gitlab-ci.yml@refs/heads/main",
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got jobcontext.JobMetadata
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got != input {
		t.Fatalf("metadata: got %+v, want %+v", got, input)
	}
}
