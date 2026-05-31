import { useEffect, useMemo, useState } from "react";
import { Plus, RefreshCw, Save, ShieldCheck, Trash2 } from "lucide-react";
import { toast } from "react-hot-toast";

import {
  deleteCoverDomainProfile,
  getCoverDomainProfiles,
  getNodeCoverServices,
  getNodeList,
  getTunnelCoverSelections,
  getTunnelList,
  updateCoverDomainProfile,
  updateTunnelCoverSelection,
} from "@/api";
import type {
  CoverDomainProfileApiItem,
  NodeApiItem,
  NodeCoverServiceApiItem,
  TunnelApiItem,
  TunnelChainNodePayload,
  TunnelCoverSelectionApiItem,
} from "@/api/types";
import { Button } from "@/shadcn-bridge/heroui/button";
import { Card, CardBody, CardHeader } from "@/shadcn-bridge/heroui/card";
import { Chip } from "@/shadcn-bridge/heroui/chip";
import { Input, Textarea } from "@/shadcn-bridge/heroui/input";
import { Select, SelectItem } from "@/shadcn-bridge/heroui/select";
import { Spinner } from "@/shadcn-bridge/heroui/spinner";
import { Switch } from "@/shadcn-bridge/heroui/switch";

const emptyProfile = (): CoverDomainProfileApiItem => ({
  name: "",
  enabled: 1,
  siteLabel: "",
  domains: "",
  certProfile: "",
  dnsProvider: "",
  dnsProfile: "",
  templateProfile: "static",
  upstreamOrigin: "https://ezbid.tw",
  staticHtml: "",
});

