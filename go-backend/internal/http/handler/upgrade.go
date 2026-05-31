package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"go-backend/internal/http/response"
)

const (
	githubRepo                    = "passerby7890/flvx-public-safe"
	githubAPIBase                 = "https://api.github.com"
	githubHTMLBase                = "https://github.com"
	upgradeTimeout                = 5 * time.Minute
	batchWorkers                  = 5
	panelPatchedAgentVersion      = "fix-20260530-entry-demux"
	agentDownloadBaseURLConfigKey = "agent_download_base_url"

	releaseChannelStable  = "stable"
	releaseChannelDev     = "dev"
	releaseChannelPatched = "patched"

	defaultGithubProxyEnabled = true
	defaultGithubProxyURL     = "https://gcode.hostcentral.cc"
)

var (
	stableVersionPattern = regexp.MustCompile(`^\d+(?:\.\d+)+$`)
	testKeywordPattern   = regexp.MustCompile(`(?i)(alpha|beta|rc)`)
)

type githubRelease struct {
	TagName     string `json:"tag_name"`
	Name        string `json:"name"`
	PublishedAt string `json:"published_at"`
	Prerelease  bool   `json:"prerelease"`
	Draft       bool   `json:"draft"`
}

func normalizeReleaseChannel(channel string) string {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case releaseChannelDev:
		return releaseChannelDev
	case releaseChannelPatched, "panel":
		return releaseChannelPatched
	default:
		return releaseChannelStable
	}
}

func releaseChannelFromTag(tag string) string {
	normalized := strings.ToLower(strings.TrimSpace(tag))
	if normalized == "" {
		return releaseChannelDev
	}
	if testKeywordPattern.MatchString(normalized) {
		return releaseChannelDev
	}
	if stableVersionPattern.MatchString(normalized) {
		return releaseChannelStable
	}

	return releaseChannelDev
}

func releaseChannelLabel(channel string) string {
	switch normalizeReleaseChannel(channel) {
	case releaseChannelDev:
		return "测试版"
	case releaseChannelPatched:
		return "内建版"
	default:
		return "正式版"
	}
}

func (h *Handler) getGithubProxyConfig() (enabled bool, proxyURL string) {
	enabled = defaultGithubProxyEnabled
	proxyURL = defaultGithubProxyURL

	if h == nil || h.repo == nil {
		return
	}

	if enabledCfg, err := h.repo.GetConfigByName("github_proxy_enabled"); err == nil && enabledCfg != nil {
		enabled = enabledCfg.Value != "false"
	}

	if urlCfg, err := h.repo.GetConfigByName("github_proxy_url"); err == nil && urlCfg != nil && urlCfg.Value != "" {
		proxyURL = strings.TrimSpace(urlCfg.Value)
		if !strings.HasPrefix(proxyURL, "http://") && !strings.HasPrefix(proxyURL, "https://") {
			proxyURL = "https://" + proxyURL
		}
		proxyURL = strings.TrimSuffix(proxyURL, "/")
	}

	return
}

func (h *Handler) buildPanelBaseURL(r *http.Request) string {
	if configured := h.getConfiguredAgentDownloadBaseURL(); configured != "" {
		return configured
	}

	if derived := h.deriveAgentDownloadBaseURLFromPanelConfig(); derived != "" {
		return derived
	}

	if r != nil {
		for _, raw := range []string{r.Header.Get("Origin"), r.Header.Get("Referer")} {
			if parsed := parseBaseURL(raw); parsed != "" {
				return parsed
			}
		}
	}

	scheme := "http"
	if r != nil {
		if forwardedProto := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); forwardedProto != "" {
			scheme = forwardedProto
		} else if r.TLS != nil {
			scheme = "https"
		}

		host := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Host"), ",")[0])
		if host == "" {
			host = strings.TrimSpace(r.Host)
		}
		if host != "" {
			return normalizeBaseURLWithScheme(scheme, host)
		}
	}

	if h != nil && h.repo != nil {
		if hostValue, err := h.repo.GetViteConfigValue("ip"); err == nil {
			host := strings.TrimSpace(hostValue)
			if host != "" {
				return normalizeBaseURLWithScheme(scheme, host)
			}
		}
	}

	return ""
}

func (h *Handler) getConfiguredAgentDownloadBaseURL() string {
	if h == nil || h.repo == nil {
		return ""
	}
	value, err := h.repo.GetViteConfigValue(agentDownloadBaseURLConfigKey)
	if err != nil {
		return ""
	}
	return normalizeConfiguredBaseURL(value)
}

