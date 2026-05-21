package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/cicd-sensor/cicd-sensor/internal/managerauth"
)

// tokenSecretBytes is the raw entropy size. 48 bytes encode to a
// 64-character RawURLEncoding string with ~384 bits of entropy and no
// padding.
const tokenSecretBytes = 48

func runTokenGenerate(_ context.Context, args []string, stdout, stderr io.Writer) (int, error) {
	fs := flag.NewFlagSet("token generate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "usage: cicd-sensorctl token generate")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0, nil
		}
		return 2, err
	}
	if fs.NArg() != 0 {
		return 2, newUsageError(2, "token generate: unexpected positional arguments")
	}

	secret, err := generateTokenSecret()
	if err != nil {
		return 1, fmt.Errorf("token generate: %w", err)
	}

	fmt.Fprintln(stdout, managerauth.TokenPrefix+secret)
	return 0, nil
}

func generateTokenSecret() (string, error) {
	buf := make([]byte, tokenSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
