import { getConfigByName, getConfigs } from "@/api";

export type SiteConfig = typeof siteConfig;

const CACHE_PREFIX = "vite_config_";
const VERSION = import.meta.env.VITE_APP_VERSION || "dev";
const APP_VERSION = "1.0.3";
const DEFAULT_FAVICON = "/favicon.ico";
const FAVICON_LINK_ID = "app-favicon";
const GITHUB_REPO =
  import.meta.env.VITE_GITHUB_REPO ||
  "https://github.com/passerby7890/flvx-public-safe";

const getInitialConfig = () => {
  if (typeof window === "undefined") {
    return {
      name: "FLVX",
      version: VERSION,
      app_version: APP_VERSION,
      github_repo: GITHUB_REPO,
      app_logo: "",
      app_favicon: "",
    };
  }

  const cachedAppName = localStorage.getItem(CACHE_PREFIX + "app_name");
  const cachedAppLogo = localStorage.getItem(CACHE_PREFIX + "app_logo") || "";
  const cachedAppFavicon =
    localStorage.getItem(CACHE_PREFIX + "app_favicon") || "";

  return {
    name: cachedAppName || "FLVX",
    version: VERSION,
    app_version: APP_VERSION,
    github_repo: GITHUB_REPO,
    app_logo: cachedAppLogo,
    app_favicon: cachedAppFavicon,
  };
};

export const siteConfig = getInitialConfig();

export const configCache = {
  get: (key: string): string | null => {
    return localStorage.getItem(CACHE_PREFIX + key);
  },

  set: (key: string, value: string): void => {
    localStorage.setItem(CACHE_PREFIX + key, value);
  },

  remove: (key: string): void => {
    localStorage.removeItem(CACHE_PREFIX + key);
  },

  clear: (): void => {
    Object.keys(localStorage).forEach((key) => {
      if (key.startsWith(CACHE_PREFIX)) {
        localStorage.removeItem(key);
      }
    });
  },
};

export const getCachedConfig = async (key: string): Promise<string | null> => {
  const cachedValue = configCache.get(key);

  if (cachedValue !== null) {
    return cachedValue;
  }

  const response = await getConfigByName(key);

  if (
    response.code === 0 &&
    response.data &&
    typeof response.data.value === "string"
  ) {
    const value = response.data.value;

    configCache.set(key, value);

    return value;
  }

  return null;
};

export const getCachedConfigs = async (): Promise<Record<string, string>> => {
  const configKeys = ["app_name", "app_logo", "app_favicon"];
  const cachedConfigs: Record<string, string> = {};
  let hasCachedData = false;

  configKeys.forEach((key) => {
    const cachedValue = configCache.get(key);

    if (cachedValue !== null) {
      cachedConfigs[key] = cachedValue;
      hasCachedData = true;
    }
  });

  const fetchPublicConfigs = async (): Promise<Record<string, string>> => {
    const publicConfigMap: Record<string, string> = {};

    await Promise.all(
      configKeys.map(async (key) => {
        try {
          const response = await getConfigByName(key);

          if (
            response.code === 0 &&
            response.data &&
            typeof response.data.value === "string"
          ) {
            const value = response.data.value;

            publicConfigMap[key] = value;
            configCache.set(key, value);
          }
        } catch {
          // Ignore single key fetch failures.
        }
      }),
    );

    return publicConfigMap;
  };

  try {
    const response = await getConfigs();

    if (response.code === 0 && response.data) {
      const configs = response.data;

      Object.entries(configs).forEach(([key, value]) => {
        configCache.set(key, value as string);
      });

      return configs;
    }

    if (hasCachedData) {
      return cachedConfigs;
    }

    return await fetchPublicConfigs();
  } catch {
    if (hasCachedData) {
      return cachedConfigs;
    }

    return await fetchPublicConfigs();
  }
};

const updateDocumentFavicon = (faviconUrl: string) => {
  if (typeof document === "undefined") {
    return;
  }

  const normalized = faviconUrl.trim() || DEFAULT_FAVICON;

  let iconLink = document.head.querySelector<HTMLLinkElement>(
    `link#${FAVICON_LINK_ID}`,
  );

  if (!iconLink) {
    iconLink = document.createElement("link");
    iconLink.id = FAVICON_LINK_ID;
    iconLink.rel = "icon";
    document.head.appendChild(iconLink);
  }

  iconLink.rel = "icon";
  iconLink.href = normalized;
  if (normalized.startsWith("data:image/png")) {
    iconLink.type = "image/png";
  } else {
    iconLink.removeAttribute("type");
  }

  let shortcutIconLink = document.head.querySelector<HTMLLinkElement>(
    'link[rel="shortcut icon"]',
  );

  if (!shortcutIconLink) {
    shortcutIconLink = document.createElement("link");
    shortcutIconLink.rel = "shortcut icon";
    document.head.appendChild(shortcutIconLink);
  }

  shortcutIconLink.href = normalized;
  if (normalized.startsWith("data:image/png")) {
    shortcutIconLink.type = "image/png";
  } else {
    shortcutIconLink.removeAttribute("type");
  }

  const duplicatedIcons = Array.from(
    document.head.querySelectorAll<HTMLLinkElement>('link[rel="icon"]'),
  ).filter((link) => link !== iconLink);

  duplicatedIcons.forEach((link) => link.remove());
};

export const updateSiteConfig = async (configMap?: Record<string, string>) => {
  const resolvedConfigMap = configMap ?? (await getCachedConfigs());

  Object.entries(resolvedConfigMap).forEach(([key, value]) => {
    configCache.set(key, String(value));
  });

  const hasAppName = Object.prototype.hasOwnProperty.call(
    resolvedConfigMap,
    "app_name",
  );
  const hasAppLogo = Object.prototype.hasOwnProperty.call(
    resolvedConfigMap,
    "app_logo",
  );
  const hasAppFavicon = Object.prototype.hasOwnProperty.call(
    resolvedConfigMap,
    "app_favicon",
  );

  const appName = hasAppName
    ? String(resolvedConfigMap.app_name || "").trim()
    : siteConfig.name;
  const appLogo = hasAppLogo
    ? String(resolvedConfigMap.app_logo || "").trim()
    : (siteConfig.app_logo || "").trim();
  const appFavicon = hasAppFavicon
    ? String(resolvedConfigMap.app_favicon || "").trim()
    : (siteConfig.app_favicon || "").trim();

  if (appName && appName !== siteConfig.name) {
    siteConfig.name = appName;
  }

  siteConfig.app_logo = appLogo;
  siteConfig.app_favicon = appFavicon;
  if (typeof document !== "undefined") {
    document.title = siteConfig.name;
  }
  updateDocumentFavicon(siteConfig.app_favicon);
};

export const clearConfigCache = (keys?: string[]) => {
  if (keys && keys.length > 0) {
    keys.forEach((key) => configCache.remove(key));
  } else {
    configCache.clear();
  }
};

if (typeof window !== "undefined") {
  if (typeof document !== "undefined") {
    document.title = siteConfig.name;
  }
  updateDocumentFavicon(siteConfig.app_favicon);

  setTimeout(() => {
    void updateSiteConfig();
  }, 50);
}
