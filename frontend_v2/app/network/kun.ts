import axios from "axios";
import type { Response } from "./models.ts";

const KunApi = {
  isAvailable(): boolean {
    return true;
  },

  async getPatch(id: string): Promise<Response<KunPatchResponse>> {
    try {
      const client = axios.create({
        validateStatus(status) {
          return status === 200 || status === 404;
        },
      });
      const res = await client.get("/api/moyu/patch", {
        params: { vndb_id: id },
      });
      if (res.status === 404) {
        return {
          success: false,
          message: "404",
        };
      }
      if (res.status !== 200) {
        throw new Error(`HTTP error! status: ${res.status}`);
      }
      return {
        success: true,
        message: "ok",
        data: res.data.data,
      };
    } catch (error) {
      console.error("Error fetching files:", error);
      return { success: false, message: "Failed to fetch files" };
    }
  },
};

export default KunApi;

export interface KunUser {
  id: string;
  name: string;
  avatar_url: string;
}

export interface KunPatchResponse {
  id: string;
  vndb_id: string;
  web_url: string;
  type: string[];
  language: string[];
  platform: string[];
  resource_count: number;
  resources: KunPatchResourceResponse[];
}

export interface KunPatchResourceResponse {
  id: string;
  storage: "s3" | "user";
  name: string;
  model_name: string;
  localization_group_name: string;
  size: string;
  note: string;
  hash: string;
  type: string[];
  language: string[];
  platform: string[];
  download_count: number;
  web_url: string;
  created_at: string;
  updated_at: string;
  patch_id: string;
  publisher?: KunUser;
}

const SUPPORTED_LANGUAGE_MAP: Record<string, string> = {
  "zh-Hans": "简体中文",
  "zh-Hant": "繁體中文",
  "ja": "日本語",
  "en": "English",
  "other": "其它",
};

export function kunLanguageToString(language: string): string {
  return SUPPORTED_LANGUAGE_MAP[language] || language;
}

const SUPPORTED_PLATFORM_MAP: Record<string, string> = {
  windows: "Windows",
  android: "Android",
  macos: "MacOS",
  ios: "iOS",
  linux: "Linux",
  other: "其它",
};

export function kunPlatformToString(platform: string): string {
  return SUPPORTED_PLATFORM_MAP[platform] || platform;
}

const resourceTypes = [
  {
    value: "manual",
    label: "人工翻译补丁",
  },
  {
    value: "ai",
    label: "AI 翻译补丁",
  },
  {
    value: "machine_polishing",
    label: "机翻润色",
  },
  {
    value: "machine",
    label: "机翻补丁",
  },
  {
    value: "save",
    label: "全 CG 存档",
  },
  {
    value: "crack",
    label: "破解补丁",
  },
  {
    value: "fix",
    label: "修正补丁",
  },
  {
    value: "mod",
    label: "魔改补丁",
  },
  {
    value: "other",
    label: "其它",
  },
];

export function kunResourceTypeToString(type: string): string {
  const resourceType = resourceTypes.find((t) => t.value === type);
  return resourceType ? resourceType.label : type;
}