const formatDomains = (value?: string) =>
  (value || "")
    .split(/\r?\n|,/)
    .map((item) => item.trim())
    .filter(Boolean)
    .join("??) || "-";

const formatTime = (value?: number) => {
  if (!value) return "撠?郊";
  return new Date(value).toLocaleString();
};

const collectNodeIds = (items?: TunnelChainNodePayload[]) =>
  (items || [])
    .map((item) => Number(item.nodeId || 0))
    .filter((id, index, list) => id > 0 && list.indexOf(id) === index);

export default function CoverSitePage() {
  const [loading, setLoading] = useState(true);
  const [savingProfile, setSavingProfile] = useState(false);
  const [savingTunnel, setSavingTunnel] = useState(false);
  const [profiles, setProfiles] = useState<CoverDomainProfileApiItem[]>([]);
  const [tunnels, setTunnels] = useState<TunnelApiItem[]>([]);
  const [nodes, setNodes] = useState<NodeApiItem[]>([]);
  const [selections, setSelections] = useState<TunnelCoverSelectionApiItem[]>([]);
  const [nodeServices, setNodeServices] = useState<NodeCoverServiceApiItem[]>([]);
  const [selectedTunnelId, setSelectedTunnelId] = useState<number>(0);
  const [selectedProfileIds, setSelectedProfileIds] = useState<number[]>([]);
  const [coverEnabled, setCoverEnabled] = useState(false);
  const [selectedProfileId, setSelectedProfileId] = useState<number>(0);
  const [profileForm, setProfileForm] = useState<CoverDomainProfileApiItem>(emptyProfile());

  const profileMap = useMemo(() => new Map(profiles.map((item) => [item.id || 0, item])), [profiles]);
  const nodeMap = useMemo(() => new Map(nodes.map((item) => [item.id, item])), [nodes]);
  const nodeServiceMap = useMemo(() => {
    return new Map(nodeServices.map((item) => [item.nodeId, item]));
  }, [nodeServices]);
  const selectionMap = useMemo(() => {
    return new Map(selections.map((item) => [item.tunnelId, item.profileIds || []]));
  }, [selections]);

  const selectedTunnel = tunnels.find((item) => item.id === selectedTunnelId);
  const selectedEntryNodeIds = useMemo(() => {
    if (!selectedTunnel) return [];
    const explicit = collectNodeIds(selectedTunnel.inNodeId);
    if (explicit.length > 0) return explicit;
    if (selectedTunnel.entryNodeId) return [Number(selectedTunnel.entryNodeId)];
    return [];
  }, [selectedTunnel]);
  const selectedEntryNodes = selectedEntryNodeIds.map((id) => nodeMap.get(id)).filter(Boolean) as NodeApiItem[];
  const selectedProfiles = selectedProfileIds.map((id) => profileMap.get(id)).filter(Boolean) as CoverDomainProfileApiItem[];

  const loadData = async () => {
    setLoading(true);
    try {
      const [profileRes, tunnelRes, selectionRes, nodeRes, serviceRes] = await Promise.all([
        getCoverDomainProfiles(),
        getTunnelList(),
        getTunnelCoverSelections(),
        getNodeList(),
        getNodeCoverServices(),
      ]);
      if (profileRes.code === 0 && profileRes.data) {
        setProfiles(profileRes.data);
        setSelectedProfileId((current) => current || profileRes.data[0]?.id || 0);
      }
      if (tunnelRes.code === 0 && tunnelRes.data) {
        setTunnels(tunnelRes.data);
        setSelectedTunnelId((current) => current || tunnelRes.data[0]?.id || 0);
      }
      if (selectionRes.code === 0 && selectionRes.data) setSelections(selectionRes.data);
      if (nodeRes.code === 0 && nodeRes.data) setNodes(nodeRes.data);
      if (serviceRes.code === 0 && serviceRes.data) setNodeServices(serviceRes.data);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void loadData();
  }, []);

  useEffect(() => {
    if (!selectedProfileId) {
      setProfileForm(emptyProfile());
      return;
    }
    setProfileForm({ ...emptyProfile(), ...(profileMap.get(selectedProfileId) ?? {}) });
  }, [selectedProfileId, profileMap]);

  useEffect(() => {
    if (!selectedTunnelId) {
      setSelectedProfileIds([]);
      setCoverEnabled(false);
      return;
    }
    const boundProfileIds = selectionMap.get(selectedTunnelId) ?? [];
    setSelectedProfileIds(boundProfileIds);
    setCoverEnabled(boundProfileIds.length > 0);
  }, [selectedTunnelId, selectionMap]);

  const refreshProfiles = async () => {
    const fresh = await getCoverDomainProfiles();
    if (fresh.code === 0 && fresh.data) setProfiles(fresh.data);
  };

  const refreshSelections = async () => {
    const fresh = await getTunnelCoverSelections();
    if (fresh.code === 0 && fresh.data) setSelections(fresh.data);
  };

  const refreshNodeServices = async () => {
    const fresh = await getNodeCoverServices();
    if (fresh.code === 0 && fresh.data) setNodeServices(fresh.data);
  };

  const toggleProfileForTunnel = (profileId: number, checked: boolean) => {
    setSelectedProfileIds((current) => {
      if (checked) return current.includes(profileId) ? current : [...current, profileId];
      return current.filter((item) => item !== profileId);
    });
  };

  const saveTunnelSelection = async () => {
    if (!selectedTunnelId) return;
    if (coverEnabled && selectedProfileIds.length === 0) {
      toast.error("?舐撽祉?窈?喳??暸?蝏?銋?);
      return;
    }
    setSavingTunnel(true);
    try {
      const res = await updateTunnelCoverSelection({
        tunnelId: selectedTunnelId,
        profileIds: coverEnabled ? selectedProfileIds : [],
      });
      if (res.code !== 0) {
        toast.error(res.msg || "撽祉?蔭憟憭梯揖");
        return;
      }
      toast.success("撽祉?蔭撌脣??典?亙??);
      await refreshSelections();
      await refreshNodeServices();
    } finally {
      setSavingTunnel(false);
    }
  };

  const saveProfile = async () => {
    setSavingProfile(true);
    try {
      const res = await updateCoverDomainProfile({
        ...profileForm,
        id: selectedProfileId || profileForm.id,
      });
      if (res.code !== 0) {
        toast.error(res.msg || "霂髡獢??靽?憭梯揖");
        return;
      }
      toast.success("霂髡獢??撌脖?摮?);
      await refreshProfiles();
      await refreshSelections();
      await refreshNodeServices();
      if (res.data?.id) setSelectedProfileId(res.data.id);
    } finally {
      setSavingProfile(false);
    }
  };

  const deleteProfile = async () => {
    if (!selectedProfileId) return;
    setSavingProfile(true);
    try {
      const res = await deleteCoverDomainProfile(selectedProfileId);
      if (res.code !== 0) {
        toast.error(res.msg || "霂髡獢???憭梯揖");
        return;
      }
      toast.success("霂髡獢??撌脣???);
      setSelectedProfileId(0);
      setProfileForm(emptyProfile());
      await refreshProfiles();
      await refreshSelections();
      await refreshNodeServices();
    } finally {
      setSavingProfile(false);
    }
  };

  if (loading) {
    return (
      <div className="flex min-h-[50vh] items-center justify-center">
        <Spinner />
      </div>
    );
  }

  return (
    <div className="mx-auto flex w-full max-w-7xl flex-col gap-5 p-4 md:p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-semibold tracking-normal">撽祉蝡?/h1>
          <p className="mt-1 text-sm text-default-500">
            ??折?撟嗅??銋佗??舐?頂蝏??芸憟?啗砲?折????????          </p>
        </div>
        <Button startContent={<RefreshCw className="h-4 w-4" />} variant="bordered" onPress={() => void loadData()}>
          ?瑟
        </Button>
      </div>

      <div className="grid gap-5 xl:grid-cols-[minmax(0,1fr)_380px]">
        <Card className="rounded-md">
          <CardHeader className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 className="text-lg font-semibold">?折?撽祉?蔭</h2>
              <div className="mt-1 text-sm text-default-500">{selectedTunnel?.name || "霂琿?折?"}</div>
            </div>
            <Chip color={coverEnabled ? "success" : "default"} variant="flat">
              {coverEnabled ? "撌脣?? : "?芸??}
            </Chip>
          </CardHeader>
          <CardBody className="space-y-5">
            <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_220px]">
              <Select
                label="?折?"
                selectedKeys={selectedTunnelId ? new Set([String(selectedTunnelId)]) : new Set()}
                onSelectionChange={(keys) => setSelectedTunnelId(Number(Array.from(keys)[0] || 0))}
              >
                {tunnels.map((tunnel) => (
                  <SelectItem key={String(tunnel.id)} textValue={tunnel.name}>
                    {tunnel.name}
                  </SelectItem>
                ))}
              </Select>
              <div className="flex items-end">
                <div className="flex h-10 items-center gap-3">
                  <Switch isSelected={coverEnabled} onValueChange={setCoverEnabled} />
                  <span className="text-sm font-medium text-default-700">?舐撽祉</span>
                </div>
              </div>
            </div>

            <div className="rounded-md border border-divider bg-default-50/70 p-3">
              <div className="text-sm font-medium text-default-800">?亙??/div>
              <div className="mt-2 flex flex-wrap gap-2">
                {selectedEntryNodes.length > 0 ? (
                  selectedEntryNodes.map((node) => {
                    const service = nodeServiceMap.get(node.id);
                    const ok = service?.lastStatus === "OK" || String(service?.lastStatus || "").includes("synced");
                    return (
                      <Chip key={node.id} color={ok ? "success" : "default"} variant="flat">
                        {node.name}
                      </Chip>
                    );
                  })
                ) : (
                  <span className="text-sm text-default-500">餈葵?折??桀?瘝⊥??亙??/span>
                )}
              </div>
            </div>

            <div>
              <div className="mb-2 flex items-center justify-between gap-3">
                <div>
                  <h3 className="text-base font-semibold">?暸?銝芷?雿輻??銋?/h3>
                  <p className="mt-1 text-sm text-default-500">靽?????折?蝏?嚗僎?郊 nginx 銝?443 SNI ????/p>
                </div>
                <Chip variant="flat">{selectedProfiles.length} 蝏?/Chip>
              </div>
              <div className="grid gap-3 md:grid-cols-2">
                {profiles.map((profile) => {
                  const id = profile.id || 0;
                  const checked = selectedProfileIds.includes(id);
                  return (
                    <label
                      key={id}
                      className={`flex min-h-[92px] items-start justify-between gap-3 rounded-md border px-3 py-3 text-sm transition ${
                        checked ? "border-primary bg-primary-50" : "border-divider bg-content1"
                      } ${profile.enabled !== 1 ? "opacity-60" : ""}`}
                    >
                      <span className="min-w-0">
                        <span className="flex items-center gap-2 font-medium">
                          <ShieldCheck className="h-4 w-4 text-primary" />
                          <span className="truncate">{profile.name}</span>
                        </span>
                        <span className="mt-1 block text-xs text-default-500">{formatDomains(profile.domains)}</span>
                        <span className="mt-1 block truncate text-xs text-default-500">
                          霂髡?桀?嚗profile.certProfile || "-"}
                        </span>
                      </span>
                      <input
                        checked={checked}
                        className="mt-1 h-4 w-4 shrink-0"
                        disabled={profile.enabled !== 1}
                        type="checkbox"
                        onChange={(event) => toggleProfileForTunnel(id, event.target.checked)}
                      />
                    </label>
                  );
                })}
              </div>
            </div>

            <div className="flex flex-wrap items-center justify-between gap-3 rounded-md border border-divider p-3">
              <div className="text-sm text-default-600">
                撠??典 {selectedEntryNodeIds.length} ?啣??嚗?                {selectedEntryNodes.map((node) => node.name).join("??) || "-"}
              </div>
              <Button
                color="primary"
                isLoading={savingTunnel}
                startContent={<Save className="h-4 w-4" />}
                onPress={() => void saveTunnelSelection()}
              >
                靽?撟嗅???              </Button>
            </div>
          </CardBody>
        </Card>

        <div className="flex flex-col gap-5">
          <Card className="rounded-md">
            <CardHeader>
              <div>
                <h2 className="text-lg font-semibold">霂髡獢??摨?/h2>
                <div className="mt-1 text-sm text-default-500">{profiles.length} 蝏?冽﹝獢?/div>
              </div>
            </CardHeader>
            <CardBody className="space-y-3">
              {profiles.map((profile) => (
                <button
                  key={profile.id}
                  className={`w-full rounded-md border px-3 py-3 text-left text-sm transition ${
                    selectedProfileId === profile.id
                      ? "border-primary bg-primary-50"
                      : "border-divider bg-content1 hover:bg-default-50"
                  }`}
                  type="button"
                  onClick={() => setSelectedProfileId(profile.id || 0)}
                >
                  <div className="flex items-center justify-between gap-2">
                    <span className="truncate font-medium">{profile.name}</span>
                    <Chip color={profile.enabled === 1 ? "success" : "default"} size="sm" variant="flat">
                      {profile.enabled === 1 ? "?舐" : "?"}
                    </Chip>
                  </div>
                  <div className="mt-1 text-xs text-default-500">{formatDomains(profile.domains)}</div>
                  <div className="mt-1 truncate text-xs text-default-500">?誨?格?嚗profile.upstreamOrigin || "?△"}</div>
                </button>
              ))}
            </CardBody>
          </Card>

          <Card className="rounded-md">
            <CardHeader>
              <div>
                <h2 className="text-lg font-semibold">?亙?箏?甇亦??/h2>
                <div className="mt-1 text-sm text-default-500">靽??折??蔭?蝟餌??芸?湔</div>
              </div>
            </CardHeader>
            <CardBody className="space-y-3">
              {selectedEntryNodeIds.length > 0 ? (
                selectedEntryNodeIds.map((nodeId) => {
                  const node = nodeMap.get(nodeId);
                  const service = nodeServiceMap.get(nodeId);
                  return (
                    <div key={nodeId} className="rounded-md border border-divider p-3 text-sm">
                      <div className="flex items-center justify-between gap-2">
                        <span className="font-medium">{node?.name || `? ${nodeId}`}</span>
                        <Chip color={service?.enabled === 1 ? "success" : "default"} size="sm" variant="flat">
                          {service?.enabled === 1 ? "?撌脣?? : "敺??}
                        </Chip>
                      </div>
                      <div className="mt-2 text-xs text-default-500">?祉?蝡臬嚗service?.publicPort || 443}</div>
                      <div className="mt-1 text-xs text-default-500">
                        ?砍 nginx嚗service?.localListen || "127.0.0.1:10443"}
                      </div>
                      <div className="mt-1 break-words text-xs text-default-500">
                        ?郊嚗formatTime(service?.lastSyncTime)}
                        {service?.lastStatus ? ` / ${service.lastStatus}` : ""}
                      </div>
                    </div>
                  );
                })
              ) : (
                <div className="rounded-md border border-divider p-3 text-sm text-default-500">???亙?箇??/div>
              )}
            </CardBody>
          </Card>
        </div>
      </div>

      <Card className="rounded-md">
        <CardHeader className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="text-lg font-semibold">擃漣嚗?銋行﹝獢輕??/h2>
            <div className="mt-1 text-sm text-default-500">銝?砍?閬銝??折?撟嗅??銋佗?餈??其??啣??耨甇??銋行﹝獢?/div>
          </div>
          <Button
            startContent={<Plus className="h-4 w-4" />}
            variant="bordered"
            onPress={() => {
              setSelectedProfileId(0);
              setProfileForm(emptyProfile());
            }}
          >
            ?啣?獢??
          </Button>
        </CardHeader>
        <CardBody className="space-y-4">
          <div className="flex items-center gap-3">
            <Switch
              isSelected={profileForm.enabled === 1}
              onValueChange={(checked) => setProfileForm((current) => ({ ...current, enabled: checked ? 1 : 0 }))}
            />
            <span className="text-sm text-default-700">獢???舐</span>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <Input
              label="獢???妍"
              value={profileForm.name || ""}
              onChange={(event) => setProfileForm((current) => ({ ...current, name: event.target.value }))}
            />
            <Input
              label="蝡?倌"
              value={profileForm.siteLabel || ""}
              onChange={(event) => setProfileForm((current) => ({ ...current, siteLabel: event.target.value }))}
            />
          </div>

          <Textarea
            label="閬???"
            minRows={3}
            placeholder={"*.example-entry.test\nexample-entry.test"}
            value={profileForm.domains || ""}
            onChange={(event) => setProfileForm((current) => ({ ...current, domains: event.target.value }))}
          />

          <div className="grid gap-4 md:grid-cols-2">
            <Input
              label="霂髡?桀?"
              placeholder="default-entry-cover"
              value={profileForm.certProfile || ""}
              onChange={(event) => setProfileForm((current) => ({ ...current, certProfile: event.target.value }))}
            />
            <Input
              label="?誨?格?"
              placeholder="https://ezbid.tw"
              value={profileForm.upstreamOrigin || ""}
              onChange={(event) => setProfileForm((current) => ({ ...current, upstreamOrigin: event.target.value }))}
            />
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            <Input
              label="DNS ???
              placeholder="cloudns / porkbun"
              value={profileForm.dnsProvider || ""}
              onChange={(event) => setProfileForm((current) => ({ ...current, dnsProvider: event.target.value }))}
            />
            <Input
              label="DNS 獢??"
              value={profileForm.dnsProfile || ""}
              onChange={(event) => setProfileForm((current) => ({ ...current, dnsProfile: event.target.value }))}
            />
          </div>

          <Textarea
            label="??HTML"
            minRows={4}
            value={profileForm.staticHtml || ""}
            onChange={(event) => setProfileForm((current) => ({ ...current, staticHtml: event.target.value }))}
          />

          <div className="flex flex-wrap justify-end gap-2">
            {selectedProfileId ? (
              <Button
                color="danger"
                isLoading={savingProfile}
                startContent={<Trash2 className="h-4 w-4" />}
                variant="bordered"
                onPress={() => void deleteProfile()}
              >
                ?獢??
              </Button>
            ) : null}
            <Button color="primary" isLoading={savingProfile} startContent={<Save className="h-4 w-4" />} onPress={() => void saveProfile()}>
              靽?獢??
            </Button>
          </div>
        </CardBody>
      </Card>
    </div>
  );
}