func (h *Handler) deriveAgentDownloadBaseURLFromPanelConfig() string {
	if h == nil || h.repo == nil {
		return ""
	}
	panelAddr, err := h.repo.GetViteConfigValue("ip")
	if err != nil {
		return ""
	}
	return deriveAgentDownloadBaseURLFromNodeAddress(panelAddr)
}

func parseBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return strings.TrimRight(parsed.Scheme+"://"+parsed.Host, "/")
}

func normalizeBaseURLWithScheme(scheme, host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
		return strings.TrimRight(host, "/")
	}
	return strings.TrimRight(fmt.Sprintf("%s://%s", scheme, host), "/")
}

func normalizeConfiguredBaseURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if parsed := parseBaseURL(raw); parsed != "" {
		return parsed
	}
	return normalizeBaseURLWithScheme("http", processServerAddress(raw))
}

func deriveAgentDownloadBaseURLFromNodeAddress(raw string) string {
	host := processServerAddress(raw)
	if host == "" {
		return ""
	}
	trimmedHost := strings.Trim(host, "[]")
	if !strings.Contains(host, ":") || (looksLikeIPv6(trimmedHost) && !strings.HasPrefix(host, "[")) {
		return normalizeBaseURLWithScheme("http", host)
	}

	splitHost, splitPort, err := net.SplitHostPort(host)
	if err != nil {
		return normalizeBaseURLWithScheme("http", host)
	}
	if strings.TrimSpace(splitPort) == "6365" {
		host = net.JoinHostPort(splitHost, "6366")
	}
	return normalizeBaseURLWithScheme("http", host)
}

func (h *Handler) buildPanelAgentAssetURL(r *http.Request, filename string) string {
	baseURL := strings.TrimRight(h.buildPanelBaseURL(r), "/")
	filename = strings.TrimLeft(strings.TrimSpace(filename), "/")
	if baseURL == "" || filename == "" {
		return ""
	}
	return fmt.Sprintf("%s/agent/%s", baseURL, filename)
}

func (h *Handler) buildPanelPatchedAgentURLs(r *http.Request) (downloadURL, checksumURL string, ok bool) {
	downloadURL = h.buildPanelAgentAssetURL(r, "gost-{ARCH}")
	checksumURL = h.buildPanelAgentAssetURL(r, "gost-{ARCH}.sha256")
	ok = downloadURL != "" && checksumURL != ""
	return
}

func (h *Handler) resolvePreferredAgentUpgrade(channel, version string, r *http.Request) (resolvedVersion, downloadURL, checksumURL string, local bool, err error) {
	channel = normalizeReleaseChannel(channel)
	version = strings.TrimSpace(version)

	if channel == releaseChannelPatched {
		if version != "" && version != panelPatchedAgentVersion {
			return "", "", "", false, fmt.Errorf("内建版仅支持当前面板版本 %s", panelPatchedAgentVersion)
		}
		downloadURL, checksumURL, ok := h.buildPanelPatchedAgentURLs(r)
		if !ok {
			return "", "", "", false, errors.New("当前面板未提供内建版节点安装包")
		}
		return panelPatchedAgentVersion, downloadURL, checksumURL, true, nil
	}

	if version != "" {
		return version, h.buildGithubDownloadURL(version, "gost-{ARCH}"), h.buildGithubDownloadURL(version, "gost-{ARCH}.sha256"), false, nil
	}

	resolvedVersion, err = resolveLatestReleaseByChannel(channel)
	if err != nil {
		return "", "", "", false, err
	}
	return resolvedVersion, h.buildGithubDownloadURL(resolvedVersion, "gost-{ARCH}"), h.buildGithubDownloadURL(resolvedVersion, "gost-{ARCH}.sha256"), false, nil
}

func (h *Handler) buildGithubDownloadURL(version, filename string) string {
	enabled, proxyURL := h.getGithubProxyConfig()
	base := fmt.Sprintf("%s/%s/releases/download/%s/%s", githubHTMLBase, githubRepo, version, filename)

	if enabled {
		return fmt.Sprintf("%s/%s", proxyURL, base)
	}
	return base
}

