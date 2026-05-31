import { useState, useEffect, useRef } from "react";
import { useNavigate } from "react-router-dom";
import { AnimatePresence, motion } from "framer-motion";
import toast from "react-hot-toast";

import { Button } from "@/shadcn-bridge/heroui/button";
import { Card, CardBody, CardHeader } from "@/shadcn-bridge/heroui/card";
import { Input } from "@/shadcn-bridge/heroui/input";
import { Textarea } from "@/shadcn-bridge/heroui/input";
import { Spinner } from "@/shadcn-bridge/heroui/spinner";
import { Divider } from "@/shadcn-bridge/heroui/divider";
import { Switch } from "@/shadcn-bridge/heroui/switch";
import { Select, SelectItem } from "@/shadcn-bridge/heroui/select";
import { Checkbox } from "@/shadcn-bridge/heroui/checkbox";
import {
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
} from "@/shadcn-bridge/heroui/modal";
import {
  updateConfigs,
  exportBackup,
  importBackup,
  getAnnouncement,
  updateAnnouncement,
  type AnnouncementData,
} from "@/api";
import { BackIcon, SettingsIcon } from "@/components/icons";
import { ThemeSettings } from "@/components/theme-settings";
import { isAdmin } from "@/utils/auth";
import { getCachedConfigs, configCache, updateSiteConfig } from "@/config/site";
import {
  type UpdateReleaseChannel,
  getUpdateReleaseChannel,
  setUpdateReleaseChannel,
} from "@/utils/version-update";
import {
  convertBrandAssetToPngDataURL,
  isPngDataURL,
  type BrandAssetKind,
} from "@/utils/brand-asset";

// 蝞??靽??暹?蝏辣
const SaveIcon = ({ className }: { className?: string }) => (
  <svg
    className={className}
    fill="none"
    stroke="currentColor"
    strokeLinecap="round"
    strokeLinejoin="round"
    strokeWidth="2"
    viewBox="0 0 24 24"
  >
    <path d="M19 21H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11l5 5v11a2 2 0 0 1-2 2z" />
    <polyline points="17,21 17,13 7,13 7,21" />
    <polyline points="7,3 7,8 15,8" />
  </svg>
);

interface ConfigItem {
  key: string;
  label: string;
  placeholder?: string;
  description?: string;
  type: "input" | "switch" | "select";
  options?: { label: string; value: string; description?: string }[];
  dependsOn?: string; // 靘???蝵桅★key
  dependsValue?: string; // 靘???蝵桅★??}

const BRAND_PREVIEW_KEYS = ["app_logo", "app_favicon"] as const;

type BrandPreviewKey = (typeof BRAND_PREVIEW_KEYS)[number];

const isBrandPreviewKey = (key: string): key is BrandPreviewKey =>
  BRAND_PREVIEW_KEYS.includes(key as BrandPreviewKey);

const BRAND_FILE_ACCEPT = "image/png,image/jpeg,image/webp,image/svg+xml";

const toBrandAssetKind = (key: BrandPreviewKey): BrandAssetKind => {
  return key === "app_logo" ? "logo" : "favicon";
};

// 蝵??蔭憿孵?銋?const CONFIG_ITEMS: ConfigItem[] = [
  {
    key: "ip",
    label: "?Ｘ?垢?啣?",
    placeholder: "霂瑁??仿?踹?蝡涅P:PORT",
    description:
      '?澆?"ip:port"??domain:port",?其?撖寞??嗡蝙?具??CDN?TTPS,?悖?唳??撖?,
    type: "input",
  },
  {
    key: "agent_download_base_url",
    label: "Agent 銝蝸?箏?",
    placeholder: "靘?: http://203.0.113.10:6366",
    description:
      "?其??摰????Agent 鈭??嗡?頧賜??砍??啣??遣霈桀‵??蝡舫?踹?嚗??冽?剔垢????,
    type: "input",
  },
  {
    key: "panel_domain",
    label: "?Ｘ??",
    placeholder: "霂瑁??仿?踹???,
    description: "敶??Ｘ?????其?銝隞?輯?銵??血鈭急撉?頨思遢",
    type: "input",
  },
  {
    key: "app_name",
    label: "摨?妍",
    placeholder: "霂瑁??亙??典?蝘?,
    description: "?冽?閫?倌憿萄?撖潸?蝷箇?摨?妍",
    type: "input",
  },
  {
    key: "app_logo",
    label: "蝵△閫? Logo",
    description: "?其?憿菟撌虫?閫紡?芾???銝????芸頧祆銝?PNG 撟嗆?銋?靽?",
    type: "input",
  },
  {
    key: "app_favicon",
    label: "瘚??函憬?亙??,
    description: "?其?瘚??冽?蝑暸△?暹?嚗?隡?隡?刻蓮?Ｖ蛹 PNG 撟嗆?銋?靽?",
    type: "input",
  },
  {
    key: "forward_compact_mode",
    label: "閫?憿菟蝎曄?璅∪?",
    description: "撘?臬?嚗??△?Ｗ?銵其蝙??2.1.6-alpha8 ?瑕?嚗撅?蔭嚗?,
    type: "switch",
  },
  {
    key: "monitor_tunnel_quality_enabled",
    label: "摰?折?韐券?璉瘚?,
    description: "?喲???垢?迫?芸?瑟嚗?蝡臬?甇Ｗ??園?捶?瘚??典??蔭嚗?,
    type: "switch",
  },
  {
    key: "captcha_enabled",
    label: "?舐撉???,
    description: "撘?臬?嚗?瑞敶?閬???霂?撉?",
    type: "switch",
  },
  {
    key: "cloudflare_site_key",
    label: "Cloudflare Site Key",
    placeholder: "霂瑁???Cloudflare Site Key",
    description: "Cloudflare Turnstile 蝡撖",
    type: "input",
    dependsOn: "captcha_enabled",
    dependsValue: "true",
  },
  {
    key: "cloudflare_secret_key",
    label: "Cloudflare Secret Key",
    placeholder: "霂瑁???Cloudflare Secret Key",
    description: "Cloudflare Turnstile 撖",
    type: "input",
    dependsOn: "captcha_enabled",
    dependsValue: "true",
  },
  {
    key: "github_proxy_enabled",
    label: "撘??GitHub ??,
    description: "?其???湔??鋆??砌?頧踝?閫??典??啣 GitHub 霈輸???桅?",
    type: "switch",
  },
  {
    key: "github_proxy_url",
    label: "??",
    placeholder: "https://gcode.hostcentral.cc",
    description: "GitHub 銝蝸?誨??嚗??臬?????",
    type: "input",
    dependsOn: "github_proxy_enabled",
    dependsValue: "true",
  },
];

