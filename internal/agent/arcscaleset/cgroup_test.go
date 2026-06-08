package arcscaleset

import (
	"errors"
	"testing"
)

func TestExtractPodUIDFromCgroup_SystemdDriver(t *testing.T) {
	cases := []struct {
		name    string
		cgroup  string
		wantUID string
	}{
		{
			name:    "guaranteed QoS cgroup v2",
			cgroup:  "0::/kubepods.slice/kubepods-pod6c0e1428_e8e2_47e1_92e7_5a9f23a0b8d3.slice/cri-containerd-abc.scope\n",
			wantUID: "6c0e1428-e8e2-47e1-92e7-5a9f23a0b8d3",
		},
		{
			name:    "burstable QoS cgroup v2",
			cgroup:  "0::/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod6c0e1428_e8e2_47e1_92e7_5a9f23a0b8d3.slice/cri-containerd-abc.scope\n",
			wantUID: "6c0e1428-e8e2-47e1-92e7-5a9f23a0b8d3",
		},
		{
			name:    "besteffort QoS cgroup v2",
			cgroup:  "0::/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod6c0e1428_e8e2_47e1_92e7_5a9f23a0b8d3.slice/cri-containerd-abc.scope\n",
			wantUID: "6c0e1428-e8e2-47e1-92e7-5a9f23a0b8d3",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := extractPodUIDFromCgroup(tc.cgroup)
			if err != nil {
				t.Fatalf("extractPodUIDFromCgroup: %v", err)
			}
			if got != tc.wantUID {
				t.Fatalf("uid: got %q, want %q", got, tc.wantUID)
			}
		})
	}
}

func TestExtractPodUIDFromCgroup_CgroupfsDriver(t *testing.T) {
	cgroup := "0::/kubepods/burstable/pod6c0e1428-e8e2-47e1-92e7-5a9f23a0b8d3/abc123def456\n"
	got, err := extractPodUIDFromCgroup(cgroup)
	if err != nil {
		t.Fatalf("extractPodUIDFromCgroup: %v", err)
	}
	if want := "6c0e1428-e8e2-47e1-92e7-5a9f23a0b8d3"; got != want {
		t.Fatalf("uid: got %q, want %q", got, want)
	}
}

func TestExtractPodUIDFromCgroup_NotInPod(t *testing.T) {
	// Plain systemd service — not under kubepods at all.
	cgroup := "0::/system.slice/sshd.service\n"
	_, err := extractPodUIDFromCgroup(cgroup)
	if !errors.Is(err, ErrNotInKubernetesPod) {
		t.Fatalf("error: got %v, want ErrNotInKubernetesPod", err)
	}
}

func TestExtractPodUIDFromCgroup_NonKubepodsPodSegmentRejected(t *testing.T) {
	// A user-created slice that happens to contain "-pod" must NOT match.
	cgroup := "0::/custom.slice/operator-pod-deadbeef.slice/whatever.scope\n"
	_, err := extractPodUIDFromCgroup(cgroup)
	if !errors.Is(err, ErrNotInKubernetesPod) {
		t.Fatalf("error: got %v, want ErrNotInKubernetesPod (non-kubepods slice must not match)", err)
	}
}

func TestExtractPodUIDFromCgroup_MultilineCgroupV1(t *testing.T) {
	cgroup := "11:cpu,cpuacct:/kubepods.slice/kubepods-pod6c0e1428_e8e2_47e1_92e7_5a9f23a0b8d3.slice/docker-abc.scope\n" +
		"10:memory:/kubepods.slice/kubepods-pod6c0e1428_e8e2_47e1_92e7_5a9f23a0b8d3.slice/docker-abc.scope\n"
	got, err := extractPodUIDFromCgroup(cgroup)
	if err != nil {
		t.Fatalf("extractPodUIDFromCgroup: %v", err)
	}
	if want := "6c0e1428-e8e2-47e1-92e7-5a9f23a0b8d3"; got != want {
		t.Fatalf("uid: got %q, want %q", got, want)
	}
}