func fetchGitHubReleases(perPage int) ([]githubRelease, error) {
	if perPage <= 0 {
		perPage = 20
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/repos/%s/releases?per_page=%d", githubAPIBase, githubRepo, perPage))
	if err != nil {
		return nil, fmt.Errorf("请求GitHub API失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("GitHub API返回 %d: %s", resp.StatusCode, string(body))
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("解析GitHub API响应失败: %v", err)
	}

	return releases, nil
}

func resolveLatestReleaseByChannel(channel string) (string, error) {
	normalizedChannel := normalizeReleaseChannel(channel)
	releases, err := fetchGitHubReleases(50)
	if err != nil {
		return "", err
	}

	for _, r := range releases {
		if r.Draft {
			continue
		}
		tag := strings.TrimSpace(r.TagName)
		if tag == "" {
			continue
		}
		if releaseChannelFromTag(tag) == normalizedChannel {
			return tag, nil
		}
	}

	return "", fmt.Errorf("未找到%s版本号", releaseChannelLabel(normalizedChannel))
}

func (h *Handler) nodeUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("请求失败"))
		return
	}

	var req struct {
		ID      int64  `json:"id"`
		Version string `json:"version"`
		Channel string `json:"channel"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		response.WriteJSON(w, response.ErrDefault("请求参数错误"))
		return
	}
	if req.ID <= 0 {
		response.WriteJSON(w, response.ErrDefault("节点ID无效"))
		return
	}

	channel := normalizeReleaseChannel(req.Channel)
	version := strings.TrimSpace(req.Version)
	resolvedVersion, downloadURL, checksumURL, _, err := h.resolvePreferredAgentUpgrade(channel, version, r)
	if err != nil {
		response.WriteJSON(w, response.Err(-2, fmt.Sprintf("获取最新%s失败: %v", releaseChannelLabel(channel), err)))
		return
	}

	result, err := h.wsServer.SendCommand(req.ID, "UpgradeAgent", map[string]interface{}{
		"downloadUrl": downloadURL,
		"checksumUrl": checksumURL,
	}, upgradeTimeout)
	if err != nil {
		response.WriteJSON(w, response.Err(-2, fmt.Sprintf("升级失败: %v", err)))
		return
	}
	h.markNodePendingUpgradeRedeploy(req.ID)

	response.WriteJSON(w, response.OK(map[string]interface{}{
		"version": resolvedVersion,
		"message": result.Message,
	}))
}

func resolveLatestRelease() (string, error) {
	return resolveLatestReleaseByChannel(releaseChannelStable)
}

func resolveLatestReleaseAPI() (string, error) {
	return resolveLatestReleaseByChannel(releaseChannelStable)
}

func (h *Handler) nodeBatchUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("请求失败"))
		return
	}

	var req struct {
		IDs     []int64 `json:"ids"`
		Version string  `json:"version"`
		Channel string  `json:"channel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.WriteJSON(w, response.ErrDefault("请求参数错误"))
		return
	}
	if len(req.IDs) == 0 {
		response.WriteJSON(w, response.ErrDefault("ids不能为空"))
		return
	}

	channel := normalizeReleaseChannel(req.Channel)
	version := strings.TrimSpace(req.Version)
	resolvedVersion, downloadURL, checksumURL, _, err := h.resolvePreferredAgentUpgrade(channel, version, r)
	if err != nil {
		response.WriteJSON(w, response.Err(-2, fmt.Sprintf("获取最新%s失败: %v", releaseChannelLabel(channel), err)))
		return
	}

	type upgradeResult struct {
		ID      int64  `json:"id"`
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	results := make([]upgradeResult, len(req.IDs))
	sem := make(chan struct{}, batchWorkers)
	var wg sync.WaitGroup

	for i, id := range req.IDs {
		wg.Add(1)
		go func(index int, nodeID int64) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := h.wsServer.SendCommand(nodeID, "UpgradeAgent", map[string]interface{}{
				"downloadUrl": downloadURL,
				"checksumUrl": checksumURL,
			}, upgradeTimeout)
			if err != nil {
				results[index] = upgradeResult{ID: nodeID, Success: false, Message: err.Error()}
				return
			}
			h.markNodePendingUpgradeRedeploy(nodeID)
			results[index] = upgradeResult{ID: nodeID, Success: true, Message: result.Message}
		}(i, id)
	}
	wg.Wait()

	response.WriteJSON(w, response.OK(map[string]interface{}{
		"version": resolvedVersion,
		"results": results,
	}))
}

