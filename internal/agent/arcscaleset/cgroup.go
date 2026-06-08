// Package arcscaleset turns runner-pod peer PIDs into Actions Runner
// Controller scale-set identities. It pairs a periodic-refresh cache of
// Kubernetes pod metadata with a cgroup parser that extracts the pod UID
// from the /proc/<pid>/cgroup line of a peer process running inside a
// kubelet-managed pod.
package arcscaleset

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// podCgroupReader points at the file the resolver reads to find the peer's
// cgroup path. Tests override it.
var podCgroupReader = readProcCgroup

func readProcCgroup(pid int32) (string, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err != nil {
		return "", fmt.Errorf("read /proc/%d/cgroup: %w", pid, err)
	}
	return string(data), nil
}

// ErrNotInKubernetesPod reports that the cgroup path the peer reported did
// not match the kubelet cgroup layout. The Agent treats this as the
// single-scale-set fallback signal: the peer is either not in a Kubernetes pod
// or is in a pod the agent should not classify.
var ErrNotInKubernetesPod = errors.New("peer cgroup does not belong to a Kubernetes pod")

// extractPodUIDFromCgroup parses the kubelet cgroup layout to find the
// owning pod UID. It accepts both the cgroup v2 unified-hierarchy form
// (single `0::/...` line) and the legacy cgroup v1 multi-line form.
//
// Recognised layouts (systemd cgroup driver):
//
//	kubepods-pod<uid>.slice                        — guaranteed-QoS pods
//	kubepods-besteffort-pod<uid>.slice             — best-effort QoS pods
//	kubepods-burstable-pod<uid>.slice              — burstable QoS pods
//
// Recognised layouts (cgroupfs cgroup driver, present on older nodes):
//
//	kubepods/pod<uid>/
//	kubepods/besteffort/pod<uid>/
//	kubepods/burstable/pod<uid>/
//
// systemd encodes the pod UID with `_` separators; cgroupfs uses `-`. Both
// are normalised back to the canonical UUID form returned by the kube-apiserver.
func extractPodUIDFromCgroup(cgroupFile string) (string, error) {
	for _, line := range strings.Split(cgroupFile, "\n") {
		// cgroup v2 unified: "0::/<path>"
		// cgroup v1: "<hierarchy-id>:<controllers>:/<path>"
		idx := strings.LastIndex(line, ":")
		if idx < 0 {
			continue
		}
		path := line[idx+1:]
		uid, ok := parseKubeletPodUIDFromPath(path)
		if !ok {
			continue
		}
		if uid == "" {
			continue
		}
		return uid, nil
	}
	return "", ErrNotInKubernetesPod
}

// parseKubeletPodUIDFromPath scans a single cgroup path for the kubelet
// pod-cgroup segment and returns the decoded pod UID.
func parseKubeletPodUIDFromPath(path string) (string, bool) {
	for _, segment := range strings.Split(path, "/") {
		// systemd driver: "<prefix>-pod<uid>.slice"
		if strings.HasSuffix(segment, ".slice") && strings.Contains(segment, "-pod") {
			head, tail, ok := strings.Cut(segment, "-pod")
			if !ok {
				continue
			}
			// Only accept the kubepods.slice family; this filters out
			// segments like "system-pod.slice" that an operator may
			// have created.
			if !strings.HasPrefix(head, "kubepods") {
				continue
			}
			rest := strings.TrimSuffix(tail, ".slice")
			return normaliseSystemdPodUID(rest), true
		}
		// cgroupfs driver: "pod<uid>" as its own segment, parent is
		// "kubepods" or a QoS slice underneath it.
		if strings.HasPrefix(segment, "pod") {
			uid := strings.TrimPrefix(segment, "pod")
			if looksLikePodUID(uid) {
				return uid, true
			}
		}
	}
	return "", false
}

// normaliseSystemdPodUID converts the `_`-separated systemd encoding back to
// the canonical UUID form returned by the kube-apiserver.
func normaliseSystemdPodUID(s string) string {
	return strings.ReplaceAll(s, "_", "-")
}

// looksLikePodUID does a cheap sanity check on a candidate UID. A
// canonical Kubernetes pod UID is a 36-character UUID; we accept anything
// non-trivial that starts with a hex digit so cluster variants are not
// silently dropped.
func looksLikePodUID(s string) bool {
	if len(s) < 8 {
		return false
	}
	c := s[0]
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
}
