package handler

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go-backend/internal/http/response"
	"go-backend/internal/store/model"
)

const (
	defaultCoverPublicPort  = 443
	defaultCoverLocalListen = "127.0.0.1:10443"
	maxCoverHTMLBytes       = 512 * 1024
	coverSyncTimeout        = 2 * time.Minute
)

type coverDomainProfileRequest struct {
	ID              int64  `json:"id"`
	Name            string `json:"name"`
	Enabled         int    `json:"enabled"`
	SiteLabel       string `json:"siteLabel"`
	Domains         string `json:"domains"`
	CertProfile     string `json:"certProfile"`
	DNSProvider     string `json:"dnsProvider"`
	DNSProfile      string `json:"dnsProfile"`
	TemplateProfile string `json:"templateProfile"`
	UpstreamOrigin  string `json:"upstreamOrigin"`
	StaticHTML      string `json:"staticHtml"`
	CreatedTime     int64  `json:"createdTime"`
	UpdatedTime     int64  `json:"updatedTime"`
}

type tunnelCoverSelection struct {
	TunnelID   int64   `json:"tunnelId"`
	ProfileIDs []int64 `json:"profileIds"`
}

type nodeCoverServiceRequest struct {
	ID           int64  `json:"id"`
	NodeID       int64  `json:"nodeId"`
	Enabled      int    `json:"enabled"`
	PublicPort   int    `json:"publicPort"`
	LocalListen  string `json:"localListen"`
	LastSyncTime int64  `json:"lastSyncTime"`
	LastStatus   string `json:"lastStatus"`
	CreatedTime  int64  `json:"createdTime"`
	UpdatedTime  int64  `json:"updatedTime"`
}

type coverNodeSyncRequest struct {
	NodeID int64 `json:"nodeId"`
}

type coverCertFiles struct {
	Fullchain string
	Key       string
}

type builtInCoverProfile struct {
	Name        string
	SiteLabel   string
	Domains     string
	CertProfile string
}

func (h *Handler) coverProfileList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("method not allowed"))
		return
	}
	if err := h.ensureBuiltInCoverProfiles(); err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}
	rows, err := h.repo.ListCoverDomainProfiles()
	if err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}
	response.WriteJSON(w, response.OK(rows))
}

func (h *Handler) coverProfileGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("method not allowed"))
		return
	}
	var req struct {
		ID int64 `json:"id"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		response.WriteJSON(w, response.ErrDefault("invalid request"))
		return
	}
	if req.ID <= 0 {
		response.WriteJSON(w, response.ErrDefault("profile id is required"))
		return
	}
	row, err := h.repo.GetCoverDomainProfile(req.ID)
	if err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}
	if row == nil {
		response.WriteJSON(w, response.ErrDefault("cover profile not found"))
		return
	}
	response.WriteJSON(w, response.OK(row))
}

func (h *Handler) coverProfileUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("method not allowed"))
		return
	}
	var req coverDomainProfileRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		response.WriteJSON(w, response.ErrDefault("invalid request"))
		return
	}

	profile, domains, err := h.normalizeCoverDomainProfileRequest(req)
	if err != nil {
		response.WriteJSON(w, response.ErrDefault(err.Error()))
		return
	}
	if profile.Enabled == 1 {
		if err := h.validateCoverDomainConflicts(profile.ID, domains); err != nil {
			response.WriteJSON(w, response.ErrDefault(err.Error()))
			return
		}
		if _, _, err := readCoverCertMaterial(profile.CertProfile); err != nil {
			response.WriteJSON(w, response.ErrDefault(fmt.Sprintf("certificate %s is not available: %v", profile.CertProfile, err)))
			return
		}
	}

	if err := h.repo.UpsertCoverDomainProfile(profile); err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}
	saved, err := h.repo.GetCoverDomainProfileByName(profile.Name)
	if err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}
	if saved != nil {
		if err := h.syncCoverRuntimeForProfile(saved.ID, true); err != nil {
			response.WriteJSON(w, response.ErrDefault(err.Error()))
			return
		}
	}
	response.WriteJSON(w, response.OK(saved))
}

func (h *Handler) coverProfileDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("method not allowed"))
		return
	}
	var req struct {
		ID int64 `json:"id"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		response.WriteJSON(w, response.ErrDefault("invalid request"))
		return
	}
	if req.ID <= 0 {
		response.WriteJSON(w, response.ErrDefault("profile id is required"))
		return
	}

	bindings, err := h.repo.ListTunnelCoverBindingsByProfileID(req.ID)
	if err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}
	tunnelIDs := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		tunnelIDs = append(tunnelIDs, binding.TunnelID)
	}
	if err := h.repo.DeleteCoverDomainProfile(req.ID); err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}
	if err := h.syncCoverRuntimeForTunnels(tunnelIDs, false); err != nil {
		response.WriteJSON(w, response.ErrDefault(err.Error()))
		return
	}
	response.WriteJSON(w, response.OKEmpty())
}