func (h *Handler) nodeCustomUpgrade(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("请求失败"))
		return
	}

	var req struct {
		IDs         []int64 `json:"ids"`
		DownloadURL string  `json:"downloadUrl"`
		ChecksumURL string  `json:"checksumUrl"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		response.WriteJSON(w, response.ErrDefault("请求参数错误"))
		return
	}
	if len(req.IDs) == 0 {
		response.WriteJSON(w, response.ErrDefault("ids不能为空"))
		return
	}
	req.DownloadURL = strings.TrimSpace(req.DownloadURL)
	req.ChecksumURL = strings.TrimSpace(req.ChecksumURL)
	if req.DownloadURL == "" {
		response.WriteJSON(w, response.ErrDefault("下载地址不能为空"))
		return
	}

	type upgradeResult struct {
		ID      int64  `json:"id"`
		Success bool   `json:"success"`
		Message string `json:"message"`
	}

	results := make([]upgradeResult, len(req.IDs))
	sem := make(chan struct{}, batchWorkers)
	var wg sync.WaitGroup

	for i, id := range req.IDs {
		wg.Add(1)
		go func(index int, nodeID int64) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			result, err := h.wsServer.SendCommand(nodeID, "UpgradeAgent", map[string]interface{}{
				"downloadUrl": req.DownloadURL,
				"checksumUrl": req.ChecksumURL,
			}, upgradeTimeout)
			if err != nil {
				results[index] = upgradeResult{ID: nodeID, Success: false, Message: err.Error()}
				return
			}
			h.markNodePendingUpgradeRedeploy(nodeID)
			results[index] = upgradeResult{ID: nodeID, Success: true, Message: result.Message}
		}(i, id)
	}
	wg.Wait()

	response.WriteJSON(w, response.OK(map[string]interface{}{
		"downloadUrl": req.DownloadURL,
		"checksumUrl": req.ChecksumURL,
		"results":     results,
	}))
}

func (h *Handler) nodeCustomCommand(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("请求失败"))
		return
	}

	var req struct {
		ID          int64       `json:"id"`
		CommandType string      `json:"commandType"`
		Data        interface{} `json:"data"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		response.WriteJSON(w, response.ErrDefault("请求参数错误"))
		return
	}
	if req.ID <= 0 {
		response.WriteJSON(w, response.ErrDefault("节点ID无效"))
		return
	}
	req.CommandType = strings.TrimSpace(req.CommandType)
	if req.CommandType == "" {
		response.WriteJSON(w, response.ErrDefault("commandType不能为空"))
		return
	}

	result, err := h.wsServer.SendCommand(req.ID, req.CommandType, req.Data, 30*time.Second)
	if err != nil {
		response.WriteJSON(w, response.Err(-2, fmt.Sprintf("执行失败: %v", err)))
		return
	}
	response.WriteJSON(w, response.OK(result))
}

func (h *Handler) inspectNodePort(nodeID int64, port int) (map[string]interface{}, error) {
	if h == nil || nodeID <= 0 || port <= 0 {
		return nil, errors.New("invalid node port inspection context")
	}
	result, err := h.wsServer.SendCommand(nodeID, "CheckPort", map[string]interface{}{
		"port": port,
	}, 15*time.Second)
	if err != nil {
		return nil, err
	}
	if result.Data == nil {
		return nil, errors.New("empty port inspection result")
	}
	return result.Data, nil
}

func (h *Handler) listReleases(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("请求失败"))
		return
	}

	var req struct {
		Channel string `json:"channel"`
	}
	if err := decodeJSON(r.Body, &req); err != nil && err != io.EOF {
		response.WriteJSON(w, response.ErrDefault("请求参数错误"))
		return
	}

	channel := normalizeReleaseChannel(req.Channel)

	if channel == releaseChannelPatched {
		type releaseItem struct {
			Version     string `json:"version"`
			Name        string `json:"name"`
			PublishedAt string `json:"publishedAt"`
			Prerelease  bool   `json:"prerelease"`
			Channel     string `json:"channel"`
		}

		_, _, ok := h.buildPanelPatchedAgentURLs(r)
		if !ok {
			response.WriteJSON(w, response.OK([]releaseItem{}))
			return
		}

		response.WriteJSON(w, response.OK([]releaseItem{
			{
				Version:     panelPatchedAgentVersion,
				Name:        "当前面板内建版",
				PublishedAt: "",
				Prerelease:  true,
				Channel:     releaseChannelPatched,
			},
		}))
		return
	}

	releases, err := fetchGitHubReleases(50)
	if err != nil {
		response.WriteJSON(w, response.Err(-2, fmt.Sprintf("获取版本列表失败: %v", err)))
		return
	}

	type releaseItem struct {
		Version     string `json:"version"`
		Name        string `json:"name"`
		PublishedAt string `json:"publishedAt"`
		Prerelease  bool   `json:"prerelease"`
		Channel     string `json:"channel"`
	}

	items := make([]releaseItem, 0, len(releases))
	for _, r := range releases {
		if r.Draft {
			continue
		}
		tag := strings.TrimSpace(r.TagName)
		if tag == "" {
			continue
		}
		itemChannel := releaseChannelFromTag(tag)
		if itemChannel != channel {
			continue
		}
		items = append(items, releaseItem{
			Version:     tag,
			Name:        r.Name,
			PublishedAt: r.PublishedAt,
			Prerelease:  itemChannel == releaseChannelDev,
			Channel:     itemChannel,
		})
	}

	response.WriteJSON(w, response.OK(items))
}

