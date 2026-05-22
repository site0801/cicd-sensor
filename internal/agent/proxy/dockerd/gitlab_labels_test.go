package dockerd

import (
	"strings"
	"testing"

	"github.com/cicd-sensor/cicd-sensor/internal/jobcontext"
)

func TestJobIdentityFromGitLabRunnerLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		labels    map[string]string
		want      jobcontext.JobIdentity
		wantErr   bool
		errSubstr string
	}{
		{
			name: "complete labels yield gitlab identity",
			labels: map[string]string{
				gitLabRunnerJobURLLabel: "https://gitlab.com/cicd-sensor/cicd-sensor-testing/-/jobs/14202203981",
				gitLabRunnerJobIDLabel:  "14202203981",
			},
			want: jobcontext.GitLabJobIdentity("gitlab.com", "cicd-sensor/cicd-sensor-testing", "14202203981"),
		},
		{
			name: "self-hosted host with subgroup",
			labels: map[string]string{
				gitLabRunnerJobURLLabel: "https://gitlab.example.com/group/sub/project/-/jobs/42",
				gitLabRunnerJobIDLabel:  "42",
			},
			want: jobcontext.GitLabJobIdentity("gitlab.example.com", "group/sub/project", "42"),
		},
		{
			name: "deeply nested subgroups are preserved verbatim",
			labels: map[string]string{
				gitLabRunnerJobURLLabel: "https://gitlab.example.com/top/sub1/sub2/sub3/project/-/jobs/123",
				gitLabRunnerJobIDLabel:  "123",
			},
			want: jobcontext.GitLabJobIdentity("gitlab.example.com", "top/sub1/sub2/sub3/project", "123"),
		},
		{
			name: "host is normalized to lower case",
			labels: map[string]string{
				gitLabRunnerJobURLLabel: "https://GitLab.Example.COM/group/project/-/jobs/7",
				gitLabRunnerJobIDLabel:  "7",
			},
			want: jobcontext.GitLabJobIdentity("gitlab.example.com", "group/project", "7"),
		},
		{
			name:      "nil labels rejected",
			labels:    nil,
			wantErr:   true,
			errSubstr: "labels are required",
		},
		{
			name: "missing job url label rejected",
			labels: map[string]string{
				gitLabRunnerJobIDLabel: "42",
			},
			wantErr:   true,
			errSubstr: "job.url",
		},
		{
			name: "missing job id label rejected",
			labels: map[string]string{
				gitLabRunnerJobURLLabel: "https://gitlab.com/g/p/-/jobs/42",
			},
			wantErr:   true,
			errSubstr: "job.id",
		},
		{
			name: "url without /-/jobs/ segment rejected",
			labels: map[string]string{
				gitLabRunnerJobURLLabel: "https://gitlab.com/group/project/pipelines/100",
				gitLabRunnerJobIDLabel:  "42",
			},
			wantErr:   true,
			errSubstr: "unsupported job URL path",
		},
		{
			name: "url with empty project path rejected",
			labels: map[string]string{
				gitLabRunnerJobURLLabel: "https://gitlab.com/-/jobs/42",
				gitLabRunnerJobIDLabel:  "42",
			},
			wantErr:   true,
			errSubstr: "empty project path",
		},
		{
			name: "url with empty job id rejected",
			labels: map[string]string{
				gitLabRunnerJobURLLabel: "https://gitlab.com/g/p/-/jobs/",
				gitLabRunnerJobIDLabel:  "42",
			},
			wantErr:   true,
			errSubstr: "empty job id",
		},
		{
			name: "url job id mismatch with label job id rejected",
			labels: map[string]string{
				gitLabRunnerJobURLLabel: "https://gitlab.com/g/p/-/jobs/100",
				gitLabRunnerJobIDLabel:  "200",
			},
			wantErr:   true,
			errSubstr: "does not match",
		},
		{
			name: "non-numeric job id rejected by Validate",
			labels: map[string]string{
				gitLabRunnerJobURLLabel: "https://gitlab.com/g/p/-/jobs/abc",
				gitLabRunnerJobIDLabel:  "abc",
			},
			wantErr:   true,
			errSubstr: "positive integer",
		},
		{
			name: "unparseable url rejected",
			labels: map[string]string{
				gitLabRunnerJobURLLabel: "://bad",
				gitLabRunnerJobIDLabel:  "1",
			},
			wantErr:   true,
			errSubstr: "parse",
		},
		{
			name: "url with no host rejected",
			labels: map[string]string{
				gitLabRunnerJobURLLabel: "https:///g/p/-/jobs/1",
				gitLabRunnerJobIDLabel:  "1",
			},
			wantErr:   true,
			errSubstr: "no host",
		},
		{
			name: "trailing slash on job id is tolerated and matched",
			labels: map[string]string{
				gitLabRunnerJobURLLabel: "https://gitlab.com/g/p/-/jobs/42/",
				gitLabRunnerJobIDLabel:  "42",
			},
			want: jobcontext.GitLabJobIdentity("gitlab.com", "g/p", "42"),
		},
		{
			name: "query and fragment are ignored",
			labels: map[string]string{
				gitLabRunnerJobURLLabel: "https://gitlab.com/g/p/-/jobs/42?foo=bar#trace",
				gitLabRunnerJobIDLabel:  "42",
			},
			want: jobcontext.GitLabJobIdentity("gitlab.com", "g/p", "42"),
		},
		{
			name: "extra job url path segment rejected",
			labels: map[string]string{
				gitLabRunnerJobURLLabel: "https://gitlab.com/g/p/-/jobs/42/extra",
				gitLabRunnerJobIDLabel:  "42",
			},
			wantErr:   true,
			errSubstr: "unsupported job URL path",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := jobIdentityFromGitLabLabels(tc.labels)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("got nil err, want error containing %q", tc.errSubstr)
				}
				if tc.errSubstr != "" && !strings.Contains(err.Error(), tc.errSubstr) {
					t.Fatalf("error %q does not contain %q", err.Error(), tc.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("identity: got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestJobMetadataFromGitLabContainer(t *testing.T) {
	t.Parallel()

	t.Run("labels supply identity-anchored metadata; env supplies low-trust fields", func(t *testing.T) {
		labels := map[string]string{
			gitLabRunnerJobSHALabel: "c4c41b82483929ffab3abae20b60dd9f793400ba",
			gitLabRunnerJobRefLabel: "main",
		}
		env := []string{
			"CI_PIPELINE_SOURCE=push",
			"CI_JOB_NAME=build",
			"GITLAB_USER_LOGIN=rung",
		}
		got := jobMetadataFromGitLabContainer(labels, env)
		want := jobcontext.JobMetadata{
			CommitSHA: "c4c41b82483929ffab3abae20b60dd9f793400ba",
			Branch:    "main",
			Trigger:   "push",
			Workflow:  "build",
			Actor:     "rung",
		}
		if got != want {
			t.Fatalf("metadata: got %+v, want %+v", got, want)
		}
	})

	t.Run("first-wins resolves user-spoofed env duplicates to runner-set value", func(t *testing.T) {
		// In observed runs, gitlab-runner emits predefined vars first and
		// .gitlab-ci.yml `variables:` overrides are appended later. firstEnv
		// must therefore return the FIRST occurrence so the trusted value wins.
		env := []string{
			"CI_PIPELINE_SOURCE=push",
			"CI_PIPELINE_SOURCE=api", // hypothetical late-write spoof
			"GITLAB_USER_LOGIN=rung",
			"GITLAB_USER_LOGIN=attacker",
		}
		got := jobMetadataFromGitLabContainer(nil, env)
		if got.Trigger != "push" {
			t.Fatalf("trigger: got %q, want first-occurrence %q", got.Trigger, "push")
		}
		if got.Actor != "rung" {
			t.Fatalf("actor: got %q, want first-occurrence %q", got.Actor, "rung")
		}
	})

	t.Run("missing labels and env yield zero metadata, not error", func(t *testing.T) {
		got := jobMetadataFromGitLabContainer(nil, nil)
		if (got != jobcontext.JobMetadata{}) {
			t.Fatalf("expected zero metadata, got %+v", got)
		}
	})

	t.Run("malformed env entries are skipped without panic", func(t *testing.T) {
		env := []string{"=novalue", "noequals", "", "CI_PIPELINE_SOURCE=schedule"}
		if got := jobMetadataFromGitLabContainer(nil, env); got.Trigger != "schedule" {
			t.Fatalf("trigger after malformed entries: got %q, want %q", got.Trigger, "schedule")
		}
	})
}
