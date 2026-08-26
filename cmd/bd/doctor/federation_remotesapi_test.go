package doctor

import (
	"net"
	"strconv"
	"strings"
	"testing"

	"github.com/steveyegge/beads/internal/doltserver"
)

func TestFederationRemotesAPICheckStates(t *testing.T) {
	state := &doltserver.State{Running: true, PID: 42}

	disabled := federationRemotesAPICheck(state, 0)
	if disabled.Status != StatusOK || !strings.Contains(disabled.Message, "Disabled") {
		t.Fatalf("disabled check = %+v, want OK disabled", disabled)
	}

	unreachable := federationRemotesAPICheck(state, 1)
	if unreachable.Status != StatusError || !strings.Contains(unreachable.Message, "port 1") {
		t.Fatalf("unreachable check = %+v, want port error", unreachable)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port, err := strconv.Atoi(strings.TrimPrefix(listener.Addr().String(), "127.0.0.1:"))
	if err != nil {
		t.Fatal(err)
	}
	reachable := federationRemotesAPICheck(state, port)
	if reachable.Status != StatusOK || !strings.Contains(reachable.Message, strconv.Itoa(port)) {
		t.Fatalf("reachable check = %+v, want OK port %d", reachable, port)
	}
}
