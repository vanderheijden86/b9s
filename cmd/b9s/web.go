package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"github.com/vanderheijden86/beadwork/internal/datasource"
	"github.com/vanderheijden86/beadwork/pkg/config"
	"github.com/vanderheijden86/beadwork/pkg/debug"
	"github.com/vanderheijden86/beadwork/pkg/loader"
	"github.com/vanderheijden86/beadwork/pkg/ui"
	"github.com/vanderheijden86/beadwork/pkg/web"
)

const defaultWebListen = "127.0.0.1:7979"

// webSecretPath holds the secret the pairing token derives from. It lives
// beside config.yaml so a paired phone stays paired across restarts.
func webSecretPath() string {
	return filepath.Join(config.ConfigDir(), "web-secret")
}

// runWeb is `b9s web`: it serves the mobile UI for the project b9s would open
// in this folder, until interrupted.
func runWeb(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("b9s web", flag.ContinueOnError)
	fs.SetOutput(stderr)
	listen := fs.String("listen", defaultWebListen, "Address to serve on. Anything but loopback needs the pairing token")
	noToken := fs.Bool("no-token", false, "Serve without pairing (loopback addresses only)")
	newToken := fs.Bool("new-token", false, "Replace the pairing secret, which unpairs every browser")
	trustHeader := fs.String("trust-header", "", "Behind a login proxy: pair requests whose `header` names --owner, instead of a pairing link")
	owner := fs.String("owner", "", "The `email` --trust-header must carry")
	projectsRoot := fs.String("projects-root", "", "List every Beads checkout directly under `dir` in the project sheet, after the recent ones")
	filter := fs.String("filter", "", "Start the browser with this issue query applied")
	debugFlag := fs.Bool("debug", false, "Enable debug logging to .b9s/debug.log")
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: b9s web [flags]")
		fmt.Fprintln(stderr, "\nServes the mobile web UI for the project in this folder.")
		fmt.Fprintln(stderr, "Open the printed pairing link once on each phone or browser.")
		fmt.Fprintln(stderr, "\nFlags:")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if *debugFlag {
		cwd, _ := os.Getwd()
		if cleanup, err := debug.EnableFileLogging(cwd); err == nil {
			defer cleanup()
		}
	}

	if (*trustHeader == "") != (*owner == "") {
		fmt.Fprintln(stderr, "b9s web: --trust-header and --owner go together")
		return 2
	}
	if *trustHeader != "" && *noToken {
		fmt.Fprintln(stderr, "b9s web: --trust-header cannot be combined with --no-token")
		return 2
	}

	var auth *web.Auth
	if !*noToken {
		secret, err := web.LoadOrCreateSecret(webSecretPath(), *newToken)
		if err != nil {
			fmt.Fprintf(stderr, "b9s web: pairing secret: %v\n", err)
			return 1
		}
		if *trustHeader != "" {
			auth, err = web.NewHeaderAuth(secret, *trustHeader, *owner)
		} else {
			auth, err = web.NewAuth(secret)
		}
		if err != nil {
			fmt.Fprintf(stderr, "b9s web: %v\n", err)
			return 1
		}
	}
	if err := web.CheckListen(*listen, auth); err != nil {
		fmt.Fprintf(stderr, "b9s web: %v\n", err)
		return 2
	}

	appCfg, cfgErr := config.Load()
	if cfgErr != nil {
		appCfg = config.DefaultConfig()
	}
	startupBeadsDir, _ := loader.GetBeadsDir("")
	startupDir := filepath.Dir(startupBeadsDir)
	// Never fall back to another project: a server started in the wrong
	// folder must fail rather than serve other data to a phone.
	choice, err := chooseStartupProject(filepath.Base(startupDir), startupDir, &appCfg, false, datasource.OpenProject)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	startupUser := choice.Opened.Source.User
	if choice.Opened.DoltFailure != nil && choice.Opened.DoltFailure.User != "" {
		startupUser = choice.Opened.DoltFailure.User
	}

	store := web.NewStore(appCfg.RefreshPollInterval())
	defer store.Close()
	if failure := store.Open(datasource.OpenTarget{Name: choice.Name, Dir: choice.Dir}, startupKey(choice.Name, choice.Dir)); failure != nil {
		fmt.Fprint(stderr, formatOpenFailure(failure))
		return 1
	}
	if cfgErr == nil {
		if recent, ok := config.RecentFromCheckout(choice.Name, choice.Dir); ok {
			rememberRecent(recent)
		}
	}

	srv, err := web.NewServer(web.Options{
		Store:        store,
		Writer:       ui.NewIssueWriter(),
		Auth:         auth,
		StartupUser:  startupUser,
		InitialQuery: *filter,
		Projects: func() []config.Project {
			cfg, err := config.Load()
			if err != nil {
				cfg = appCfg
			}
			target := store.Target()
			return withCheckoutsUnder(ui.HeaderProjects(cfg.RecentProjects, target.Name, target.Dir), *projectsRoot)
		},
		Opened: func(p config.Project) {
			if cfgErr == nil {
				rememberRecent(config.RecentProject{Name: p.Name, Database: p.Database, Host: p.Host, Path: p.Path})
			}
		},
	})
	if err != nil {
		fmt.Fprintf(stderr, "b9s web: %v\n", err)
		return 1
	}

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintf(stderr, "b9s web: %v\n", err)
		return 1
	}
	httpServer := &http.Server{Handler: srv, ReadHeaderTimeout: 10 * time.Second}
	printWebBanner(stdout, choice.Name, ln.Addr().String(), auth)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	serveErr := make(chan error, 1)
	go func() { serveErr <- httpServer.Serve(ln) }()
	select {
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(stderr, "b9s web: %v\n", err)
			return 1
		}
	case <-ctx.Done():
	}
	// Open event streams never end by themselves, so shutdown is bounded.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	return 0
}

