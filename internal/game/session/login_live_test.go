package session

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
)

// TestLiveLoginGate is the P2 exit gate. It intentionally requires explicit
// credentials so routine test runs never consume a short-lived login code.
func TestLiveLoginGate(t *testing.T) {
	code := strings.TrimSpace(os.Getenv("FARM_P2_05_LOGIN_CODE"))
	if code == "" {
		t.Skip("set FARM_P2_05_LOGIN_CODE and FARM_P2_05_FRIEND_GID to run the P2 live gate")
	}
	friendGID, err := strconv.ParseInt(strings.TrimSpace(os.Getenv("FARM_P2_05_FRIEND_GID")), 10, 64)
	if err != nil || friendGID <= 0 {
		t.Fatal("FARM_P2_05_FRIEND_GID must be a positive integer")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 31*time.Minute)
	defer cancel()
	gameSession, err := LoginWithOptions(ctx, code, Options{
		AccountID: strings.TrimSpace(os.Getenv("FARM_P2_05_ACCOUNT_ID")),
		UIN:       strings.TrimSpace(os.Getenv("FARM_P2_05_UIN")),
		Logger: func(level, message string) {
			t.Logf("ACE %s: %s", level, message)
		},
	})
	if err != nil {
		t.Fatalf("real login handshake: %v", err)
	}
	defer gameSession.Close()
	if !gameSession.Online() || gameSession.OpenID == "" || gameSession.GID == 0 {
		t.Fatalf("invalid online identity: openid=%q gid=%d online=%v", gameSession.OpenID, gameSession.GID, gameSession.Online())
	}
	if gameSession.InitialLands() == nil {
		t.Fatal("real AllLands did not return a reply")
	}

	if _, err := gameSession.SendMsg(ctx, transport.Command{
		ServiceName: "gamepb.visitpb.VisitService",
		MethodName:  "Enter",
		Response:    new(pb.EnterReply),
	}, &pb.EnterRequest{HostGid: friendGID, Reason: int32(pb.EnterReason_ENTER_REASON_FRIEND)}); err != nil {
		t.Fatalf("enter friend farm: %v", err)
	}
	if _, err := gameSession.SendMsg(ctx, transport.Command{
		ServiceName: "gamepb.visitpb.VisitService",
		MethodName:  "Leave",
		Response:    new(pb.LeaveReply),
	}, &pb.LeaveRequest{HostGid: friendGID}); err != nil {
		t.Fatalf("leave friend farm: %v", err)
	}

	deadline := time.NewTimer(30 * time.Minute)
	defer deadline.Stop()
	check := time.NewTicker(5 * time.Second)
	defer check.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("live gate context ended: %v", ctx.Err())
		case <-check.C:
			status := gameSession.ACEStatus()
			if !gameSession.Online() {
				t.Fatal("game session disconnected during 30-minute ACE gate")
			}
			if !status.Running {
				t.Fatal("ACE lifecycle stopped during 30-minute gate")
			}
			if status.LastError != "" || status.Runtime.LastError != "" {
				t.Fatalf("ACE warning during live gate: service=%q runtime=%q", status.LastError, status.Runtime.LastError)
			}
		case <-deadline.C:
			status := gameSession.ACEStatus()
			if status.UserHeartbeatTicks == 0 || status.Runtime.HeartbeatTicks == 0 || status.Runtime.ProcessTicks == 0 || status.Runtime.StatusReports == 0 || status.Runtime.FunctionChecks == 0 {
				t.Fatalf("ACE lifecycle counters are incomplete after 30 minutes: %+v", status)
			}
			return
		}
	}
}