func (h *Handler) coverTunnelList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("method not allowed"))
		return
	}
	rows, err := h.repo.ListTunnelCoverBindings()
	if err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}
	byTunnel := make(map[int64][]int64)
	for _, row := range rows {
		if row.Enabled != 1 || row.TunnelID <= 0 || row.ProfileID <= 0 {
			continue
		}
		byTunnel[row.TunnelID] = append(byTunnel[row.TunnelID], row.ProfileID)
	}
	out := make([]tunnelCoverSelection, 0, len(byTunnel))
	for tunnelID, profileIDs := range byTunnel {
		out = append(out, tunnelCoverSelection{TunnelID: tunnelID, ProfileIDs: profileIDs})
	}
	response.WriteJSON(w, response.OK(out))
}

func (h *Handler) coverTunnelGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("method not allowed"))
		return
	}
	var req struct {
		TunnelID int64 `json:"tunnelId"`
		ID       int64 `json:"id"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		response.WriteJSON(w, response.ErrDefault("invalid request"))
		return
	}
	tunnelID := req.TunnelID
	if tunnelID <= 0 {
		tunnelID = req.ID
	}
	if tunnelID <= 0 {
		response.WriteJSON(w, response.ErrDefault("tunnel id is required"))
		return
	}
	bindings, err := h.repo.ListTunnelCoverBindingsByTunnelID(tunnelID)
	if err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}
	profileIDs := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		profileIDs = append(profileIDs, binding.ProfileID)
	}
	response.WriteJSON(w, response.OK(tunnelCoverSelection{TunnelID: tunnelID, ProfileIDs: profileIDs}))
}

func (h *Handler) coverTunnelUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("method not allowed"))
		return
	}
	var req tunnelCoverSelection
	if err := decodeJSON(r.Body, &req); err != nil {
		response.WriteJSON(w, response.ErrDefault("invalid request"))
		return
	}
	if req.TunnelID <= 0 {
		response.WriteJSON(w, response.ErrDefault("tunnel id is required"))
		return
	}
	tunnel, err := h.getTunnelRecord(req.TunnelID)
	if err != nil || tunnel == nil {
		response.WriteJSON(w, response.ErrDefault("tunnel not found"))
		return
	}

	profileIDs, err := h.validateCoverProfileIDs(req.ProfileIDs)
	if err != nil {
		response.WriteJSON(w, response.ErrDefault(err.Error()))
		return
	}
	if err := h.repo.ReplaceTunnelCoverBindings(req.TunnelID, profileIDs); err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}
	if err := h.syncCoverRuntimeForTunnels([]int64{req.TunnelID}, len(profileIDs) > 0); err != nil {
		response.WriteJSON(w, response.ErrDefault(err.Error()))
		return
	}
	response.WriteJSON(w, response.OKEmpty())
}

func (h *Handler) coverNodeList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("method not allowed"))
		return
	}
	rows, err := h.repo.ListNodeCoverServices()
	if err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}
	response.WriteJSON(w, response.OK(rows))
}

func (h *Handler) coverNodeGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("method not allowed"))
		return
	}
	var req struct {
		NodeID int64 `json:"nodeId"`
		ID     int64 `json:"id"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		response.WriteJSON(w, response.ErrDefault("invalid request"))
		return
	}
	nodeID := req.NodeID
	if nodeID <= 0 {
		nodeID = req.ID
	}
	if nodeID <= 0 {
		response.WriteJSON(w, response.ErrDefault("node id is required"))
		return
	}
	row, err := h.repo.GetNodeCoverService(nodeID)
	if err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}
	if row == nil {
		row = defaultNodeCoverService(nodeID)
	}
	response.WriteJSON(w, response.OK(row))
}

