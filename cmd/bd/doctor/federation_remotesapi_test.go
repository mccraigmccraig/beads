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

func TestResolveFederationRemotesAPITargetUsesTargetPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("BEADS_DOLT_SHARED_SERVER", "")
	t.Setenv("BEADS_DOLT_SERVER_PORT", "")
	t.Setenv("BEADS_DOLT_REMOTESAPI_PORT", "")
	sharedRoot := filepath.Join(home, ".beads", "shared-server")
	t.Setenv("BEADS_SHARED_SERVER_DIR", sharedRoot)
	if err := os.MkdirAll(filepath.Join(sharedRoot, "dolt"), 0o700); err != nil {
		t.Fatal(err)
	}
	activeDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(activeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(activeDir, "config.yaml"),
		[]byte("dolt:\n  shared-server: false\n  port: 16666\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BEADS_DIR", activeDir)
	config.ResetForTesting()
	t.Cleanup(config.ResetForTesting)
	if err := config.SetUserYamlConfig("dolt.remotesapi-port", "8123"); err != nil {
		t.Fatal(err)
	}
	if err := config.Initialize(); err != nil {
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
	nonSharedCfg.DoltMode = configfile.DoltModeServer
	nonSharedCfg.DoltServerPort = 15555
	nonSharedCfg.DoltRemotesAPIPort = 7001
	if err := nonSharedCfg.Save(nonSharedDir); err != nil {
		t.Fatal(err)
	}
	nonShared, err := resolveFederationRemotesAPITarget(nonSharedDir)
	if err != nil {
		t.Fatal(err)
	}
	wantNonSharedData := filepath.Join(nonSharedDir, "dolt")
	if nonShared.SharedMode ||
		nonShared.DoltPath != wantNonSharedData ||
		nonShared.ServerDir != nonSharedDir ||
		nonShared.SQLConfig.ServerPort != 15555 ||
		nonShared.SQLConfig.ServerPortSource != doltserver.PortSourceMetadataJSON ||
		nonShared.SQLConfig.ServerPortSharedServer ||
		nonShared.RemotesAPIPort != 7001 {
		t.Fatalf("non-shared target = %+v, want data:%q state:%q sql:15555 shared-provenance:false rapi:7001", nonShared, wantNonSharedData, nonSharedDir)
	}

	sharedDir := filepath.Join(t.TempDir(), ".beads")
	if err := os.MkdirAll(sharedDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sharedDir, "config.yaml"), []byte("dolt:\n  shared-server: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shared, err := resolveFederationRemotesAPITarget(sharedDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(sharedDir, "dolt")); !os.IsNotExist(err) {
		t.Fatalf("precondition: shared project unexpectedly has local dolt dir: %v", err)
	}
	if !shared.SharedMode ||
		shared.DoltPath != filepath.Join(sharedRoot, "dolt") ||
		shared.ServerDir != sharedRoot ||
		shared.SQLConfig.ServerPort != doltserver.DefaultSharedServerPort ||
		shared.SQLConfig.ServerPortSource != doltserver.PortSourceSharedServerDefault ||
		!shared.SQLConfig.ServerPortSharedServer ||
		shared.RemotesAPIPort != 8123 {
		t.Fatalf("shared target = %+v, want data:%q state:%q sql:%d shared-provenance:true rapi:8123", shared, filepath.Join(sharedRoot, "dolt"), sharedRoot, doltserver.DefaultSharedServerPort)
	}
}
