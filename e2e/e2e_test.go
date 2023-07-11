package e2e

import (
	"context"
	"os"
	"testing"
)

func Test_E2E(t *testing.T) {
	if os.Getenv("RUN_E2E") != "true" {
		t.SkipNow()
	}
	ctx := context.Background()
	if s := os.Getenv("SKIP_BUILD"); s != "" && s != "true" {
		buildAssets()
	}
	c := prepare(ctx)
	t.Log(c)
	if os.Getenv("TEARDOWN") == "true" {
		teardown(c)
	}
}