func (h *Handler) coverNodeUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("method not allowed"))
		return
	}
	var req nodeCoverServiceRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		response.WriteJSON(w, response.ErrDefault("invalid request"))
		return
	}
	if req.NodeID <= 0 {
		response.WriteJSON(w, response.ErrDefault("node id is required"))
		return
	}
	if _, err := h.getNodeRecord(req.NodeID); err != nil {
		response.WriteJSON(w, response.ErrDefault("node not found"))
		return
	}

	oldService, _ := h.repo.GetNodeCoverService(req.NodeID)
	service, err := normalizeNodeCoverServiceRequest(req)
	if err != nil {
		response.WriteJSON(w, response.ErrDefault(err.Error()))
		return
	}
	if oldService != nil {
		service.LastSyncTime = oldService.LastSyncTime
		service.LastStatus = oldService.LastStatus
	}
	if err := h.repo.UpsertNodeCoverService(service); err != nil {
		response.WriteJSON(w, response.Err(-2, err.Error()))
		return
	}

	affected := []int{service.PublicPort}
	if oldService != nil && oldService.PublicPort > 0 && oldService.PublicPort != service.PublicPort {
		affected = append(affected, oldService.PublicPort)
	}
	if err := h.syncCoverSiteToNode(req.NodeID); err != nil {
		response.WriteJSON(w, response.ErrDefault(err.Error()))
		return
	}
	if err := h.syncEntryDemuxToNode(req.NodeID, true); err != nil {
		response.WriteJSON(w, response.ErrDefault(err.Error()))
		return
	}
	if err := h.rebuildSharedSNIServicesOnNode(req.NodeID, affected, 0); err != nil {
		response.WriteJSON(w, response.ErrDefault(err.Error()))
		return
	}
	response.WriteJSON(w, response.OKEmpty())
}

func (h *Handler) coverNodeSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("method not allowed"))
		return
	}
	var req coverNodeSyncRequest
	if err := decodeJSON(r.Body, &req); err != nil {
		response.WriteJSON(w, response.ErrDefault("invalid request"))
		return
	}
	if req.NodeID <= 0 {
		response.WriteJSON(w, response.ErrDefault("node id is required"))
		return
	}
	result, err := h.syncCoverSiteToNodeWithResult(req.NodeID)
	if err != nil {
		response.WriteJSON(w, response.ErrDefault(err.Error()))
		return
	}
	if err := h.syncEntryDemuxToNode(req.NodeID, true); err != nil {
		response.WriteJSON(w, response.ErrDefault(err.Error()))
		return
	}
	if err := h.rebuildSharedSNIServicesOnNode(req.NodeID, nil, 0); err != nil {
		response.WriteJSON(w, response.ErrDefault(err.Error()))
		return
	}
	response.WriteJSON(w, response.OK(result))
}

func (h *Handler) normalizeCoverDomainProfileRequest(req coverDomainProfileRequest) (*model.CoverDomainProfile, []string, error) {
	name := strings.TrimSpace(req.Name)
	certProfile := normalizeCoverProfileName(req.CertProfile)
	domains, err := parseCoverProfileDomains(req.Domains)
	if err != nil {
		return nil, nil, err
	}
	if name == "" {
		name = certProfile
	}
	if name == "" && len(domains) > 0 {
		name = strings.TrimPrefix(domains[0], "*.")
	}
	name = normalizeCoverProfileName(name)
	if name == "" {
		return nil, nil, fmt.Errorf("profile name is required")
	}
	if certProfile == "" {
		certProfile = inferCoverCertProfile(strings.Join(domains, "\n"))
	}
	if certProfile == "" {
		return nil, nil, fmt.Errorf("certificate profile is required")
	}
	upstream := strings.TrimSpace(req.UpstreamOrigin)
	if upstream != "" {
		parsed, err := url.Parse(upstream)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return nil, nil, fmt.Errorf("upstream origin must be a valid URL")
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return nil, nil, fmt.Errorf("upstream origin must use http or https")
		}
		upstream = parsed.String()
	}
	if len(req.StaticHTML) > maxCoverHTMLBytes {
		return nil, nil, fmt.Errorf("static HTML is too large")
	}
	templateProfile := strings.TrimSpace(req.TemplateProfile)
	if templateProfile == "" {
		templateProfile = "static"
	}
	profile := &model.CoverDomainProfile{
		ID:              req.ID,
		Name:            name,
		Enabled:         boolInt(req.Enabled == 1),
		SiteLabel:       strings.TrimSpace(req.SiteLabel),
		Domains:         strings.Join(domains, "\n"),
		CertProfile:     certProfile,
		DNSProvider:     strings.TrimSpace(req.DNSProvider),
		DNSProfile:      strings.TrimSpace(req.DNSProfile),
		TemplateProfile: templateProfile,
		UpstreamOrigin:  upstream,
		StaticHTML:      req.StaticHTML,
	}
	return profile, domains, nil
}

