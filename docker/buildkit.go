package docker

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/docker/cli/cli/config"
	"github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/session"
	"github.com/moby/buildkit/session/auth/authprovider"
	"github.com/moby/buildkit/session/secrets/secretsprovider"
	"github.com/moby/buildkit/session/sshforward/sshprovider"
	"github.com/moby/buildkit/util/entitlements"
	"github.com/moby/buildkit/util/progress/progresswriter"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const BuildKitSock = "unix:///run/buildkit/buildkitd.sock"
const DefaultPlatforms = "linux/arm64,linux/amd64"

const (
	GitKnownHostsSecretID = "git_known_hosts"
	GitNetrcSecretID      = "git_netrc"
)

type BuildKitSessionOptions struct {
	SSHAgentSocket string
	SSHPrivateKey  []byte
	Secrets        map[string][]byte
}

type BuildKitOptions struct {
	Address  string
	Platform string
	NoCache  bool
	Session  BuildKitSessionOptions

	// Set type of progress (auto, plain, tty). Use plain to show container output
	LogBuildProgress string
}

func buildKitSessionAttachables(options BuildKitSessionOptions) ([]session.Attachable, error) {
	attachable := make([]session.Attachable, 0, 2)
	if options.SSHAgentSocket != "" && len(options.SSHPrivateKey) != 0 {
		return nil, fmt.Errorf("cannot forward both an SSH agent and a private key")
	}

	sshPath := options.SSHAgentSocket
	if len(options.SSHPrivateKey) != 0 {
		keyFile, err := os.CreateTemp("", "heighliner-buildkit-ssh-key-")
		if err != nil {
			return nil, fmt.Errorf("create temporary SSH key: %w", err)
		}
		keyPath := keyFile.Name()
		defer func() { _ = os.Remove(keyPath) }()
		if _, err := keyFile.Write(options.SSHPrivateKey); err != nil {
			_ = keyFile.Close()
			return nil, fmt.Errorf("write temporary SSH key: %w", err)
		}
		if err := keyFile.Close(); err != nil {
			return nil, fmt.Errorf("close temporary SSH key: %w", err)
		}
		sshPath = keyPath
	}

	if sshPath != "" {
		provider, err := sshprovider.NewSSHAgentProvider([]sshprovider.AgentConfig{{Paths: []string{sshPath}}})
		if err != nil {
			return nil, fmt.Errorf("create BuildKit SSH provider: %w", err)
		}
		attachable = append(attachable, provider)
	}
	if len(options.Secrets) != 0 {
		attachable = append(attachable, secretsprovider.FromMap(options.Secrets))
	}
	return attachable, nil
}

func buildKitFrontendAttrs(args map[string]string, buildKitOptions BuildKitOptions) map[string]string {
	opts := map[string]string{
		"platform": buildKitOptions.Platform,
	}
	for arg, value := range args {
		opts[fmt.Sprintf("build-arg:%s", arg)] = value
	}
	if buildKitOptions.NoCache {
		opts["no-cache"] = ""
	}
	return opts
}

func GetDefaultBuildKitOptions() BuildKitOptions {
	return BuildKitOptions{
		Address:          BuildKitSock,
		Platform:         DefaultPlatforms,
		NoCache:          false,
		LogBuildProgress: "auto",
	}
}

type WriteCloser struct {
	f *os.File
	*bufio.Writer
}

func (wc *WriteCloser) Close() error {
	if err := wc.Flush(); err != nil {
		return err
	}
	return wc.f.Close()
}

func BuildDockerImageWithBuildKit(
	ctx context.Context,
	dockerfileDir string,
	tags []string,
	push bool,
	tarExport string,
	args map[string]string,
	buildKitOptions BuildKitOptions,
) error {
	c, err := client.New(ctx, buildKitOptions.Address)
	if err != nil {
		return fmt.Errorf("error getting buildkit client: %v", err)
	}

	dockerConfig := config.LoadDefaultConfigFile(os.Stderr)
	attachable := []session.Attachable{authprovider.NewDockerAuthProvider(dockerConfig)}
	authAttachables, err := buildKitSessionAttachables(buildKitOptions.Session)
	if err != nil {
		return err
	}
	attachable = append(attachable, authAttachables...)

	eg, ctx := errgroup.WithContext(ctx)

	attrs := map[string]string{
		"name": strings.Join(tags, ","),
	}

	exports := make([]client.ExportEntry, 1)

	if tarExport != "" {
		if len(strings.Split(buildKitOptions.Platform, ",")) > 1 {
			return fmt.Errorf("when using tar-export-path, only one platform is supported")
		}

		exports[0] = client.ExportEntry{
			Type:  "docker",
			Attrs: attrs,
			Output: func(m map[string]string) (io.WriteCloser, error) {
				f, err := os.Create(tarExport)
				if err != nil {
					return nil, err
				}

				return &WriteCloser{f, bufio.NewWriter(f)}, nil
			},
		}
	} else {
		export := client.ExportEntry{
			Type:  "image",
			Attrs: attrs,
		}
		if push {
			export.Attrs["push"] = "true"
		}
		exports[0] = export
	}

	opts := buildKitFrontendAttrs(args, buildKitOptions)

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
