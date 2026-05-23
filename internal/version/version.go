// Package version exposes the build-time agent/binary version so logs and
// reports can record provenance without each package re-implementing the
// ldflag dance. Set via:
//
//	-ldflags "-X github.com/cicd-sensor/cicd-sensor/internal/version.Current=v0.0.13"
//
// Defaults to "dev" for go run / unset builds.
package version

var Current = "dev"
