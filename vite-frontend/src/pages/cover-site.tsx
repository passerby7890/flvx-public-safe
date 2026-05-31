import { useEffect, useMemo, useState } from "react";
import { Plus, RefreshCw, Save, Trash2 } from "lucide-react";
import { toast } from "react-hot-toast";

import {
  deleteCoverDomainProfile,
  getCoverDomainProfiles,
  getTunnelCoverSelections,
  getTunnelList,
  updateCoverDomainProfile,
  updateTunnelCoverSelection,
} from "@/api";
import type {
  CoverDomainProfileApiItem,
  TunnelApiItem,
  TunnelCoverSelectionApiItem,
} from "@/api/types";
import { Button } from "@/shadcn-bridge/heroui/button";
import { Card, CardBody, CardHeader } from "@/shadcn-bridge/heroui/card";
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
  upstreamOrigin: "https://example.com",
  staticHtml: "",
});

const formatDomains = (value?: string) =>
  (value || "")
    .split(/\r?\n|,/)
    .map((item) => item.trim())
    .filter(Boolean)
    .join(", ") || "-";

const formatTime = (value?: number) => {
  if (!value) return "-";
  return new Date(value).toLocaleString();
};

export default function CoverSitePage() {
  const [loading, setLoading] = useState(true);
  const [savingProfile, setSavingProfile] = useState(false);
  const [savingTunnel, setSavingTunnel] = useState(false);
  const [profiles, setProfiles] = useState<CoverDomainProfileApiItem[]>([]);
  const [tunnels, setTunnels] = useState<TunnelApiItem[]>([]);
  const [selections, setSelections] = useState<TunnelCoverSelectionApiItem[]>(
    [],
  );
  const [selectedTunnelId, setSelectedTunnelId] = useState<number>(0);
  const [selectedProfileIds, setSelectedProfileIds] = useState<number[]>([]);
  const [coverEnabled, setCoverEnabled] = useState(false);
  const [selectedProfileId, setSelectedProfileId] = useState<number>(0);
  const [profileForm, setProfileForm] =
    useState<CoverDomainProfileApiItem>(emptyProfile());

  const profileMap = useMemo(
    () => new Map(profiles.map((item) => [item.id || 0, item])),
    [profiles],
  );
  const selectionMap = useMemo(
    () => new Map(selections.map((item) => [item.tunnelId, item.profileIds])),
    [selections],
  );

  const selectedTunnel = tunnels.find((item) => item.id === selectedTunnelId);
  const selectedProfiles = selectedProfileIds
    .map((id) => profileMap.get(id))
    .filter(Boolean) as CoverDomainProfileApiItem[];

  const loadData = async () => {
    setLoading(true);
    try {
      const [profileRes, tunnelRes, selectionRes] = await Promise.all([
        getCoverDomainProfiles(),
        getTunnelList(),
        getTunnelCoverSelections(),
      ]);

      if (profileRes.code === 0 && profileRes.data) {
        setProfiles(profileRes.data);
        setSelectedProfileId((current) => current || profileRes.data[0]?.id || 0);
      }
      if (tunnelRes.code === 0 && tunnelRes.data) {
        setTunnels(tunnelRes.data);
        setSelectedTunnelId((current) => current || tunnelRes.data[0]?.id || 0);
      }
      if (selectionRes.code === 0 && selectionRes.data) {
        setSelections(selectionRes.data);
      }
    } catch {
      toast.error("加载伪装站点配置失败");
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

    setProfileForm({
      ...emptyProfile(),
      ...(profileMap.get(selectedProfileId) ?? {}),
    });
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

  const saveTunnelSelection = async () => {
    if (!selectedTunnelId) return;
    if (coverEnabled && selectedProfileIds.length === 0) {
      toast.error("启用伪装站点时至少选择一个配置");
      return;
    }

    setSavingTunnel(true);
    try {
      const res = await updateTunnelCoverSelection({
        tunnelId: selectedTunnelId,
        profileIds: coverEnabled ? selectedProfileIds : [],
      });

      if (res.code !== 0) {
        toast.error(res.msg || "保存隧道绑定失败");
        return;
      }

      toast.success("隧道绑定已保存");
      await refreshSelections();
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
        toast.error(res.msg || "保存配置失败");
        return;
      }

      toast.success("配置已保存");
      await refreshProfiles();
      await refreshSelections();
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
        toast.error(res.msg || "删除配置失败");
        return;
      }

      toast.success("配置已删除");
      setSelectedProfileId(0);
      setProfileForm(emptyProfile());
      await refreshProfiles();
      await refreshSelections();
    } finally {
      setSavingProfile(false);
    }
  };

  const toggleProfileForTunnel = (profileId: number, checked: boolean) => {
    setSelectedProfileIds((current) => {
      if (checked) {
        return current.includes(profileId) ? current : [...current, profileId];
      }

      return current.filter((item) => item !== profileId);
    });
  };

  if (loading) {
    return (
      <div className="flex min-h-[400px] items-center justify-center">
        <Spinner label="加载中..." size="lg" />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-6xl space-y-5 p-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">伪装站点</h1>
          <p className="text-sm text-default-500">
            管理域名配置，并把伪装站点绑定到隧道入口。
          </p>
        </div>
        <Button startContent={<RefreshCw className="h-4 w-4" />} onPress={loadData}>
          刷新
        </Button>
      </div>

      <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_420px]">
        <Card className="rounded-md">
          <CardHeader>
            <div>
              <h2 className="text-lg font-semibold">隧道绑定</h2>
              <p className="mt-1 text-sm text-default-500">
                选择隧道后勾选要应用的伪装站点配置。
              </p>
            </div>
          </CardHeader>
          <CardBody className="space-y-4">
            <Select
              label="隧道"
              placeholder="请选择隧道"
              selectedKeys={selectedTunnelId ? [String(selectedTunnelId)] : []}
              onSelectionChange={(keys) => {
                const selected = Array.from(keys)[0];

                setSelectedTunnelId(Number(selected || 0));
              }}
            >
              {tunnels.map((tunnel) => (
                <SelectItem key={String(tunnel.id)}>
                  {tunnel.name || `Tunnel #${tunnel.id}`}
                </SelectItem>
              ))}
            </Select>

            <Switch isSelected={coverEnabled} onValueChange={setCoverEnabled}>
              启用伪装站点
            </Switch>

            <div className="space-y-2">
              {profiles.length === 0 ? (
                <p className="text-sm text-default-500">暂无伪装站点配置</p>
              ) : (
                profiles.map((profile) => {
                  const id = profile.id || 0;

                  return (
                    <label
                      key={id}
                      className="flex items-start justify-between gap-3 rounded-md border border-divider p-3"
                    >
                      <span>
                        <span className="block text-sm font-medium">
                          {profile.name || `Profile #${id}`}
                        </span>
                        <span className="block text-xs text-default-500">
                          {formatDomains(profile.domains)}
                        </span>
                      </span>
                      <input
                        checked={selectedProfileIds.includes(id)}
                        className="mt-1 h-4 w-4 shrink-0"
                        disabled={!coverEnabled || profile.enabled !== 1}
                        type="checkbox"
                        onChange={(event) =>
                          toggleProfileForTunnel(id, event.target.checked)
                        }
                      />
                    </label>
                  );
                })
              )}
            </div>

            <div className="rounded-md border border-divider p-3 text-sm text-default-600">
              当前隧道: {selectedTunnel?.name || "-"}
              <br />
              已选择配置:{" "}
              {selectedProfiles.map((profile) => profile.name).join(", ") || "-"}
            </div>

            <div className="flex justify-end">
              <Button
                color="primary"
                isLoading={savingTunnel}
                startContent={<Save className="h-4 w-4" />}
                onPress={() => void saveTunnelSelection()}
              >
                保存绑定
              </Button>
            </div>
          </CardBody>
        </Card>

        <Card className="rounded-md">
          <CardHeader>
            <div className="flex w-full items-center justify-between gap-3">
              <div>
                <h2 className="text-lg font-semibold">配置详情</h2>
                <p className="mt-1 text-sm text-default-500">
                  已创建 {profiles.length} 个配置
                </p>
              </div>
              <Button
                isIconOnly
                aria-label="新建配置"
                onPress={() => {
                  setSelectedProfileId(0);
                  setProfileForm(emptyProfile());
                }}
              >
                <Plus className="h-4 w-4" />
              </Button>
            </div>
          </CardHeader>
          <CardBody className="space-y-4">
            <Select
              label="选择配置"
              placeholder="新建或选择配置"
              selectedKeys={selectedProfileId ? [String(selectedProfileId)] : []}
              onSelectionChange={(keys) => {
                const selected = Array.from(keys)[0];

                setSelectedProfileId(Number(selected || 0));
              }}
            >
              {profiles.map((profile) => (
                <SelectItem key={String(profile.id || 0)}>
                  {profile.name || `Profile #${profile.id}`}
                </SelectItem>
              ))}
            </Select>

            <Input
              label="配置名称"
              placeholder="default-entry-cover"
              value={profileForm.name || ""}
              onChange={(event) =>
                setProfileForm((current) => ({
                  ...current,
                  name: event.target.value,
                }))
              }
            />

            <Switch
              isSelected={profileForm.enabled === 1}
              onValueChange={(checked) =>
                setProfileForm((current) => ({
                  ...current,
                  enabled: checked ? 1 : 0,
                }))
              }
            >
              启用配置
            </Switch>

            <Textarea
              label="域名"
              placeholder={"example.com\nwww.example.com"}
              value={profileForm.domains || ""}
              onChange={(event) =>
                setProfileForm((current) => ({
                  ...current,
                  domains: event.target.value,
                }))
              }
            />

            <Input
              label="证书配置"
              placeholder="default-entry-cover"
              value={profileForm.certProfile || ""}
              onChange={(event) =>
                setProfileForm((current) => ({
                  ...current,
                  certProfile: event.target.value,
                }))
              }
            />

            <Input
              label="上游站点"
              placeholder="https://example.com"
              value={profileForm.upstreamOrigin || ""}
              onChange={(event) =>
                setProfileForm((current) => ({
                  ...current,
                  upstreamOrigin: event.target.value,
                }))
              }
            />

            <div className="grid gap-4 md:grid-cols-2">
              <Input
                label="DNS 提供商"
                placeholder="cloudns / porkbun"
                value={profileForm.dnsProvider || ""}
                onChange={(event) =>
                  setProfileForm((current) => ({
                    ...current,
                    dnsProvider: event.target.value,
                  }))
                }
              />
              <Input
                label="DNS 配置"
                value={profileForm.dnsProfile || ""}
                onChange={(event) =>
                  setProfileForm((current) => ({
                    ...current,
                    dnsProfile: event.target.value,
                  }))
                }
              />
            </div>

            <Textarea
              label="静态 HTML"
              minRows={5}
              value={profileForm.staticHtml || ""}
              onChange={(event) =>
                setProfileForm((current) => ({
                  ...current,
                  staticHtml: event.target.value,
                }))
              }
            />

            <p className="text-xs text-default-500">
              最后更新: {formatTime(profileForm.updatedTime)}
            </p>

            <div className="flex justify-end gap-2">
              {selectedProfileId > 0 && (
                <Button
                  color="danger"
                  isLoading={savingProfile}
                  startContent={<Trash2 className="h-4 w-4" />}
                  variant="flat"
                  onPress={() => void deleteProfile()}
                >
                  删除
                </Button>
              )}
              <Button
                color="primary"
                isLoading={savingProfile}
                startContent={<Save className="h-4 w-4" />}
                onPress={() => void saveProfile()}
              >
                保存配置
              </Button>
            </div>
          </CardBody>
        </Card>
      </div>
    </div>
  );
}
