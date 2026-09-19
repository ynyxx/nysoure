import axios from "axios";
import type { Response } from "./models.ts";

const KunApi = {
  isAvailable(): boolean {
    return true;
  },

  async getPatch(vndbId: string): Promise<Response<MoyuPatch>> {
    try {
      const client = axios.create({
        validateStatus(status) {
          return status === 200 || status === 404; // Accept only 200 and 404 responses
        },
      });
      const res = await client.get<MoyuPatchList>(
        `/api/moyu/patch?vndb_id=${encodeURIComponent(vndbId)}`,
      );
      const patch = res.status === 200 ? res.data.items[0] : undefined;
      if (!patch) {
        return {
          success: false,
          message: "404",
        };
      }
      return {
        success: true,
        message: "ok",
        data: patch,
      };
    } catch (error) {
      console.error("Error fetching files:", error);
      return { success: false, message: "Failed to fetch files" };
    }
  },
};

export default KunApi;

// Schemas of NextMoe's /v2/moyu API, see
// https://developer.nextmoe.dev/specs/moyu-openapi.yaml

export interface MoyuUser {
  object: "user";
  id: string;
  name: string;
  avatar_url: string;
}

export interface MoyuPatch {
  object: "patch";
  id: string;
  // e.g. "vndb_id": "v19658",
  vndb_id: string;
  catalog_work_id: string | null;
  content_limit: "sfw" | "nsfw" | null;
  // e.g. "release_date": "2016-11-25",
  release_date: string | null;
  type: string[];
  language: string[];
  platform: string[];
  resource_count: number;
  download_count: number;
  view_count: number;
  favorite_count: number;
  comment_count: number;
  web_url: string;
  created_at: string;
  updated_at: string;
  resource_updated_at: string;
  publisher?: MoyuUser;
  resources?: MoyuPatchResource[];
}

export interface MoyuPatchResource {
  object: "patch_resource";
  id: string;
  patch_id: string;
  name: string;
  storage: "s3" | "user";
  size: string;
  hash: string;
  model_name: string;
  localization_group_name: string;
  note: string;
  type: string[];
  language: string[];
  platform: string[];
  download_count: number;
  like_count: number;
  web_url: string;
  created_at: string;
  updated_at: string;
  publisher?: MoyuUser;
}

export interface MoyuPatchList {
  object: "list";
  items: MoyuPatch[];
  next_cursor: string | null;
  total: number | null;
  missing?: string[];
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
