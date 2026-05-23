package main

import (
	"os"
	"os/exec"
	"testing"

	"github.com/cicd-sensor/cicd-sensor/internal/version"
)

func TestManagerVersionFlag(t *testing.T) {
	for _, arg := range []string{"--version", "-v"} {
		t.Run(arg, func(t *testing.T) {
			cmd := exec.Command(os.Args[0], "-test.run=TestManagerVersionProcess", "--", arg)
			cmd.Env = append(os.Environ(), "CICD_SENSOR_MANAGER_VERSION_PROCESS=1")
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("version command %s failed: %v\n%s", arg, err, out)
			}
			if got, want := string(out), version.Current+"\n"; got != want {
				t.Fatalf("version output: got %q, want %q", got, want)
			}
		})
	}
}

func TestManagerVersionProcess(t *testing.T) {
	if os.Getenv("CICD_SENSOR_MANAGER_VERSION_PROCESS") != "1" {
		return
	}
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"cicd-sensor-manager"}, os.Args[i+1:]...)
			main()
			os.Exit(0)
		}
	}
	os.Exit(2)
}
