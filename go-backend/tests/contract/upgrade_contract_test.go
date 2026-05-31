package contract_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"go-backend/internal/auth"
)

func TestNodeReleasesPatchedContract(t *testing.T) {
	secret := "upgrade-contract-jwt-secret"
	router, _ := setupContractRouter(t, secret)

	adminToken, err := auth.GenerateToken(1, "admin_user", 0, secret)
	if err != nil {
		t.Fatalf("generate admin token: %v", err)
	}

	out := requestContractEnvelope(
		t,
		router,
		adminToken,
		"/api/v1/node/releases",
		map[string]interface{}{"channel": "patched"},
	)
	if out.Code != 0 {
		t.Fatalf("node releases failed: code=%d msg=%q", out.Code, out.Msg)
	}

	rows := mustContractSlice(t, out.Data, "node releases data")
	if len(rows) == 0 {
		t.Fatalf("expected at least one patched release, got empty list")
	}

	first, ok := rows[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected release row map, got %T", rows[0])
	}
	if got := strings.TrimSpace(contractValueAsString(first["channel"])); got != "patched" {
		t.Fatalf("expected channel=patched, got %q", got)
	}
	if version := strings.TrimSpace(contractValueAsString(first["version"])); version == "" {
		t.Fatalf("expected non-empty patched version")
	}
}

func TestNodeUpgradeBatchRollbackContracts(t *testing.T) {
	secret := "upgrade-command-contract-jwt-secret"
	router, repo := setupContractRouter(t, secret)
	server := httptest.NewServer(router)
	defer server.Close()

	adminToken, err := auth.GenerateToken(1, "admin_user", 0, secret)
	if err != nil {
		t.Fatalf("generate admin token: %v", err)
	}

	releasesOut := requestContractEnvelope(
		t,
		router,
		adminToken,
		"/api/v1/node/releases",
		map[string]interface{}{"channel": "patched"},
	)
	if releasesOut.Code != 0 {
		t.Fatalf("node releases failed: code=%d msg=%q", releasesOut.Code, releasesOut.Msg)
	}
	releaseRows := mustContractSlice(t, releasesOut.Data, "node releases data")
	if len(releaseRows) == 0 {
		t.Fatalf("expected patched release list to be non-empty")
	}
	releaseRow, ok := releaseRows[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected release row map, got %T", releaseRows[0])
	}
	expectedVersion := strings.TrimSpace(contractValueAsString(releaseRow["version"]))
	if expectedVersion == "" {
		t.Fatalf("expected non-empty patched version from /node/releases")
	}

	nodeID := insertContractNode(t, repo, "upgrade-contract-node", "198.51.100.88", "46000-46010", "upgrade-contract-node-secret", 1)

	var mu sync.Mutex
	commandCounts := make(map[string]int)
	var upgradePayloads []map[string]interface{}

	stopNode := startMockNodeSessionWithCommandRecorder(t, server.URL, "upgrade-contract-node-secret", func(cmdType string, data json.RawMessage) (bool, string) {
		key := strings.ToLower(strings.TrimSpace(cmdType))
		mu.Lock()
		commandCounts[key]++
		if strings.EqualFold(strings.TrimSpace(cmdType), "UpgradeAgent") {
			var payload map[string]interface{}
			if err := json.Unmarshal(data, &payload); err == nil {
				upgradePayloads = append(upgradePayloads, payload)
			}
		}
		mu.Unlock()
		return false, ""
	})
	defer stopNode()

	waitNodeStatus(t, repo, nodeID, 1)

	upgradeOut := requestContractEnvelope(
		t,
		router,
		adminToken,
		"/api/v1/node/upgrade",
		map[string]interface{}{
			"id":      nodeID,
			"channel": "patched",
		},
	)
	if upgradeOut.Code != 0 {
		t.Fatalf("node upgrade failed: code=%d msg=%q", upgradeOut.Code, upgradeOut.Msg)
	}
	upgradeData, ok := upgradeOut.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected node upgrade data map, got %T", upgradeOut.Data)
	}
	gotVersion := strings.TrimSpace(contractValueAsString(upgradeData["version"]))
	if gotVersion != expectedVersion {
		t.Fatalf("expected upgrade version=%q, got %q", expectedVersion, gotVersion)
	}

	batchOut := requestContractEnvelope(
		t,
		router,
		adminToken,
		"/api/v1/node/batch-upgrade",
		map[string]interface{}{
			"ids":     []int64{nodeID},
			"channel": "patched",
		},
	)
	if batchOut.Code != 0 {
		t.Fatalf("node batch upgrade failed: code=%d msg=%q", batchOut.Code, batchOut.Msg)
	}
	batchData, ok := batchOut.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("expected node batch upgrade data map, got %T", batchOut.Data)
	}
	results := mustContractSlice(t, batchData["results"], "node batch upgrade results")
	if len(results) != 1 {
		t.Fatalf("expected 1 batch result, got %d", len(results))
	}
	resultRow, ok := results[0].(map[string]interface{})
	if !ok {
		t.Fatalf("expected batch result row map, got %T", results[0])
	}
	if success, ok := resultRow["success"].(bool); !ok || !success {
		t.Fatalf("expected batch upgrade success=true, got %v", resultRow["success"])
	}

	rollbackOut := requestContractEnvelope(
		t,
		router,
		adminToken,
		"/api/v1/node/rollback",
		map[string]interface{}{"id": nodeID},
	)
	if rollbackOut.Code != 0 {
		t.Fatalf("node rollback failed: code=%d msg=%q", rollbackOut.Code, rollbackOut.Msg)
	}

	mu.Lock()
	defer mu.Unlock()
	if commandCounts["upgradeagent"] < 2 {
		t.Fatalf("expected at least 2 UpgradeAgent commands, got %d (%v)", commandCounts["upgradeagent"], commandCounts)
	}
	if commandCounts["rollbackagent"] < 1 {
		t.Fatalf("expected at least 1 RollbackAgent command, got %d (%v)", commandCounts["rollbackagent"], commandCounts)
	}
	if len(upgradePayloads) == 0 {
		t.Fatalf("expected upgrade payloads to be recorded")
	}
	for _, payload := range upgradePayloads {
		downloadURL := strings.TrimSpace(contractValueAsString(payload["downloadUrl"]))
		checksumURL := strings.TrimSpace(contractValueAsString(payload["checksumUrl"]))
		if !strings.Contains(downloadURL, "/agent/gost-") {
			t.Fatalf("unexpected downloadUrl in upgrade payload: %q", downloadURL)
		}
		if !strings.Contains(checksumURL, "/agent/gost-") || !strings.HasSuffix(checksumURL, ".sha256") {
			t.Fatalf("unexpected checksumUrl in upgrade payload: %q", checksumURL)
		}
	}
}

