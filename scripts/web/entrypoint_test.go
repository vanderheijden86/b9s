package web_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/vanderheijden86/b9s/pkg/config"
)

func TestWebEntrypointAttachmentSettings(t *testing.T) {
	for _, scenario := range []string{"environment", "mounted config", "missing config"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "bin")
			capture := filepath.Join(dir, "capture")
			for _, path := range []string{bin, capture, filepath.Join(dir, "secrets")} {
				if err := os.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, text string, mode os.FileMode) {
				t.Helper()
				if err := os.WriteFile(path, []byte(text), mode); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(dir, "secrets", "password"), "test-password", 0o600)
			write(filepath.Join(dir, "commit"), "test-commit\n", 0o600)
			write(filepath.Join(bin, "httpd"), "#!/bin/sh\nexit 0\n", 0o700)
			// Startup must never touch the host's home or start a host SQL client.
			write(filepath.Join(bin, "rm"), "#!/bin/sh\nexit 0\n", 0o700)
			write(filepath.Join(bin, "mariadb"), `#!/bin/sh
case "$*" in
  *"SHOW DATABASES"*) printf 'information_schema\ndemo\n' ;;
  *"SELECT 1 FROM "*) exit 0 ;;
  *) exit 2 ;;
esac
`, 0o700)
			write(filepath.Join(bin, "b9s"), `#!/bin/sh
set -eu
cp "$HOME/.config/b9s/config.yaml" "$CAPTURE/config.yaml"
printf '%s\n' "$B9S_ATTACHMENTS_S3_ACCESS_KEY_ID" "$B9S_ATTACHMENTS_S3_SECRET_ACCESS_KEY" > "$CAPTURE/credentials"
printf '%s\n' "$@" > "$CAPTURE/argv"
`, 0o700)

			source, err := os.ReadFile("entrypoint.sh")
			if err != nil {
				t.Fatal(err)
			}
			// Only container filesystem paths are remapped. The shell, SQL
			// discovery, config seeding and exec environment run as shipped.
			sandbox := strings.NewReplacer(
				"/data/home", filepath.Join(dir, "home"),
				"/app/.commit-sha", filepath.Join(dir, "commit"),
				"/www", filepath.Join(dir, "www"),
				"/usr/local/bin/b9s", filepath.Join(bin, "b9s"),
			).Replace(string(source))
			script := filepath.Join(dir, "entrypoint.sh")
			write(script, sandbox, 0o700)

			env := []string{
				"PATH=" + bin + ":/usr/bin:/bin", "CAPTURE=" + capture,
				"B9S_DOLT_HOST=127.0.0.1", "B9S_DOLT_PORT=13306",
				"B9S_DOLT_USER=owner", "B9S_WEB_OWNER_EMAIL=owner@example.invalid",
				"B9S_WEB_ACTOR=owner", "B9S_WEB_SECRETS=" + filepath.Join(dir, "secrets"),
				"B9S_ATTACHMENTS_S3_ACCESS_KEY_ID=test-access",
				"B9S_ATTACHMENTS_S3_SECRET_ACCESS_KEY=test-secret",
			}
			settings := "attachments:\n  backend: s3\n  prefix: workspace\n  max_bytes: 1024\n" +
				"  s3:\n    endpoint: https://s3.example.invalid\n    region: test-region\n" +
				"    bucket: test-bucket\n    path_style: true\n    url_ttl: 5m\n"
			if scenario != "environment" {
				mount := filepath.Join(dir, "mounted.yaml")
				if scenario == "mounted config" {
					write(mount, settings, 0o400)
				}
				env = append(env, "B9S_WEB_CONFIG_FILE="+mount)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "sh", script)
			cmd.Env = env
			cmd.WaitDelay = time.Second
			out, runErr := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("startup exceeded deadline: %v\n%s", ctx.Err(), out)
			}
			if scenario == "missing config" {
				if runErr == nil {
					t.Fatal("startup accepted a missing B9S_WEB_CONFIG_FILE")
				}
				if !strings.Contains(string(out), "B9S_WEB_CONFIG_FILE") {
					t.Fatalf("missing config diagnostic: %s", out)
				}
				if _, err := os.Stat(filepath.Join(capture, "argv")); !os.IsNotExist(err) {
					t.Fatal("b9s started despite missing mounted config")
				}
				return
			}
			if runErr != nil {
				t.Fatalf("startup: %v\n%s", runErr, out)
			}
			credentials, err := os.ReadFile(filepath.Join(capture, "credentials"))
			if err != nil || string(credentials) != "test-access\ntest-secret\n" {
				t.Fatal("attachment credentials did not reach b9s")
			}
			argv, err := os.ReadFile(filepath.Join(capture, "argv"))
			if err != nil || !strings.Contains(string(argv), "--trust-header\nX-Forwarded-Email\n--owner\nowner@example.invalid\n") {
				t.Fatalf("login proxy arguments: %s (%v)", argv, err)
			}
			got, err := os.ReadFile(filepath.Join(capture, "config.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "environment" {
				if len(got) != 0 {
					t.Fatalf("unconfigured startup seeded unexpected settings: %s", got)
				}
				return
			}
			if string(got) != settings {
				t.Fatalf("mounted settings did not reach generated config: %s", got)
			}
			cfg, err := config.LoadFrom(filepath.Join(capture, "config.yaml"))
			if err != nil || cfg.Attachments == nil || cfg.Attachments.S3.Bucket != "test-bucket" {
				t.Fatalf("generated attachment config did not load: %+v (%v)", cfg.Attachments, err)
			}
			mounted, err := os.ReadFile(filepath.Join(dir, "mounted.yaml"))
			if err != nil || string(mounted) != settings {
				t.Fatal("startup changed the mounted config")
			}
		})
	}
}