const BACKUP_TYPE_OPTIONS = [
  { value: "users", label: "?冽" },
  { value: "nodes", label: "?" },
  { value: "tunnels", label: "?折?" },
  { value: "forwards", label: "閫?" },
  { value: "userTunnels", label: "?冽?折???" },
  { value: "speedLimits", label: "???? },
  { value: "tunnelGroups", label: "?折???" },
  { value: "userGroups", label: "?冽??" },
  { value: "permissions", label: "????" },
  { value: "configs", label: "蝟餌??蔭" },
] as const;

const BACKUP_TYPE_VALUES = BACKUP_TYPE_OPTIONS.map((option) => option.value);

// ???隞?摮粉??蝵殷??踹??芰?
const getInitialConfigs = (): Record<string, string> => {
  if (typeof window === "undefined") return {};

  const configKeys = [
    "app_name",
    "captcha_enabled",
    "cloudflare_site_key",
    "cloudflare_secret_key",
    "forward_compact_mode",
    "monitor_tunnel_quality_enabled",
    "ip",
    "agent_download_base_url",
    "panel_domain",
    "app_logo",
    "app_favicon",
    "github_proxy_enabled",
    "github_proxy_url",
  ];
  const initialConfigs: Record<string, string> = {};

  try {
    configKeys.forEach((key) => {
      const cachedValue = localStorage.getItem("vite_config_" + key);

      if (cachedValue) {
        initialConfigs[key] = cachedValue;
      }
    });
  } catch {}

  return initialConfigs;
};

export default function ConfigPage() {
  const navigate = useNavigate();
  const initialConfigs = getInitialConfigs();
  const [configs, setConfigs] =
    useState<Record<string, string>>(initialConfigs);
  const [loading, setLoading] = useState(
    Object.keys(initialConfigs).length === 0,
  );
  const [saving, setSaving] = useState(false);
  const [hasChanges, setHasChanges] = useState(false);
  const [originalConfigs, setOriginalConfigs] =
    useState<Record<string, string>>(initialConfigs);

  const [exportTypes, setExportTypes] = useState<string[]>([]);
  const [importTypes, setImportTypes] = useState<string[]>([]);
  const [exporting, setExporting] = useState(false);
  const [importing, setImporting] = useState(false);
  const [exportSelectorOpen, setExportSelectorOpen] = useState(false);
  const [importSelectorOpen, setImportSelectorOpen] = useState(false);
  const [importFileName, setImportFileName] = useState("");
  const backupFileInputRef = useRef<HTMLInputElement>(null);
  const logoFileInputRef = useRef<HTMLInputElement>(null);
  const faviconFileInputRef = useRef<HTMLInputElement>(null);

  const [announcement, setAnnouncement] = useState<AnnouncementData>({
    content: "",
    enabled: 0,
  });
  const [announcementLoading, setAnnouncementLoading] = useState(true);
  const [announcementSaving, setAnnouncementSaving] = useState(false);
  const [updateChannel, setUpdateChannel] = useState<UpdateReleaseChannel>(
    getUpdateReleaseChannel(),
  );
  const [previewLoadFailed, setPreviewLoadFailed] = useState<
    Partial<Record<BrandPreviewKey, boolean>>
  >({});
  const [brandUploading, setBrandUploading] = useState<
    Partial<Record<BrandPreviewKey, boolean>>
  >({});

  const canGoBack =
    typeof window !== "undefined" &&
    typeof window.history.state?.idx === "number" &&
    window.history.state.idx > 0;

  const handleBack = () => {
    if (canGoBack) {
      navigate(-1);

      return;
    }

    navigate("/profile", { replace: true });
  };

  // ??璉??  useEffect(() => {
    if (!isAdmin()) {
      toast.error("??銝雲嚗?恣???臭誑霈輸甇日△??);
      navigate("/dashboard", { replace: true });

      return;
    }
  }, [navigate]);

  // ?蝸?蔭?唳嚗???蝻?嚗?  const loadConfigs = async (currentConfigs?: Record<string, string>) => {
    const configsToCompare = currentConfigs || configs;
    const hasInitialData = Object.keys(configsToCompare).length > 0;

    // 憒?撌脫?蝻??唳嚗??曄內loading嚗?暺??    if (!hasInitialData) {
      setLoading(true);
    }

    try {
      const configData = await getCachedConfigs();

      // ?芣??冽?格????嗆??湔
      const hasDataChanged =
        JSON.stringify(configData) !== JSON.stringify(configsToCompare);

      if (hasDataChanged) {
        setConfigs(configData);
        setOriginalConfigs({ ...configData });
        setHasChanges(false);
      } else {
      }
    } catch {
      // ?芣??冽瓷??摮?格?蝷粹?霂?      if (!hasInitialData) {
        toast.error("?蝸?蔭?粹?嚗窈??");
      }
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    const timer = setTimeout(() => {
      loadConfigs(initialConfigs);
      loadAnnouncement();
    }, 100);

    return () => clearTimeout(timer);
  }, []);

  const loadAnnouncement = async () => {
    setAnnouncementLoading(true);
    try {
      const res = await getAnnouncement();

      if (res.code === 0 && res.data) {
        setAnnouncement(res.data);
      }
    } catch {
    } finally {
      setAnnouncementLoading(false);
    }
  };

  const saveAnnouncement = async () => {
    setAnnouncementSaving(true);
    try {
      const res = await updateAnnouncement(announcement);

      if (res.code === 0) {
        toast.success("?砍?靽???");
      } else {
        toast.error(res.msg || "靽?憭梯揖");
      }
    } catch {
      toast.error("靽??砍?憭梯揖嚗窈??");
    } finally {
      setAnnouncementSaving(false);
    }
  };

  const handleUpdateChannelChange = (channel: UpdateReleaseChannel) => {
    setUpdateChannel(channel);
    setUpdateReleaseChannel(channel);
    toast.success(
      `?湔??撌脣??Ｖ蛹${channel === "stable" ? "蝔喳??? : "撘??"}`,
    );
  };

  const handleConfigChange = (key: string, value: string) => {
    const newConfigs = { ...configs, [key]: value };

    setConfigs(newConfigs);

    if (isBrandPreviewKey(key)) {
      setPreviewLoadFailed((prev) => ({ ...prev, [key]: false }));
    }

    const hasChangesNow =
      Object.keys(newConfigs).some(
        (k) => newConfigs[k] !== originalConfigs[k],
      ) ||
      Object.keys(originalConfigs).some(
        (k) => originalConfigs[k] !== newConfigs[k],
      );

    setHasChanges(hasChangesNow);
  };

  // 靽??蔭
  const handleSave = async () => {
    setSaving(true);
    try {
      const changedKeys = Object.keys(configs).filter(
        (key) => configs[key] !== originalConfigs[key],
      );

      if (changedKeys.length === 0) {
        setHasChanges(false);

        return;
      }

      const changedPayload: Record<string, string> = {};

      changedKeys.forEach((key) => {
        changedPayload[key] = configs[key] || "";
      });

      const response = await updateConfigs(changedPayload);

      if (response.code === 0) {
        toast.success("?蔭靽???");

        Object.entries(configs).forEach(([key, value]) => {
          configCache.set(key, value);
        });

        setOriginalConfigs({ ...configs });
        setHasChanges(false);

        if (
          changedKeys.some((key) =>
            ["app_name", "app_logo", "app_favicon"].includes(key),
          )
        ) {
          await updateSiteConfig(configs);
        }

        // 閫血??蔭?湔鈭辣嚗?嗡?蝏辣
        window.dispatchEvent(
          new CustomEvent("configUpdated", {
            detail: { changedKeys },
          }),
        );

        // 憒??折?韐券?璉瘚??喳??湛?? tunnel-monitor-view
        if (changedKeys.includes("monitor_tunnel_quality_enabled")) {
          window.dispatchEvent(
            new CustomEvent("monitorTunnelQualityEnabledChanged", {
              detail: { enabled: configs["monitor_tunnel_quality_enabled"] === "true" },
            }),
          );
        }
      } else {
        toast.error("靽??蔭憭梯揖: " + response.msg);
      }
    } catch {
      toast.error("靽??蔭?粹?嚗窈??");
    } finally {
      setSaving(false);
    }
  };

  // 璉?仿?蝵桅★?臬摨砲?曄內嚗?韏??伐?
  const shouldShowItem = (item: ConfigItem): boolean => {
    if (!item.dependsOn || !item.dependsValue) {
      return true;
    }

    return configs[item.dependsOn] === item.dependsValue;
  };

  const getBrandInputRef = (key: BrandPreviewKey) => {
    return key === "app_logo" ? logoFileInputRef : faviconFileInputRef;
  };

  const triggerBrandFilePicker = (key: BrandPreviewKey) => {
    if (brandUploading[key]) {
      return;
    }

    getBrandInputRef(key).current?.click();
  };

  const clearBrandAsset = (key: BrandPreviewKey) => {
    handleConfigChange(key, "");
    setPreviewLoadFailed((prev) => ({ ...prev, [key]: false }));
  };

  const handleBrandFileChange = async (
    key: BrandPreviewKey,
    event: React.ChangeEvent<HTMLInputElement>,
  ) => {
    const file = event.target.files?.[0];

    if (!file) {
      return;
    }

    setBrandUploading((prev) => ({ ...prev, [key]: true }));

    try {
      const pngDataURL = await convertBrandAssetToPngDataURL(
        file,
        toBrandAssetKind(key),
      );

      handleConfigChange(key, pngDataURL);
      toast.success(key === "app_logo" ? "Logo 銝???" : "Favicon 銝???");
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "?曄?憭?憭梯揖嚗窈??";

      toast.error(message);
    } finally {
      setBrandUploading((prev) => ({ ...prev, [key]: false }));
      event.target.value = "";
    }
  };

  const renderBrandPreview = (key: BrandPreviewKey) => {
    const previewUrl = (configs[key] || "").trim();
    const appNamePreview = (configs.app_name || "").trim() || "摨?妍";
    const failed = previewLoadFailed[key] === true;
    const showImage = previewUrl.length > 0 && !failed;

    return (
      <div className="mt-3 rounded-lg border border-default-200 dark:border-default-100/30 bg-default-50/60 dark:bg-default-100/10 p-3">
        <p className="text-xs text-default-500">摰憸?</p>
        <div className="mt-2 rounded-md border border-default-200 dark:border-default-100/30 bg-white dark:bg-black px-3 py-2">
          {key === "app_logo" ? (
            <div className="flex h-10 items-center gap-2">
              {showImage ? (
                <img
                  alt="logo preview"
                  className="h-7 w-7 rounded-sm border border-default-200 object-cover dark:border-default-100/30"
                  src={previewUrl}
                  onError={() =>
                    setPreviewLoadFailed((prev) => ({ ...prev, [key]: true }))
                  }
                  onLoad={() =>
                    setPreviewLoadFailed((prev) => ({ ...prev, [key]: false }))
                  }
                />
              ) : (
                <div className="flex h-7 w-7 items-center justify-center rounded-sm bg-default-200 text-[10px] font-semibold text-default-600 dark:bg-default-700 dark:text-default-200">
                  LOGO
                </div>
              )}
              <span className="truncate text-sm font-semibold text-foreground">
                {appNamePreview}
              </span>
            </div>
          ) : (
            <div className="flex h-7 max-w-[260px] items-center gap-2 rounded border border-default-200 bg-default-100/70 px-2 dark:border-default-100/30 dark:bg-default-100/20">
              {showImage ? (
                <img
                  alt="favicon preview"
                  className="h-4 w-4 rounded-sm object-contain"
                  src={previewUrl}
                  onError={() =>
                    setPreviewLoadFailed((prev) => ({ ...prev, [key]: true }))
                  }
                  onLoad={() =>
                    setPreviewLoadFailed((prev) => ({ ...prev, [key]: false }))
                  }
                />
              ) : (
                <div className="h-4 w-4 rounded-sm bg-default-300 dark:bg-default-600" />
              )}
              <span className="truncate text-xs text-default-700 dark:text-default-300">
                {appNamePreview}
              </span>
            </div>
          )}
        </div>

        {previewUrl.length === 0 ? (
          <p className="mt-2 text-xs text-default-500">
            銝??曄???摰?曄內憸?
          </p>
        ) : null}

        {previewUrl.length > 0 && failed ? (
          <p className="mt-2 text-xs text-danger">?曄??蝸憭梯揖嚗窈?銝?</p>
        ) : null}

        {previewUrl.length > 0 && !isPngDataURL(previewUrl) ? (
          <p className="mt-2 text-xs text-warning-600 dark:text-warning-400">
            敶??舀??URL ?蔭嚗遣霈桅??唬?隡?誑?舐???頧?          </p>
        ) : null}
      </div>
    );
  };

  const renderBrandAssetUploader = (
    key: BrandPreviewKey,
    isChanged: boolean,
  ) => {
    const value = (configs[key] || "").trim();
    const uploading = brandUploading[key] === true;
    const isLogo = key === "app_logo";

    return (
      <div
        className={`rounded-lg border p-3 ${
          isChanged
            ? "border-warning-300"
            : "border-default-200 dark:border-default-100/30"
        }`}
      >
        <input
          ref={getBrandInputRef(key)}
          accept={BRAND_FILE_ACCEPT}
          className="hidden"
          type="file"
          onChange={(event) => {
            void handleBrandFileChange(key, event);
          }}
        />

        <div className="flex flex-wrap items-center gap-2">
          <Button
            color="primary"
            isLoading={uploading}
            size="sm"
            variant="flat"
            onPress={() => triggerBrandFilePicker(key)}
          >
            {value.length > 0
              ? isLogo
                ? "?踵 Logo"
                : "?踵 Favicon"
              : isLogo
                ? "銝? Logo"
                : "銝? Favicon"}
          </Button>
          <Button
            isDisabled={value.length === 0 || uploading}
            size="sm"
            variant="light"
            onPress={() => clearBrandAsset(key)}
          >
            皜
          </Button>
          <span className="text-xs text-default-500">
            隞???隞塚??芸頧祆銝?PNG
          </span>
        </div>

        <p className="mt-2 text-xs text-default-500">
          {isLogo
            ? "撱箄悅銝??孵耦?曄?嚗頂蝏?蝏?頧祆銝?96x96 PNG"
            : "撱箄悅銝??孵耦?曄?嚗頂蝏?蝏?頧祆銝?64x64 PNG"}
        </p>

        {renderBrandPreview(key)}
      </div>
    );
  };

  // 皜脫?銝?蝐餃???蝵桅★
  const renderConfigItem = (item: ConfigItem) => {
    const isChanged =
      hasChanges && configs[item.key] !== originalConfigs[item.key];

    switch (item.type) {
      case "input":
        if (isBrandPreviewKey(item.key)) {
          return renderBrandAssetUploader(item.key, isChanged);
        }

        return (
          <Input
            classNames={{
              input: "text-sm",
              inputWrapper: isChanged
                ? "border-warning-300 data-[hover=true]:border-warning-400"
                : "",
            }}
            placeholder={item.placeholder}
            size="md"
            value={configs[item.key] || ""}
            variant="bordered"
            onChange={(e) => handleConfigChange(item.key, e.target.value)}
          />
        );

      case "switch":
        return (
          <Switch
            classNames={{
              wrapper: isChanged ? "border-warning-300" : "",
            }}
            color="primary"
            isSelected={configs[item.key] === "true"}
            size="md"
            onValueChange={(checked) =>
              handleConfigChange(item.key, checked ? "true" : "false")
            }
          >
            <span className="text-sm text-gray-700 dark:text-gray-300">
              {configs[item.key] === "true" ? "撌脣?? : "撌脩???}
            </span>
          </Switch>
        );

      case "select":
        return (
          <Select
            classNames={{
              trigger: isChanged
                ? "border-warning-300 data-[hover=true]:border-warning-400"
                : "",
            }}
            placeholder="霂琿撉??掩??
            selectedKeys={configs[item.key] ? [configs[item.key]] : []}
            size="md"
            variant="bordered"
            onSelectionChange={(keys) => {
              const selectedKey = Array.from(keys)[0] as string;

              if (selectedKey) {
                handleConfigChange(item.key, selectedKey);
              }
            }}
          >
            {item.options?.map((option) => (
              <SelectItem key={option.value} description={option.description}>
                {option.label}
              </SelectItem>
            )) || []}
          </Select>
        );

      default:
        return null;
    }
  };

  const handleExport = async () => {
    if (exportTypes.length === 0) {
      toast.error("霂瑁撠銝蝘?桃掩??);

      return;
    }
    setExporting(true);
    try {
      await exportBackup(exportTypes);
      toast.success("撖澆??");
      setExportSelectorOpen(false);
    } catch {
      toast.error("撖澆憭梯揖嚗窈??");
    } finally {
      setExporting(false);
    }
  };

  const triggerImportFilePicker = () => {
    if (importTypes.length === 0) {
      toast.error("霂瑕??閬紡?亦??唳蝐餃?");

      return;
    }

    setImportSelectorOpen(false);
    requestAnimationFrame(() => backupFileInputRef.current?.click());
  };

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];

    if (!file) return;

    if (importTypes.length === 0) {
      toast.error("霂瑕??閬紡?亦??唳蝐餃?");

      return;
    }

    setImportFileName(file.name);
    setImporting(true);

    try {
      const text = await file.text();
      const data = JSON.parse(text);

      const response = await importBackup({
        types: importTypes,
        ...data,
      });

      if (response.code === 0) {
        toast.success(`撖澆??: ${JSON.stringify(response.data)}`);
        setImportTypes([]);
        setImportFileName("");
      } else {
        toast.error("撖澆憭梯揖: " + response.msg);
      }
    } catch {
      toast.error("撖澆憭梯揖嚗窈璉?交?隞嗆撘?);
    } finally {
      setImporting(false);
      if (backupFileInputRef.current) {
        backupFileInputRef.current.value = "";
      }
    }
  };

  const toggleTypeSelection = (
    type: string,
    setTypes: React.Dispatch<React.SetStateAction<string[]>>,
  ) => {
    setTypes((prev) =>
      prev.includes(type)
        ? prev.filter((item) => item !== type)
        : [...prev, type],
    );
  };

  const isAllTypesSelected = (types: string[]) =>
    BACKUP_TYPE_VALUES.every((type) => types.includes(type));

  const renderTypeSelection = (
    label: string,
    selectedTypes: string[],
    setTypes: React.Dispatch<React.SetStateAction<string[]>>,
  ) => {
    const allSelected = isAllTypesSelected(selectedTypes);

    return (
      <div className="space-y-3">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <span className="text-sm font-medium text-default-700 dark:text-default-300">
            {label}
          </span>
          <div className="flex items-center gap-2">
            <Button
              size="sm"
              variant="flat"
              onPress={() =>
                setTypes(allSelected ? [] : [...BACKUP_TYPE_VALUES])
              }
            >
              {allSelected ? "???券? : "?券?}
            </Button>
            <Button size="sm" variant="light" onPress={() => setTypes([])}>
              皜征
            </Button>
          </div>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
          {BACKUP_TYPE_OPTIONS.map((option) => {
            const isSelected = selectedTypes.includes(option.value);

            return (
              <button
                key={option.value}
                aria-pressed={isSelected}
                className={`w-full px-4 py-3 rounded-lg border transition-all duration-200 cursor-pointer text-left ${
                  isSelected
                    ? "bg-primary-50 dark:bg-primary-900/20 border-primary-300 dark:border-primary-500/50 shadow-sm"
                    : "bg-white dark:bg-default-50 border-default-200 dark:border-default-100/30 hover:border-primary-200 dark:hover:border-primary-500/30 hover:shadow-sm"
                }`}
                type="button"
                onClick={() => toggleTypeSelection(option.value, setTypes)}
              >
                <div className="flex items-center gap-3">
                  <Checkbox
                    classNames={{
                      base: "pointer-events-none",
                    }}
                    color="primary"
                    isSelected={isSelected}
                    size="md"
                  />
                  <span
                    className={`font-medium ${
                      isSelected
                        ? "text-default-900 dark:text-default-100"
                        : "text-default-700 dark:text-default-500"
                    }`}
                  >
                    {option.label}
                  </span>
                </div>
              </button>
            );
          })}
        </div>
      </div>
    );
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[400px]">
        <Spinner label="?蝸?蔭銝?.." size="lg" />
      </div>
    );
  }

  return (
    <div className="p-6 max-w-4xl mx-auto">
      {/* 憿菟?? */}
      <div className="flex items-center gap-3 mb-6">
        <Button
          isIconOnly
          aria-label="餈?銝?憿?
          className="min-w-0 w-9 h-9"
          size="sm"
          variant="flat"
          onPress={handleBack}
        >
          <BackIcon className="w-5 h-5" />
        </Button>
        <SettingsIcon className="w-8 h-8 text-primary" />
        <div>
          <h1 className="text-2xl font-bold">蝵??蔭</h1>
          <p className="text-gray-600 dark:text-gray-400">
            蝞∠?蝵???砌縑?臬??曄內霈曄蔭
          </p>
        </div>
      </div>

      <Card className="shadow-md">
        <CardHeader className="pb-6">
          <div className="flex items-center w-full">
            <div>
              <h2 className="text-xl font-semibold">?箸霈曄蔭</h2>
              <p className="text-sm text-gray-600 dark:text-gray-400">
                ?蔭蝵???砌縑?荔?餈?霈曄蔭隡蔣??蝡??曄內??
              </p>
            </div>
          </div>
        </CardHeader>

        <Divider />

        <CardBody className="space-y-6 pt-8 md:pt-8">
          {CONFIG_ITEMS.map((item, index) => {
            // 璉?仿?蝵桅★?臬摨砲?曄內
            if (!shouldShowItem(item)) {
              return null;
            }

            // 霈∠??臬?舀???銝芣蝷箇?憿寧嚗鈭摰?行蝷箏??瑪嚗?            const remainingItems = CONFIG_ITEMS.slice(index + 1).filter(
              shouldShowItem,
            );
            const isLastItem = remainingItems.length === 0;

            return (
              <div key={item.key} className="space-y-3">
                <div className="flex flex-col gap-1">
                  <label className="text-sm font-medium text-gray-700 dark:text-gray-300">
                    {item.label}
                  </label>
                  {item.description && (
                    <p className="text-xs text-gray-500 dark:text-gray-400">
                      {item.description}
                    </p>
                  )}
                </div>

                {/* 皜脫??蔭憿?*/}
                {renderConfigItem(item)}

                {/* ??蝥?*/}
                {!isLastItem && <Divider className="mt-6" />}
              </div>
            );
          })}

          <Divider className="my-2" />

          <div className="space-y-3">
            <div className="flex flex-col gap-1">
              <p className="text-sm font-medium text-gray-700 dark:text-gray-300">
                ?湔??
              </p>
              <p className="text-xs text-gray-500 dark:text-gray-400">
                蝔喳????寥?蝥舀摮??穿?撘??隞????alpha / beta / rc
                ???研?              </p>
            </div>

            <Select
              selectedKeys={[updateChannel]}
              size="md"
              variant="bordered"
              onSelectionChange={(keys) => {
                const selected =
                  (Array.from(keys)[0] as UpdateReleaseChannel) || "stable";

                handleUpdateChannelChange(selected);
              }}
            >
              <SelectItem key="stable" description="隞滲?啣??嚗? 2.1.4">
                蝔喳???              </SelectItem>
              <SelectItem
                key="dev"
                description="隞?alpha / beta / rc ?喲摮???
              >
                撘??
              </SelectItem>
            </Select>
          </div>

          <div className="flex justify-end pt-6 border-t border-divider/50 mt-4">
            <Button
              color="primary"
              disabled={!hasChanges}
              isLoading={saving}
              startContent={<SaveIcon className="w-4 h-4" />}
              onPress={handleSave}
            >
              {saving ? "靽?銝?.." : "靽??蔭"}
            </Button>
          </div>
        </CardBody>
      </Card>

      {/* 銝駁?霈曄蔭 */}
      <div className="mt-6">
        <ThemeSettings />
      </div>

      {hasChanges && (
        <Card className="mt-4 bg-warning-50 dark:bg-warning-900/20 border-warning-200 dark:border-warning-800 shadow-sm overflow-hidden">
          <div className="h-10 flex items-center justify-center gap-2 text-warning-700 dark:text-warning-300">
            <div className="w-2 h-2 bg-warning-500 rounded-full animate-pulse flex-shrink-0" />
            <span className="text-sm font-medium leading-none">
              璉瘚?蔭?嚗窈霈啣?靽??函?靽格
            </span>
          </div>
        </Card>
      )}

      <Card className="mt-6 shadow-md">
        <CardHeader className="pb-6">
          <div className="flex justify-between items-center w-full">
            <div>
              <h2 className="text-xl font-semibold">?砍?蝞∠?</h2>
              <p className="text-sm text-gray-600 dark:text-gray-400">
                霈曄蔭擐△?曄內???摰?              </p>
            </div>
          </div>
        </CardHeader>

        <Divider />

        <CardBody className="space-y-4 pt-8 md:pt-8">
          {announcementLoading ? (
            <div className="flex justify-center py-8">
              <Spinner size="lg" />
            </div>
          ) : (
            <>
              <div className="space-y-2">
                <Switch
                  isSelected={announcement.enabled === 1}
                  onValueChange={(checked) =>
                    setAnnouncement({
                      ...announcement,
                      enabled: checked ? 1 : 0,
                    })
                  }
                >
                  <span className="text-sm text-gray-700 dark:text-gray-300">
                    {announcement.enabled === 1 ? "撌脣?? : "撌脩???}
                  </span>
                </Switch>
                <p className="text-xs text-gray-500 dark:text-gray-400">
                  ?舐???砍?撠擐△憿園?曄內
                </p>
              </div>

              <Textarea
                label="?砍??捆"
                minRows={4}
                placeholder="?舀? Markdown嚗?憒?**??**??暹](https://example.com)?? ?”"
                value={announcement.content}
                variant="bordered"
                onChange={(e) =>
                  setAnnouncement({ ...announcement, content: e.target.value })
                }
              />
              <p className="text-xs text-gray-500 dark:text-gray-400">
                ?砍??舀? Markdown 霂剜?嚗?乩??冽?倌憿菜?撘
              </p>

              <div className="flex justify-end mt-4 pt-4 border-t border-divider/50">
                <Button
                  color="primary"
                  isLoading={announcementSaving}
                  startContent={<SaveIcon className="w-4 h-4" />}
                  onPress={saveAnnouncement}
                >
                  靽??砍?
                </Button>
              </div>
            </>
          )}
        </CardBody>
      </Card>

      {/* 憭遢銝憭?*/}
      <Card className="mt-6 shadow-md">
        <CardHeader className="pb-6">
          <div className="flex justify-between items-center w-full">
            <div>
              <h2 className="text-xl font-semibold">?唳憭遢銝憭?/h2>
              <p className="text-sm text-gray-600 dark:text-gray-400">
                撖澆?紡?亦頂蝏?殷??舀???孵??唳蝐餃?
              </p>
            </div>
          </div>
        </CardHeader>

        <Divider />

        <CardBody className="space-y-6 pt-8 md:pt-8">
          {/* 撖澆?典? */}
          <div className="space-y-4">
            <h3 className="text-lg font-medium">撖澆?唳</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              ?閬紡?箇??唳蝐餃?嚗紡?箔蛹 JSON ?澆??辣
            </p>
            <p className="text-xs text-default-500">
              敶?撌脤?{exportTypes.length} / {BACKUP_TYPE_VALUES.length}
            </p>

            <div className="flex justify-end gap-3 pt-4">
              <Button
                color="primary"
                isLoading={exporting}
                onPress={() => setExportSelectorOpen(true)}
              >
                {exporting ? "撖澆銝?.." : "?撟嗅紡??}
              </Button>
            </div>
          </div>

          <Divider />

          {/* 撖澆?典? */}
          <div className="space-y-4">
            <h3 className="text-lg font-medium">撖澆?唳</h3>
            <p className="text-sm text-gray-600 dark:text-gray-400">
              ?閬紡?亦??唳蝐餃?嚗??憭遢?辣?Ｗ??唳
            </p>
            <p className="text-xs text-default-500">
              敶?撌脤?{importTypes.length} / {BACKUP_TYPE_VALUES.length}
            </p>

            <input
              ref={backupFileInputRef}
              accept=".json"
              className="hidden"
              type="file"
              onChange={handleFileChange}
            />

            <div className="flex justify-end gap-3 pt-4">
              <Button
                color="primary"
                isLoading={importing}
                variant="flat"
                onPress={() => setImportSelectorOpen(true)}
              >
                {importing ? "撖澆銝?.." : "?撟嗅紡??}
              </Button>
              {importFileName && (
                <span className="self-center text-sm text-gray-600 dark:text-gray-400">
                  撌脤: {importFileName}
                </span>
              )}
            </div>
          </div>
        </CardBody>
      </Card>

      <Modal
        backdrop="blur"
        classNames={{
          base: "!w-[calc(100%-32px)] !mx-auto sm:!w-full rounded-2xl overflow-hidden",
        }}
        isOpen={exportSelectorOpen}
        onOpenChange={setExportSelectorOpen}
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader>?撖澆?捆</ModalHeader>
              <ModalBody>
                {renderTypeSelection("撖澆?捆", exportTypes, setExportTypes)}
              </ModalBody>
              <ModalFooter>
                <Button variant="light" onPress={onClose}>
                  ??
                </Button>
                <Button
                  color="primary"
                  isLoading={exporting}
                  onPress={handleExport}
                >
                  {exporting ? "撖澆銝?.." : "蝖株恕撖澆"}
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>

      <Modal
        backdrop="blur"
        classNames={{
          base: "!w-[calc(100%-32px)] !mx-auto sm:!w-full rounded-2xl overflow-hidden",
        }}
        isOpen={importSelectorOpen}
        onOpenChange={setImportSelectorOpen}
      >
        <ModalContent>
          {(onClose) => (
            <>
              <ModalHeader>?撖澆?捆</ModalHeader>
              <ModalBody>
                {renderTypeSelection("撖澆?捆", importTypes, setImportTypes)}
              </ModalBody>
              <ModalFooter>
                <Button variant="light" onPress={onClose}>
                  ??
                </Button>
                <Button
                  color="primary"
                  isDisabled={importTypes.length === 0}
                  onPress={triggerImportFilePicker}
                >
                  銝?甇仿?辣
                </Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>

      {/* Floating Save Button (FAB) */}
      <AnimatePresence>
        {hasChanges && (
          <motion.div
            initial={{ y: 100, opacity: 0 }}
            animate={{ y: 0, opacity: 1 }}
            exit={{ y: 100, opacity: 0 }}
            transition={{ type: "spring", damping: 20, stiffness: 300 }}
            className="fixed bottom-6 right-6 z-50"
          >
            <Button
              isIconOnly
              color="primary"
              size="lg"
              className="w-12 h-12 rounded-full shadow-lg"
              isLoading={saving}
              onPress={handleSave}
            >
              {!saving && <SaveIcon className="w-5 h-5" />}
            </Button>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
