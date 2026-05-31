package handler

import (
	"fmt"
	"strings"
	"time"

	"go-backend/internal/store/model"
)

const entryDemuxSyncTimeout = 2 * time.Minute

type entryDemuxSyncPayload struct {
	Enabled   bool                     `json:"enabled"`
	Listeners []entryDemuxSyncListener `json:"listeners"`
}

type entryDemuxSyncListener struct {
	Name       string               `json:"name"`
	Listen     string               `json:"listen"`
	CoverAddr  string               `json:"coverAddr"`
	AnyTLSAddr string               `json:"anytlsAddr"`
	Certs      []entryDemuxSyncCert `json:"certs"`
	Metadata   map[string]string    `json:"metadata,omitempty"`
}

type entryDemuxSyncCert struct {
	Profile       string `json:"profile"`
	FullchainPEM  string `json:"fullchainPem"`
	PrivateKeyPEM string `json:"privateKeyPem"`
}

func (h *Handler) syncEntryDemuxToNode(nodeID int64, force bool) error {
	_, err := h.syncEntryDemuxToNodeWithResult(nodeID, force)
	return err
}

func (h *Handler) syncEntryDemuxToNodeWithResult(nodeID int64, force bool) (interface{}, error) {
	payload, err := h.buildEntryDemuxSyncPayload(nodeID)
	if err != nil {
		return nil, err
	}
	if !force && !payload.Enabled {
		return nil, nil
	}
	res, err := h.sendNodeCommandWithTimeout(nodeID, "SyncEntryDemux", payload, entryDemuxSyncTimeout, false, false)
	if err != nil {
		if !payload.Enabled && isUnsupportedEntryDemuxCommand(err) {
			return nil, nil
		}
		if res.Data != nil {
			return res.Data, err
		}
		return nil, err
	}
	return res.Data, nil
}

func isUnsupportedEntryDemuxCommand(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(msg, "syncentrydemux")
}

func (h *Handler) buildEntryDemuxSyncPayload(nodeID int64) (*entryDemuxSyncPayload, error) {
	payload := &entryDemuxSyncPayload{Enabled: false}
	if h == nil || h.repo == nil || nodeID <= 0 {
		return payload, nil
	}

	service, err := h.repo.GetNodeCoverService(nodeID)
	if err != nil {
		return nil, err
	}
	if service == nil || service.Enabled != 1 || !coverServiceReadyForSharedSNI(service) {
		return payload, nil
	}
	if service.PublicPort != defaultCoverPublicPort {
		return payload, nil
	}
	localListen := strings.TrimSpace(service.LocalListen)
	if localListen == "" {
		localListen = defaultCoverLocalListen
	}
	if err := validateCoverLocalListen(localListen); err != nil {
		return nil, err
	}

	forwards, err := h.repo.ListSNIForwardsOnNode(nodeID)
	if err != nil {
		return nil, err
	}
	forwards = uniqueSNIForwardRecords(forwards)

	listeners := make([]entryDemuxSyncListener, 0, len(forwards))
	for _, forward := range forwards {
		listener, ok, err := h.buildEntryDemuxListener(forward, localListen)
		if err != nil {
			return nil, err
		}
		if ok {
			listeners = append(listeners, listener)
		}
	}
	if len(listeners) == 0 {
		return payload, nil
	}
	payload.Enabled = true
	payload.Listeners = listeners
	return payload, nil
}

func (h *Handler) buildEntryDemuxListener(forward model.ForwardRecord, localListen string) (entryDemuxSyncListener, bool, error) {
	if forward.ID <= 0 || forward.TunnelID <= 0 {
		return entryDemuxSyncListener{}, false, nil
	}
	hosts, err := parseSNIForwardHosts(forward.SniRules)
	if err != nil || len(hosts) == 0 {
		return entryDemuxSyncListener{}, false, nil
	}
	profiles, err := h.repo.ListEnabledCoverDomainProfilesByTunnelIDs([]int64{forward.TunnelID})
	if err != nil {
		return entryDemuxSyncListener{}, false, err
	}
	if len(profiles) == 0 {
		return entryDemuxSyncListener{}, false, nil
	}

	certs := make([]entryDemuxSyncCert, 0, len(profiles))
	seenCerts := make(map[string]struct{}, len(profiles))
	coveredHosts := make([]string, 0, len(hosts))
	for _, profile := range profiles {
		domains, err := parseCoverProfileDomains(profile.Domains)
		if err != nil || len(domains) == 0 {
			continue
		}
		matched := false
		for _, host := range hosts {
			if sniHostCoveredByProfile(host, domains) {
				matched = true
				coveredHosts = appendUniqueString(coveredHosts, host)
			}
		}
		if !matched {
			continue
		}
		certProfile := strings.TrimSpace(profile.CertProfile)
		if certProfile == "" {
			certProfile = inferCoverCertProfile(strings.Join(domains, "\n"))
		}
		if certProfile == "" {
			return entryDemuxSyncListener{}, false, fmt.Errorf("cover profile %s has no certificate profile", profile.Name)
		}
		if _, ok := seenCerts[certProfile]; ok {
			continue
		}
		fullchain, key, err := readCoverCertMaterial(certProfile)
		if err != nil {
			return entryDemuxSyncListener{}, false, fmt.Errorf("certificate %s read failed: %w", certProfile, err)
		}
		seenCerts[certProfile] = struct{}{}
		certs = append(certs, entryDemuxSyncCert{
			Profile:       certProfile,
			FullchainPEM:  fullchain,
			PrivateKeyPEM: key,
		})
	}
	if len(certs) == 0 {
		return entryDemuxSyncListener{}, false, nil
	}

	hiddenPort := 20000 + (forward.ID % 40000)
	return entryDemuxSyncListener{
		Name:       fmt.Sprintf("entry_demux_%d", forward.ID),
		Listen:     sniTLSDemuxAddr(forward.ID),
		CoverAddr:  localListen,
		AnyTLSAddr: fmt.Sprintf("127.0.0.1:%d", hiddenPort),
		Certs:      certs,
		Metadata: map[string]string{
			"forwardId": fmt.Sprintf("%d", forward.ID),
			"hosts":     strings.Join(coveredHosts, ","),
		},
	}, true, nil
}

func uniqueSNIForwardRecords(rows []model.ForwardRecord) []model.ForwardRecord {
	if len(rows) <= 1 {
		return rows
	}
	out := make([]model.ForwardRecord, 0, len(rows))
	seen := make(map[int64]struct{}, len(rows))
	for _, row := range rows {
		if row.ID <= 0 {
			continue
		}
		if _, ok := seen[row.ID]; ok {
			continue
		}
		seen[row.ID] = struct{}{}
		out = append(out, row)
	}
	return out
}

func appendUniqueString(items []string, item string) []string {
	item = strings.TrimSpace(item)
	if item == "" {
		return items
	}
	for _, existing := range items {
		if existing == item {
			return items
		}
	}
	return append(items, item)
}
