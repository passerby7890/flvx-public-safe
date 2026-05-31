package handler

import (
	"path/filepath"
	"testing"
	"time"

	"go-backend/internal/store/repo"
)

func TestRunTunnelForwardEntryConsistencyRepairsMissingForwardPorts(t *testing.T) {
	r, err := repo.Open(filepath.Join(t.TempDir(), "forward-entry-consistency.db"))
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = r.Close() })

	h := &Handler{repo: r}
	now := time.Now().UnixMilli()

	if err := r.DB().Exec(`
		INSERT INTO node(id, name, secret, server_ip, port, created_time, updated_time, status, tcp_listen_addr, udp_listen_addr, is_remote, inx)
		VALUES
			(101, 'entry-a', 'secret-a', '10.0.0.1', '12000-12010', ?, ?, 1, '[::]', '[::]', 0, 1),
			(102, 'entry-b', 'secret-b', '10.0.0.2', '12000-12010', ?, ?, 1, '[::]', '[::]', 0, 2)
	`, now, now, now, now).Error; err != nil {
		t.Fatalf("insert nodes: %v", err)
	}

	if err := r.DB().Exec(`
		INSERT INTO tunnel(id, name, traffic_ratio, type, protocol, flow, created_time, updated_time, status, inx)
		VALUES(1, 'consistency-tunnel', 1.0, 1, 'tls', 1, ?, ?, 1, 1)
	`, now, now).Error; err != nil {
		t.Fatalf("insert tunnel: %v", err)
	}

	if err := r.DB().Exec(`
		INSERT INTO chain_tunnel(tunnel_id, chain_type, node_id, inx, protocol)
		VALUES
			(1, '1', 101, 1, 'tls'),
			(1, '1', 102, 2, 'tls')
	`).Error; err != nil {
		t.Fatalf("insert chain_tunnel: %v", err)
	}

	if err := r.DB().Exec(`
		INSERT INTO forward(id, user_id, user_name, name, tunnel_id, mode, remote_addr, strategy, created_time, updated_time, status, inx)
		VALUES(1, 1, 'tester', 'forward-a', 1, 'direct', '127.0.0.1:8080', 'fifo', ?, ?, 1, 1)
	`, now, now).Error; err != nil {
		t.Fatalf("insert forward: %v", err)
	}

	if err := r.DB().Exec(`
		INSERT INTO forward_port(forward_id, node_id, port)
		VALUES(1, 101, 12001)
	`).Error; err != nil {
		t.Fatalf("insert forward_port: %v", err)
	}

	h.runTunnelForwardEntryConsistency()

	ports, err := h.listForwardPorts(1)
	if err != nil {
		t.Fatalf("list forward ports: %v", err)
	}
	if len(ports) != 2 {
		t.Fatalf("expected 2 forward_port rows after repair, got %d: %+v", len(ports), ports)
	}

	nodeIDs := forwardPortNodeIDs(ports)
	if !sameInt64Set(nodeIDs, []int64{101, 102}) {
		t.Fatalf("expected repaired node ids [101 102], got %v", nodeIDs)
	}

	for _, fp := range ports {
		if fp.Port != 12001 {
			t.Fatalf("expected repaired port to preserve 12001, got %+v", ports)
		}
	}
}