func normalizeNodeCoverServiceRequest(req nodeCoverServiceRequest) (*model.NodeCoverService, error) {
	publicPort := req.PublicPort
	if publicPort <= 0 {
		publicPort = defaultCoverPublicPort
	}
	if publicPort > 65535 {
		return nil, fmt.Errorf("invalid public port")
	}
	localListen := strings.TrimSpace(req.LocalListen)
	if localListen == "" {
		localListen = defaultCoverLocalListen
	}
	if err := validateCoverLocalListen(localListen); err != nil {
		return nil, err
	}
	return &model.NodeCoverService{
		NodeID:      req.NodeID,
		Enabled:     boolInt(req.Enabled == 1),
		PublicPort:  publicPort,
		LocalListen: localListen,
	}, nil
}

func (h *Handler) validateCoverProfileIDs(profileIDs []int64) ([]int64, error) {
	out := make([]int64, 0, len(profileIDs))
	seen := make(map[int64]struct{}, len(profileIDs))
	for _, profileID := range profileIDs {
		if profileID <= 0 {
			continue
		}
		if _, ok := seen[profileID]; ok {
			continue
		}
		profile, err := h.repo.GetCoverDomainProfile(profileID)
		if err != nil {
			return nil, err
		}
		if profile == nil {
			return nil, fmt.Errorf("cover profile %d not found", profileID)
		}
		if profile.Enabled != 1 {
			return nil, fmt.Errorf("cover profile %s is disabled", profile.Name)
		}
		if _, err := parseCoverProfileDomains(profile.Domains); err != nil {
			return nil, fmt.Errorf("cover profile %s has invalid domains: %w", profile.Name, err)
		}
		if _, _, err := readCoverCertMaterial(profile.CertProfile); err != nil {
			return nil, fmt.Errorf("cover profile %s certificate is unavailable: %w", profile.Name, err)
		}
		seen[profileID] = struct{}{}
		out = append(out, profileID)
	}
	return out, nil
}

func (h *Handler) validateCoverDomainConflicts(currentID int64, domains []string) error {
	rows, err := h.repo.ListCoverDomainProfiles()
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row.ID == currentID || row.Enabled != 1 {
			continue
		}
		existing, err := parseCoverProfileDomains(row.Domains)
		if err != nil {
			continue
		}
		for _, domain := range domains {
			for _, other := range existing {
				if coverDomainsOverlap(domain, other) {
					return fmt.Errorf("domain %s conflicts with profile %s (%s)", domain, row.Name, other)
				}
			}
		}
	}
	return nil
}

func coverDomainsOverlap(a, b string) bool {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	if strings.HasPrefix(a, "*.") {
		base := strings.TrimPrefix(a, "*.")
		return b != base && strings.HasSuffix(b, "."+base)
	}
	if strings.HasPrefix(b, "*.") {
		base := strings.TrimPrefix(b, "*.")
		return a != base && strings.HasSuffix(a, "."+base)
	}
	return false
}

