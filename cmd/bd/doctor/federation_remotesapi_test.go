package doctor

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/steveyegge/beads/internal/config"
	"github.com/steveyegge/beads/internal/configfile"
	"github.com/steveyegge/beads/internal/doltserver"
)

func TestFederationRemotesAPICheckStates(t *testing.T) {
	state := &doltserver.State{Running: true, PID: 42}

	disabled := federationRemotesAPICheck(state, 0, true)
	if disabled.Status != StatusError || !strings.Contains(disabled.Message, "Disabled") {
		t.Fatalf("disabled check = %+v, want federation error", disabled)
	}

	unreachable := federationRemotesAPICheck(state, 1, true)
	if unreachable.Status != StatusError || !strings.Contains(unreachable.Message, "port 1") {
		t.Fatalf("unreachable check = %+v, want port error", unreachable)
	}
	if !strings.Contains(unreachable.Fix, "bd dolt set remotesapi-port") {
		t.Fatalf("shared unreachable fix = %q, want bd shared-server guidance", unreachable.Fix)
	}
	nonShared := federationRemotesAPICheck(state, 1, false)
	if strings.Contains(nonShared.Fix, "shared") || !strings.Contains(nonShared.Fix, "Dolt sql-server") {
		t.Fatalf("non-shared unreachable fix = %q, want neutral server guidance", nonShared.Fix)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	reachable := federationRemotesAPICheck(state, port, true)
	if reachable.Status != StatusOK || !strings.Contains(reachable.Message, strconv.Itoa(port)) {
		t.Fatalf("reachable check = %+v, want OK port %d", reachable, port)
	}
}

func TestFederationRemotesAPITargetUsesTargetMode(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("BEADS_SHARED_SERVER_DIR", filepath.Join(home, ".beads", "shared-server"))
	t.Setenv("BEADS_DOLT_REMOTESAPI_PORT", "")
	config.ResetForTesting()
	t.Cleanup(config.ResetForTesting)
	if err := config.SetUserYamlConfig("dolt.remotesapi-port", "8123"); err != nil {
		t.Fatal(err)
	}

	nonSharedDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(nonSharedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nonSharedDir, "config.yaml"), []byte("dolt:\n  shared-server: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	nonSharedCfg := configfile.DefaultConfig()
	nonSharedCfg.DoltRemotesAPIPort = 7001
	if err := nonSharedCfg.Save(nonSharedDir); err != nil {
		t.Fatal(err)
	}

	// The current process says shared; the explicit non-shared target must win.
	t.Setenv("BEADS_DOLT_SHARED_SERVER", "1")
	shared, serverDir, port := federationRemotesAPITarget(nonSharedDir)
	if shared || serverDir != nonSharedDir || port != 7001 {
		t.Fatalf("non-shared target = shared:%v dir:%q port:%d, want false/%q/7001", shared, serverDir, port, nonSharedDir)
	}

	sharedDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(sharedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "config.yaml"), []byte("dolt:\n  shared-server: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	// The current process says non-shared; the explicit shared target must win.
	t.Setenv("BEADS_DOLT_SHARED_SERVER", "0")
	shared, serverDir, port = federationRemotesAPITarget(sharedDir)
	wantServerDir, err := doltserver.SharedServerDir()
	if err != nil {
		t.Fatal(err)
	}
	if !shared || serverDir != wantServerDir || port != 8123 {
		t.Fatalf("shared target = shared:%v dir:%q port:%d, want true/%q/8123", shared, serverDir, port, wantServerDir)
	}
}