// withCheckoutsUnder appends every Beads checkout directly under root that
// projects does not already hold, in name order. The recent list keeps at most
// nine projects, so a board serving every database a person may read needs
// this to reach the rest.
func withCheckoutsUnder(projects []config.Project, root string) []config.Project {
	if root == "" {
		return projects
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		debug.Log("web: reading projects root %s failed: %v", root, err)
		return projects
	}
	listed := make(map[string]bool, len(projects))
	for _, p := range projects {
		listed[p.ResolvedPath()] = true
	}
	for _, e := range entries {
		dir := filepath.Join(root, e.Name())
		if !e.IsDir() || listed[dir] {
			continue
		}
		if info, err := os.Stat(filepath.Join(dir, ".beads")); err != nil || !info.IsDir() {
			continue
		}
		projects = append(projects, config.Project{Name: e.Name(), Path: dir})
	}
	return projects
}

// startupKey is the project key the header gives the startup checkout.
func startupKey(name, dir string) string {
	return ui.ProjectKey(config.Project{Name: name, Path: dir})
}

func rememberRecent(recent config.RecentProject) {
	cfg, err := config.Load()
	if err != nil {
		return
	}
	cfg.TouchRecent(recent)
	if !cfg.MarkOpened(recent, time.Now()) {
		return
	}
	if err := config.SaveRecentTo(config.ConfigPath(), cfg.RecentProjects, cfg.LockRecent); err != nil {
		debug.Log("web: saving recent projects failed: %v", err)
	}
}

// pairURL is the link a browser opens once to pair.
func pairURL(base string, auth *web.Auth) string {
	if auth == nil {
		return base + "/"
	}
	return base + "/pair?t=" + auth.PairToken()
}

func printWebBanner(w io.Writer, project, addr string, auth *web.Auth) {
	if auth.TrustsHeader() {
		fmt.Fprintf(w, "b9s web is serving %s on %s behind a login proxy\n", project, addr)
		fmt.Fprintln(w, "Only requests the proxy signed in as the owner are served.")
		return
	}
	host, port, _ := net.SplitHostPort(addr)
	local := "http://" + net.JoinHostPort(host, port)
	if host == "::" || host == "0.0.0.0" || host == "" {
		local = "http://" + net.JoinHostPort("localhost", port)
	}
	fmt.Fprintf(w, "b9s web is serving %s\n\n", project)
	fmt.Fprintf(w, "  This machine  %s\n", pairURL(local, auth))

	phone := ""
	if name := tailscaleDNSName(); name != "" {
		phone = pairURL("https://"+name, auth)
		if web.IsLoopbackAddr(addr) {
			fmt.Fprintf(w, "\n  Phone         run once:  tailscale serve --bg %s\n", port)
			fmt.Fprintf(w, "                then open: %s\n", phone)
		} else {
			phone = pairURL("http://"+net.JoinHostPort(name, port), auth)
			fmt.Fprintf(w, "  Phone         %s\n", phone)
		}
	}
	if auth != nil {
		fmt.Fprintln(w, "\n  The link pairs a browser for 90 days. b9s web --new-token unpairs them all.")
	}
	if phone != "" {
		printQR(w, phone)
	}
	fmt.Fprintln(w, "\nPress Ctrl+C to stop.")
}

// tailscaleDNSName is this machine's tailnet name, or "" without Tailscale.
func tailscaleDNSName() string {
	if os.Getenv("B9S_TEST_MODE") != "" {
		return ""
	}
	path, err := exec.LookPath("tailscale")
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "status", "--json").Output()
	if err != nil {
		return ""
	}
	var status struct {
		Self struct {
			DNSName string `json:"DNSName"`
		} `json:"Self"`
	}
	if json.Unmarshal(out, &status) != nil {
		return ""
	}
	return strings.TrimSuffix(status.Self.DNSName, ".")
}

// printQR draws url as a terminal QR code when qrencode is installed, so a
// phone camera can open it.
func printQR(w io.Writer, url string) {
	f, ok := w.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) || os.Getenv("B9S_TEST_MODE") != "" {
		return
	}
	path, err := exec.LookPath("qrencode")
	if err != nil {
		fmt.Fprintln(w, "  Install qrencode to show this link as a QR code.")
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, path, "-t", "ANSIUTF8", url).Output()
	if err == nil {
		fmt.Fprintf(w, "\n%s", out)
	}
}