func (h *Handler) ensureBuiltInCoverProfiles() error {
	for _, item := range []builtInCoverProfile{
		{
			Name:        "default-entry-cover",
			SiteLabel:   "0n21",
			Domains:     "*.example-entry.test\nexample-entry.test",
			CertProfile: "default-entry-cover",
		},
		{
			Name:        "default-cover-site",
			SiteLabel:   "example-site",
			Domains:     "*.example-cover.test\nexample-cover.test",
			CertProfile: "default-cover-site",
		},
	} {
		if _, _, err := readCoverCertMaterial(item.CertProfile); err != nil {
			continue
		}
		existing, err := h.repo.GetCoverDomainProfileByName(item.Name)
		if err != nil {
			return err
		}
		if existing != nil {
			continue
		}
		now := time.Now().UnixMilli()
		if err := h.repo.UpsertCoverDomainProfile(&model.CoverDomainProfile{
			Name:            item.Name,
			Enabled:         1,
			SiteLabel:       item.SiteLabel,
			Domains:         item.Domains,
			CertProfile:     item.CertProfile,
			TemplateProfile: "static",
			UpstreamOrigin:  "https://ezbid.tw",
			CreatedTime:     now,
			UpdatedTime:     now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) syncCoverRuntimeForProfile(profileID int64, autoEnable bool) error {
	if profileID <= 0 {
		return nil
	}
	bindings, err := h.repo.ListTunnelCoverBindingsByProfileID(profileID)
	if err != nil {
		return err
	}
	tunnelIDs := make([]int64, 0, len(bindings))
	for _, binding := range bindings {
		tunnelIDs = append(tunnelIDs, binding.TunnelID)
	}
	return h.syncCoverRuntimeForTunnels(tunnelIDs, autoEnable)
}

func (h *Handler) syncCoverRuntimeForTunnels(tunnelIDs []int64, autoEnable bool) error {
	nodeIDSet := make(map[int64]struct{})
	for _, tunnelID := range uniqueInt64s(tunnelIDs) {
		if tunnelID <= 0 {
			continue
		}
		nodeIDs, err := h.repo.TunnelEntryNodeIDs(tunnelID)
		if err != nil {
			return err
		}
		for _, nodeID := range nodeIDs {
			if nodeID > 0 {
				nodeIDSet[nodeID] = struct{}{}
			}
		}
	}
	var failures []string
	successes := 0
	for nodeID := range nodeIDSet {
		if autoEnable {
			if err := h.ensureNodeCoverServiceEnabled(nodeID); err != nil {
				failures = append(failures, fmt.Sprintf("node %d enable failed: %v", nodeID, err))
				continue
			}
		}
		service, err := h.repo.GetNodeCoverService(nodeID)
		if err != nil {
			failures = append(failures, fmt.Sprintf("node %d load service failed: %v", nodeID, err))
			continue
		}
		if service == nil {
			continue
		}
		affected := []int{service.PublicPort}
		if _, err := h.syncCoverSiteToNodeWithResult(nodeID); err != nil {
			failures = append(failures, fmt.Sprintf("node %d sync failed: %v", nodeID, err))
			continue
		}
		if err := h.syncEntryDemuxToNode(nodeID, true); err != nil {
			failures = append(failures, fmt.Sprintf("node %d entry demux sync failed: %v", nodeID, err))
			continue
		}
		if err := h.rebuildSharedSNIServicesOnNode(nodeID, affected, 0); err != nil {
			_ = h.repo.UpdateNodeCoverServiceStatus(nodeID, err.Error(), time.Now().UnixMilli())
			failures = append(failures, fmt.Sprintf("node %d shared SNI rebuild failed: %v", nodeID, err))
			continue
		}
		successes++
	}
	if successes == 0 && len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

func (h *Handler) ensureNodeCoverServiceEnabled(nodeID int64) error {
	service, err := h.repo.GetNodeCoverService(nodeID)
	if err != nil {
		return err
	}
	if service == nil {
		service = defaultNodeCoverService(nodeID)
	}
	service.Enabled = 1
	if service.PublicPort <= 0 {
		service.PublicPort = defaultCoverPublicPort
	}
	if strings.TrimSpace(service.LocalListen) == "" || validateCoverLocalListen(service.LocalListen) != nil {
		service.LocalListen = defaultCoverLocalListen
	}
	return h.repo.UpsertNodeCoverService(service)
}

func (h *Handler) syncCoverSiteToNode(nodeID int64) error {
	_, err := h.syncCoverSiteToNodeWithResult(nodeID)
	return err
}

func (h *Handler) syncCoverSiteToNodeWithResult(nodeID int64) (interface{}, error) {
	service, err := h.repo.GetNodeCoverService(nodeID)
	if err != nil {
		return nil, err
	}
	if service == nil {
		service = defaultNodeCoverService(nodeID)
	}
	profiles, err := h.buildCoverSyncProfiles(nodeID, service)
	if err != nil {
		_ = h.repo.UpdateNodeCoverServiceStatus(nodeID, err.Error(), time.Now().UnixMilli())
		return nil, err
	}
	payload := map[string]interface{}{
		"enabled":     service.Enabled == 1,
		"publicPort":  service.PublicPort,
		"localListen": service.LocalListen,
		"profiles":    profiles,
	}
	res, err := h.sendNodeCommandWithTimeout(nodeID, "SyncCoverSite", payload, coverSyncTimeout, false, false)
	status := "OK"
	if err != nil {
		status = err.Error()
		_ = h.repo.UpdateNodeCoverServiceStatus(nodeID, status, time.Now().UnixMilli())
		return res.Data, err
	}
	if res.Message != "" {
		status = res.Message
	}
	_ = h.repo.UpdateNodeCoverServiceStatus(nodeID, status, time.Now().UnixMilli())
	return res.Data, nil
}

func (h *Handler) buildCoverSyncProfiles(nodeID int64, service *model.NodeCoverService) ([]map[string]interface{}, error) {
	if service == nil || service.Enabled != 1 {
		return nil, nil
	}
	tunnelIDs, err := h.repo.ListActiveTunnelIDsByNode(nodeID)
	if err != nil {
		return nil, err
	}
	rows, err := h.repo.ListEnabledCoverDomainProfilesByTunnelIDs(tunnelIDs)
	if err != nil {
		return nil, err
	}
	items := make([]map[string]interface{}, 0, len(rows))
	seenDomains := make(map[string]string)
	for _, row := range rows {
		domains, err := parseCoverProfileDomains(row.Domains)
		if err != nil {
			return nil, err
		}
		for _, domain := range domains {
			for existingDomain, owner := range seenDomains {
				if coverDomainsOverlap(domain, existingDomain) {
					return nil, fmt.Errorf("cover domain %s conflicts with %s from profile %s", domain, existingDomain, owner)
				}
			}
			seenDomains[domain] = row.Name
		}
		certProfile := strings.TrimSpace(row.CertProfile)
		if certProfile == "" {
			certProfile = inferCoverCertProfile(strings.Join(domains, "\n"))
		}
		fullchain, key, err := readCoverCertMaterial(certProfile)
		if err != nil {
			return nil, fmt.Errorf("certificate %s read failed: %w", certProfile, err)
		}
		siteLabel := strings.TrimSpace(row.SiteLabel)
		if siteLabel == "" {
			siteLabel = row.Name
		}
		staticHTML := row.StaticHTML
		if strings.TrimSpace(staticHTML) == "" && strings.TrimSpace(row.UpstreamOrigin) == "" {
			staticHTML = defaultCoverSiteHTML(siteLabel, domains)
		}
		items = append(items, map[string]interface{}{
			"tunnelId":        row.ID,
			"siteLabel":       siteLabel,
			"domains":         domains,
			"certProfile":     certProfile,
			"fullchainPem":    fullchain,
			"privateKeyPem":   key,
			"templateProfile": defaultString(row.TemplateProfile, "static"),
			"upstreamOrigin":  strings.TrimSpace(row.UpstreamOrigin),
			"staticHtml":      staticHTML,
		})
	}
	return items, nil
}

func defaultNodeCoverService(nodeID int64) *model.NodeCoverService {
	return &model.NodeCoverService{
		NodeID:      nodeID,
		Enabled:     0,
		PublicPort:  defaultCoverPublicPort,
		LocalListen: defaultCoverLocalListen,
	}
}

func validateCoverLocalListen(addr string) error {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("local nginx listen must be host:port")
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port <= 0 || port > 65535 {
		return fmt.Errorf("invalid local nginx listen port")
	}
	host = strings.Trim(host, "[]")
	if host != "127.0.0.1" && host != "::1" && strings.ToLower(host) != "localhost" {
		return fmt.Errorf("cover nginx must listen on loopback only")
	}
	return nil
}

func readCoverCertMaterial(profile string) (string, string, error) {
	profile = strings.TrimSpace(profile)
	if profile == "" {
		return "", "", fmt.Errorf("empty cert profile")
	}
	for _, files := range coverCertCandidates(profile) {
		fullchain, certErr := os.ReadFile(files.Fullchain)
		if certErr != nil {
			continue
		}
		key, keyErr := os.ReadFile(files.Key)
		if keyErr != nil {
			continue
		}
		if !strings.Contains(string(fullchain), "BEGIN CERTIFICATE") || !strings.Contains(string(key), "PRIVATE KEY") {
			continue
		}
		return string(fullchain), string(key), nil
	}
	return "", "", fmt.Errorf("cert files not found for profile %s", profile)
}

func coverCertCandidates(profile string) []coverCertFiles {
	profile = normalizeCoverProfileName(profile)
	aliases := map[string]string{
		"0n21":                "example-entry.test",
		"default-entry-cover": "example-entry.test",
		"example-entry":       "example-entry.test",
		"example-entry.test":  "example-entry.test",
		"example-site":        "example-cover.test",
		"default-cover-site":  "example-cover.test",
		"example-cover":       "example-cover.test",
		"example-cover.test":  "example-cover.test",
	}
	certDir := aliases[profile]
	if certDir == "" {
		certDir = profile
	}
	candidates := []coverCertFiles{
		{Fullchain: filepath.Join("/root/certs", certDir, "fullchain.cer"), Key: filepath.Join("/root/certs", certDir, "cert.key")},
		{Fullchain: filepath.Join("/root/certs", certDir, "fullchain.pem"), Key: filepath.Join("/root/certs", certDir, "privkey.pem")},
		{Fullchain: filepath.Join("/root/certs", profile, "fullchain.cer"), Key: filepath.Join("/root/certs", profile, "cert.key")},
		{Fullchain: filepath.Join("/root/certs", profile, "fullchain.pem"), Key: filepath.Join("/root/certs", profile, "privkey.pem")},
	}
	return candidates
}

func normalizeCoverProfileName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			continue
		}
		b.WriteByte('-')
	}
	return strings.Trim(b.String(), ".-")
}

func inferCoverCertProfile(domains string) string {
	lower := strings.ToLower(domains)
	switch {
	case strings.Contains(lower, "example-entry"):
		return "default-entry-cover"
	case strings.Contains(lower, "example-cover"):
		return "default-cover-site"
	default:
		return ""
	}
}

func defaultCoverSiteHTML(siteLabel string, domains []string) string {
	title := strings.TrimSpace(siteLabel)
	if title == "" && len(domains) > 0 {
		title = strings.TrimPrefix(domains[0], "*.")
	}
	if title == "" {
		title = "Service Portal"
	}
	subtitle := "Secure edge service"
	if len(domains) > 0 {
		subtitle = strings.TrimPrefix(domains[0], "*.")
	}
	return fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <style>
    :root{color-scheme:light dark;font-family:Inter,Arial,sans-serif}
    body{margin:0;min-height:100vh;display:grid;place-items:center;background:#f6f7f9;color:#17202a}
    main{width:min(760px,calc(100%% - 32px));padding:40px 0}
    h1{font-size:clamp(32px,5vw,56px);line-height:1;margin:0 0 16px}
    p{font-size:18px;line-height:1.6;color:#526071;margin:0 0 24px}
    .line{width:72px;height:4px;background:#2f6fed;border-radius:2px;margin-bottom:28px}
    @media (prefers-color-scheme:dark){body{background:#111418;color:#f6f7f9}p{color:#aeb7c2}}
  </style>
</head>
<body>
  <main>
    <div class="line"></div>
    <h1>%s</h1>
    <p>%s is online. The service is running normally.</p>
  </main>
</body>
</html>
`, htmlEscape(title), htmlEscape(title), htmlEscape(subtitle))
}

func htmlEscape(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