func (h *Handler) nodeRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.WriteJSON(w, response.ErrDefault("请求失败"))
		return
	}

	var req struct {
		ID int64 `json:"id"`
	}
	if err := decodeJSON(r.Body, &req); err != nil {
		response.WriteJSON(w, response.ErrDefault("请求参数错误"))
		return
	}
	if req.ID <= 0 {
		response.WriteJSON(w, response.ErrDefault("节点ID无效"))
		return
	}

	result, err := h.wsServer.SendCommand(req.ID, "RollbackAgent", map[string]interface{}{}, 30*time.Second)
	if err != nil {
		response.WriteJSON(w, response.Err(-2, fmt.Sprintf("回退失败: %v", err)))
		return
	}

	response.WriteJSON(w, response.OK(map[string]interface{}{
		"message": result.Message,
	}))
}

func (h *Handler) markNodePendingUpgradeRedeploy(nodeID int64) {
	if h == nil || nodeID <= 0 {
		return
	}
	h.upgradeMu.Lock()
	h.pendingUpgradeRedeploy[nodeID] = struct{}{}
	h.upgradeMu.Unlock()
}

func (h *Handler) consumeNodePendingUpgradeRedeploy(nodeID int64) bool {
	if h == nil || nodeID <= 0 {
		return false
	}
	h.upgradeMu.Lock()
	_, ok := h.pendingUpgradeRedeploy[nodeID]
	if ok {
		delete(h.pendingUpgradeRedeploy, nodeID)
	}
	h.upgradeMu.Unlock()
	return ok
}

func (h *Handler) onNodeOnline(nodeID int64) {
	go func() {
		if err := h.syncNodeNetworkInfo(nodeID); err != nil {
			fmt.Printf("node network sync on online: node %d failed: %v\n", nodeID, err)
		}
	}()

	if h.consumeNodePendingUpgradeRedeploy(nodeID) {
		h.redeployNodeRuntimeAfterUpgrade(nodeID)
	}
}

func (h *Handler) redeployNodeRuntimeAfterUpgrade(nodeID int64) {
	tunnelIDs, err := h.repo.ListActiveTunnelIDsByNode(nodeID)
	if err != nil {
		fmt.Printf("post-upgrade redeploy: list tunnels for node %d failed: %v\n", nodeID, err)
		return
	}
	forwardIDs, err := h.repo.ListActiveForwardIDsByNode(nodeID)
	if err != nil {
		fmt.Printf("post-upgrade redeploy: list forwards for node %d failed: %v\n", nodeID, err)
		return
	}

	tunnelFailed := make(map[int64]struct{})
	for _, tunnelID := range tunnelIDs {
		if err := h.redeployTunnelAndForwards(tunnelID); err != nil {
			tunnelFailed[tunnelID] = struct{}{}
			fmt.Printf("post-upgrade redeploy: tunnel %d failed on node %d: %v\n", tunnelID, nodeID, err)
		}
	}

	for _, forwardID := range forwardIDs {
		forward, getErr := h.getForwardRecord(forwardID)
		if getErr != nil || forward == nil {
			continue
		}
		if _, skipped := tunnelFailed[forward.TunnelID]; skipped {
			continue
		}
		if err := h.syncForwardServices(forward, "UpdateService", true); err != nil {
			fmt.Printf("post-upgrade redeploy: forward %d failed on node %d: %v\n", forwardID, nodeID, err)
		}
	}
}
