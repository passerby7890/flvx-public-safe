package repo

import (
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"go-backend/internal/store/model"
)

func TestListNodeDependentIDsAndDeleteForwardSLAStates(t *testing.T) {
	r, err := Open(filepath.Join(t.TempDir(), "node-dependency.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	now := time.Now().UnixMilli()
	if err := r.DB().Create(&model.Node{
		ID:            1,
		Name:          "node-a",
		Secret:        "secret-a",
		ServerIP:      "10.0.0.1",
		Port:          "1000-1001",
		CreatedTime:   now,
		UpdatedTime:   sql.NullInt64{Int64: now, Valid: true},
		Status:        1,
		TCPListenAddr: "[::]",
		UDPListenAddr: "[::]",
		Inx:           1,
		IsRemote:      0,
	}).Error; err != nil {
		t.Fatalf("insert node: %v", err)
	}

	tunnels := []model.Tunnel{
		{
			ID:           1,
			Name:         "tunnel-active",
			TrafficRatio: 1,
			Type:         1,
			Protocol:     "tls",
			Flow:         1,
			CreatedTime:  now,
			UpdatedTime:  now,
			Status:       1,
			Inx:          1,
		},
		{
			ID:           2,
			Name:         "tunnel-paused",
			TrafficRatio: 1,
			Type:         1,
			Protocol:     "tls",
			Flow:         1,
			CreatedTime:  now,
			UpdatedTime:  now,
			Status:       0,
			Inx:          2,
		},
	}
	for i := range tunnels {
		if err := r.DB().Create(&tunnels[i]).Error; err != nil {
			t.Fatalf("insert tunnel %d: %v", tunnels[i].ID, err)
		}
	}

	forwards := []model.Forward{
		{
			ID:          1,
			UserID:      1,
			UserName:    "user",
			Name:        "forward-active",
			TunnelID:    1,
			Mode:        "direct",
			RemoteAddr:  "127.0.0.1:8080",
			SniRules:    "",
			Strategy:    "fifo",
			CreatedTime: now,
			UpdatedTime: now,
			Status:      1,
			Inx:         1,
		},
		{
			ID:          2,
			UserID:      1,
			UserName:    "user",
			Name:        "forward-paused",
			TunnelID:    2,
			Mode:        "direct",
			RemoteAddr:  "127.0.0.1:8081",
			SniRules:    "",
			Strategy:    "fifo",
			CreatedTime: now,
			UpdatedTime: now,
			Status:      0,
			Inx:         2,
		},
	}
	for i := range forwards {
		if err := r.DB().Create(&forwards[i]).Error; err != nil {
			t.Fatalf("insert forward %d: %v", forwards[i].ID, err)
		}
	}

	if err := r.DB().Exec(`INSERT INTO chain_tunnel(tunnel_id, chain_type, node_id, inx, protocol) VALUES(?, ?, ?, ?, ?)`, 1, "1", 1, 1, "tls").Error; err != nil {
		t.Fatalf("insert chain_tunnel active: %v", err)
	}
	if err := r.DB().Exec(`INSERT INTO chain_tunnel(tunnel_id, chain_type, node_id, inx, protocol) VALUES(?, ?, ?, ?, ?)`, 2, "1", 1, 2, "tls").Error; err != nil {
		t.Fatalf("insert chain_tunnel paused: %v", err)
	}
	if err := r.DB().Exec(`INSERT INTO forward_port(forward_id, node_id, port) VALUES(?, ?, ?)`, 1, 1, 443).Error; err != nil {
		t.Fatalf("insert forward_port active: %v", err)
	}
	if err := r.DB().Exec(`INSERT INTO forward_port(forward_id, node_id, port) VALUES(?, ?, ?)`, 2, 1, 444).Error; err != nil {
		t.Fatalf("insert forward_port paused: %v", err)
	}

	if err := r.DB().Create(&model.ForwardSLAState{
		ForwardID:           1,
		ForwardName:         "forward-active",
		UserID:              1,
		TunnelID:            1,
		Mode:                "direct",
		OverallStatus:       "healthy",
		EntryStatus:         "healthy",
		TargetStatus:        "healthy",
		EntryTotal:          1,
		EntryHealthy:        1,
		TargetTotal:         1,
		TargetHealthy:       1,
		EntryCheckedAt:      now,
		TargetCheckedAt:     now,
		CheckedAt:           now,
		Uptime24h:           1,
		Samples24h:          1,
		ConsecutiveFailures: 0,
		LastHealthyAt:       now,
		FailureKind:         "",
		Reason:              "",
		CreatedTime:         now,
		UpdatedTime:         now,
	}).Error; err != nil {
		t.Fatalf("insert sla state 1: %v", err)
	}
	if err := r.DB().Create(&model.ForwardSLAState{
		ForwardID:           2,
		ForwardName:         "forward-paused",
		UserID:              1,
		TunnelID:            2,
		Mode:                "direct",
		OverallStatus:       "healthy",
		EntryStatus:         "healthy",
		TargetStatus:        "healthy",
		EntryTotal:          1,
		EntryHealthy:        1,
		TargetTotal:         1,
		TargetHealthy:       1,
		EntryCheckedAt:      now,
		TargetCheckedAt:     now,
		CheckedAt:           now,
		Uptime24h:           1,
		Samples24h:          1,
		ConsecutiveFailures: 0,
		LastHealthyAt:       now,
		FailureKind:         "",
		Reason:              "",
		CreatedTime:         now,
		UpdatedTime:         now,
	}).Error; err != nil {
		t.Fatalf("insert sla state 2: %v", err)
	}
	if err := r.DB().Create(&model.ForwardSLASnapshot{
		ForwardID:       1,
		ForwardName:     "forward-active",
		UserID:          1,
		TunnelID:        1,
		Mode:            "direct",
		OverallStatus:   "healthy",
		EntryStatus:     "healthy",
		TargetStatus:    "healthy",
		EntryTotal:      1,
		EntryHealthy:    1,
		TargetTotal:     1,
		TargetHealthy:   1,
		FailureKind:     "",
		Reason:          "",
		EntryCheckedAt:  now,
		TargetCheckedAt: now,
		Timestamp:       now,
	}).Error; err != nil {
		t.Fatalf("insert sla snapshot: %v", err)
	}

	activeTunnels, err := r.ListActiveTunnelIDsByNode(1)
	if err != nil {
		t.Fatalf("list active tunnels: %v", err)
	}
	if !reflect.DeepEqual(activeTunnels, []int64{1}) {
		t.Fatalf("unexpected active tunnels: %v", activeTunnels)
	}

	allTunnels, err := r.ListTunnelIDsByNode(1)
	if err != nil {
		t.Fatalf("list all tunnels: %v", err)
	}
	if !reflect.DeepEqual(allTunnels, []int64{1, 2}) {
		t.Fatalf("unexpected all tunnels: %v", allTunnels)
	}

	activeForwards, err := r.ListActiveForwardIDsByNode(1)
	if err != nil {
		t.Fatalf("list active forwards: %v", err)
	}
	if !reflect.DeepEqual(activeForwards, []int64{1}) {
		t.Fatalf("unexpected active forwards: %v", activeForwards)
	}

	allForwards, err := r.ListForwardIDsByNode(1)
	if err != nil {
		t.Fatalf("list all forwards: %v", err)
	}
	if !reflect.DeepEqual(allForwards, []int64{1, 2}) {
		t.Fatalf("unexpected all forwards: %v", allForwards)
	}

	if err := r.DeleteForwardSLAStates([]int64{1, 2}); err != nil {
		t.Fatalf("delete sla states: %v", err)
	}

	states, err := r.GetForwardSLAStates([]int64{1, 2})
	if err != nil {
		t.Fatalf("get sla states: %v", err)
	}
	if len(states) != 0 {
		t.Fatalf("expected sla states to be deleted, got %+v", states)
	}

	window, err := r.GetForwardSLAWindowSummaries([]int64{1, 2}, now-1000, now+1000)
	if err != nil {
		t.Fatalf("get sla window summaries: %v", err)
	}
	if len(window) != 1 || window[1].Samples != 1 {
		t.Fatalf("expected snapshot history to remain, got %+v", window)
	}
}
