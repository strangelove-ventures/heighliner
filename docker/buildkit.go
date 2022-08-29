package docker

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/moby/buildkit/util/progress/progresswriter"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const BuildKitSock = "unix:///run/buildkit/buildkitd.sock"
const DefaultPlatforms = "linux/arm64,linux/amd64"

type BuildKitOptions struct {
	Address  string
	Platform string
	NoCache  bool

	// Set type of progress (auto, plain, tty). Use plain to show container output
	LogBuildProgress string
}

func GetDefaultBuildKitOptions() BuildKitOptions {
	return BuildKitOptions{
		Address:          BuildKitSock,
		Platform:         DefaultPlatforms,
		NoCache:          false,
		LogBuildProgress: "auto",
	}
}

func BuildDockerImageWithBuildKit(
	ctx context.Context,
	dockerfileDir string,
	tags []string,
	push bool,
	args map[string]string,
	buildKitOptions BuildKitOptions,
) error {
	c, err := client.New(ctx, buildKitOptions.Address)
	if err != nil {
		return fmt.Errorf("error getting buildkit client: %v", err)
	}

	attachable := []session.Attachable{authprovider.NewDockerAuthProvider(os.Stderr)}

	eg, ctx := errgroup.WithContext(ctx)

	export := client.ExportEntry{
		Type: "image",
		Attrs: map[string]string{
			"name": strings.Join(tags, ","),
		},
	}
	if push {
		export.Attrs["push"] = "true"
	}
	exports := []client.ExportEntry{export}

	opts := map[string]string{
		"platform": buildKitOptions.Platform,
	}
	for arg, value := range args {
		opts[fmt.Sprintf("build-arg:%s", arg)] = value
	}

	locals := map[string]string{
		"context":    ".",
		"dockerfile": dockerfileDir,
	}

	allowed := []entitlements.Entitlement{entitlements.EntitlementNetworkHost}

	solveOpt := client.SolveOpt{
		Exports:             exports,
		Frontend:            "dockerfile.v0",
		CacheExports:        []client.CacheOptionsEntry{},
		CacheImports:        []client.CacheOptionsEntry{},
		Session:             attachable,
		FrontendAttrs:       opts,
		LocalDirs:           locals,
		AllowedEntitlements: allowed,
	}

	var def *llb.Definition

	if buildKitOptions.NoCache {
		solveOpt.FrontendAttrs["no-cache"] = ""
	}

	// not using shared context to not disrupt display but let is finish reporting errors
	pw, err := progresswriter.NewPrinter(ctx, os.Stderr, buildKitOptions.LogBuildProgress)
	if err != nil {
		return err
	}

	mw := progresswriter.NewMultiWriter(pw)

	var writers []progresswriter.Writer
	for _, at := range attachable {
		if s, ok := at.(interface {
			SetLogger(progresswriter.Logger)
		}); ok {
			w := mw.WithPrefix("", false)
			s.SetLogger(func(s *client.SolveStatus) {
				w.Status() <- s
			})
			writers = append(writers, w)
		}
	}

	eg.Go(func() error {
		defer func() {
			for _, w := range writers {
				close(w.Status())
			}
		}()
		resp, err := c.Solve(ctx, def, solveOpt, progresswriter.ResetTime(mw.WithPrefix("", false)).Status())
		if err != nil {
			return err
		}
		for k, v := range resp.ExporterResponse {
			logrus.Debugf("exporter response: %s=%s", k, v)
		}

		return nil
	})

	eg.Go(func() error {
		<-pw.Done()
		return pw.Err()
	})

	return eg.Wait()
}
