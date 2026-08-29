import axios from "axios";
import type {
  CreateResourceParams,
  DownloadTokenResponse,
  RFile,
  PageResponse,
  Resource,
  ResourceDetails,
  Response,
  Storage,
  Tag,
  UploadingFile,
  User,
  UserWithToken,
  Comment,
  CommentWithResource,
  ServerConfig,
  RSort,
  TagWithCount,
  Activity,
  CommentWithRef,
  Collection,
  Statistics,
  ResourceStats,
  VndbInfo,
  VndbResourcePrefill,
  Config,
  ServerTask,
} from "./models.ts";

class Network {
  get apiBaseUrl() {
    if (typeof window !== "undefined") {
      return "/api";
    } else {
      return process.env.API_BASE_URL || "/api";
    }
  };

  constructor() {
    this.init();
  }

  private async _callApi<T>(request: () => Promise<{ data: T }>): Promise<T> {
    try {
      const response = await request();
      return response.data;
    } catch (e: any) {
      console.error(e);
      return {
        success: false,
        message: e.toString(),
      } as any;
    }
  }

  private getHeaders(additionalHeaders?: Record<string, string>) {
    return {
      ...additionalHeaders,
    };
  }

  init() {
    axios.defaults.validateStatus = (_) => true;
    axios.interceptors.response.use(
      (response) => {
        if (response.status >= 400 && response.status < 500) {
          const data = response.data;
          if (data.message) {
            throw new Error(data.message);
          } else {
            throw new Error(`Invalid response: ${response.status}`);
          }
        } else if (response.status >= 500) {
          throw new Error(`Server error: ${response.status}`);
        } else {
          return response;
        }
      },
      (error) => {
        return Promise.reject(error);
      },
    );
  }

  async login(
    username: string,
    password: string,
  ): Promise<Response<UserWithToken>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/user/login`, {
        username,
        password,
      }),
    );
  }

  async register(
    username: string,
    password: string,
    cfToken: string,
  ): Promise<Response<UserWithToken>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/user/register`, {
        username,
        password,
        cf_token: cfToken,
      }),
    );
  }

  async logout(): Promise<Response<void>> {
    return this._callApi(() => axios.post(`${this.apiBaseUrl}/user/logout`));
  }

  async getMe(): Promise<Response<UserWithToken>> {
    return this._callApi(() => axios.get(`${this.apiBaseUrl}/user/me`));
  }

  async getUserInfo(username: string): Promise<Response<User>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/user/info`, {
        params: {
          username,
        },
      }),
    );
  }

  async changePassword(
    oldPassword: string,
    newPassword: string,
  ): Promise<Response<UserWithToken>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/user/password`, {
        old_password: oldPassword,
        new_password: newPassword,
      }),
    );
  }

  async changeAvatar(file: File): Promise<Response<User>> {
    const formData = new FormData();
    formData.append("avatar", file);
    return this._callApi(() =>
      axios.put(`${this.apiBaseUrl}/user/avatar`, formData, {
        headers: {
          "Content-Type": "multipart/form-data",
        },
      }),
    );
  }

  async changeUsername(username: string): Promise<Response<User>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/user/username`, {
        username,
      }),
    );
  }

  async changeBio(bio: string): Promise<Response<User>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/user/bio`, {
        bio,
      }),
    );
  }

  getUserAvatar(user: User): string {
    return user.avatar_path;
  }

  async setUserAdmin(
    userId: number,
    isAdmin: boolean,
  ): Promise<Response<User>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/user/set_admin`, {
        user_id: userId,
        is_admin: isAdmin ? "true" : "false",
      }),
    );
  }

  async setUserUploadPermission(
    userId: number,
    canUpload: boolean,
  ): Promise<Response<User>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/user/set_upload_permission`, {
        user_id: userId,
        can_upload: canUpload ? "true" : "false",
      }),
    );
  }

  async listUsers(page: number): Promise<PageResponse<User>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/user/list`, {
        params: { page },
      }),
    );
  }

  async searchUsers(
    username: string,
    page: number,
  ): Promise<PageResponse<User>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/user/search`, {
        params: {
          username,
          page,
        },
      }),
    );
  }

  async deleteUser(userId: number): Promise<Response<void>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/user/delete`, {
        user_id: userId,
      }),
    );
  }

  async listBannedUsers(page: number): Promise<PageResponse<User>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/user/banned`, {
        params: { page },
      }),
    );
  }

  async unbanUser(userId: number): Promise<Response<User>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/user/unban`, {
        user_id: userId,
      }),
    );
  }

  async getAllTags(): Promise<Response<TagWithCount[]>> {
    return this._callApi(() => axios.get(`${this.apiBaseUrl}/tag`));
  }

  async searchTags(
    keyword: string,
    mainTag?: boolean,
  ): Promise<Response<Tag[]>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/tag/search`, {
        params: {
          keyword,
          main_tag: mainTag,
        },
      }),
    );
  }

  async searchTagSuggestions(keyword: string): Promise<Response<string[]>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/tag/suggestions`, {
        params: {
          keyword,
        },
      }),
    );
  }

  async createTag(name: string): Promise<Response<Tag>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/tag`, {
        name,
      }),
    );
  }

  async renameTag(tagId: number, newName: string): Promise<Response<Tag>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/tag/${tagId}/rename`, {
        name: newName,
      }),
    );
  }

  async getOrCreateTags(
    names: string[],
    tagType: string,
  ): Promise<Response<Tag[]>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/tag/batch`, {
        names,
        type: tagType,
      }),
    );
  }

  async getTagByName(name: string): Promise<Response<Tag>> {
    return this._callApi(() => axios.get(`${this.apiBaseUrl}/tag/${encodeURIComponent(name)}`));
  }

  async setTagInfo(
    tagId: number,
    description: string,
    aliasOf: number | null,
    type: string,
  ): Promise<Response<Tag>> {
    return this._callApi(() =>
      axios.putForm(`${this.apiBaseUrl}/tag/${tagId}/info`, {
        description,
        alias_of: aliasOf,
        type,
      }),
    );
  }

  async setTagAlias(tagID: number, aliases: string[]): Promise<Response<Tag>> {
    return this._callApi(() =>
      axios.put(`${this.apiBaseUrl}/tag/${tagID}/alias`, {
        aliases,
      }),
    );
  }

  /**
   * Upload image and return the image id
   */
  async uploadImage(file: File): Promise<Response<number>> {
    const data = await file.arrayBuffer();
    return this._callApi(() =>
      axios.put(`${this.apiBaseUrl}/image`, data, {
        headers: {
          "Content-Type": "application/octet-stream",
        },
      }),
    );
  }

  async deleteImage(id: number): Promise<Response<void>> {
    return this._callApi(() => axios.delete(`${this.apiBaseUrl}/image/${id}`));
  }

  getImageUrl(id: number): string {
    // Always use relative path to ensure SSR and client hydration match
    return `/image/${id}`;
  }

  getResampledImageUrl(id: number): string {
    // Always use relative path to ensure SSR and client hydration match
    return `/image/resampled/${id}`;
  }

  async createResource(
    params: CreateResourceParams,
  ): Promise<Response<number>> {
    console.log(this);
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/resource`, params),
    );
  }

  async editResource(
    id: number,
    params: CreateResourceParams,
  ): Promise<Response<void>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/resource/${id}`, params),
    );
  }

  async getResources(
    page: number,
    sort: RSort,
  ): Promise<PageResponse<Resource>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/resource`, {
        params: {
          page,
          sort,
        },
      }),
    );
  }

  async getAdminResourceStats(
    page: number,
    sort?: "views_asc" | "views_desc" | "downloads_asc" | "downloads_desc",
  ): Promise<PageResponse<ResourceStats>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/resource/admin/all`, {
        params: {
          page,
          sort,
        },
      }),
    );
  }

  async getResourcesByTag(
    tag: string,
    page: number,
    sort?: RSort,
  ): Promise<PageResponse<Resource>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/resource/tag/${encodeURIComponent(tag)}`, {
        params: {
          page,
          sort,
        },
      }),
    );
  }

  async getResourcesByUser(
    username: string,
    page: number,
  ): Promise<PageResponse<Resource>> {
    return this._callApi(() =>
      axios.get(
        `${this.apiBaseUrl}/resource/user/${encodeURIComponent(username)}`,
        {
          params: {
            page,
          },
        },
      ),
    );
  }

  async searchResources(
    keyword: string,
    page: number,
    options?: {
      tags?: string[];
      releaseFrom?: string;
      releaseTo?: string;
    },
  ): Promise<PageResponse<Resource>> {
    const params: Record<string, string | number> = { page };
    const trimmedKeyword = keyword.trim();
    if (trimmedKeyword) {
      params.keyword = trimmedKeyword;
    }
    if (options?.tags && options.tags.length > 0) {
      params.tags = options.tags.join(",");
    }
    if (options?.releaseFrom) {
      params.release_from = options.releaseFrom;
    }
    if (options?.releaseTo) {
      params.release_to = options.releaseTo;
    }
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/resource/search`, {
        params,
      }),
    );
  }

  async getResourceDetails(id: number, cookie?: string): Promise<Response<ResourceDetails>> {
    return this._callApi<Response<ResourceDetails>>(() =>
      axios.get(`${this.apiBaseUrl}/resource/${id}`, {
        headers: this.getHeaders(cookie ? { Cookie: cookie } : undefined),
      }),
    );
  }

  async addResourceView(id: number): Promise<Response<void>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/resource/${id}/view`),
    );
  }

  async getRandomResource(): Promise<Response<Resource>> {
    return this._callApi(() => axios.get(`${this.apiBaseUrl}/resource/random`));
  }

  async deleteResource(id: number): Promise<Response<void>> {
    return this._callApi(() =>
      axios.delete(`${this.apiBaseUrl}/resource/${id}`),
    );
  }

  async getPinnedResources(): Promise<Response<Resource[]>> {
    return this._callApi(() => axios.get(`${this.apiBaseUrl}/resource/pinned`));
  }

  async createS3Storage(
    name: string,
    endPoint: string,
    accessKeyID: string,
    secretAccessKey: string,
    bucketName: string,
    maxSizeInMB: number,
    domain: string,
  ): Promise<Response<any>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/storage/s3`, {
        name,
        endPoint,
        accessKeyID,
        secretAccessKey,
        bucketName,
        maxSizeInMB,
        domain,
      }),
    );
  }

  async createLocalStorage(
    name: string,
    path: string,
    maxSizeInMB: number,
  ): Promise<Response<any>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/storage/local`, {
        name,
        path,
        maxSizeInMB,
      }),
    );
  }

  async createFTPStorage(
    name: string,
    host: string,
    username: string,
    password: string,
    basePath: string,
    domain: string,
    maxSizeInMB: number,
  ): Promise<Response<any>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/storage/ftp`, {
        name,
        host,
        username,
        password,
        basePath,
        domain,
        maxSizeInMB,
      }),
    );
  }

  async listStorages(): Promise<Response<Storage[]>> {
    return this._callApi(() => axios.get(`${this.apiBaseUrl}/storage`));
  }

  async deleteStorage(id: number): Promise<Response<void>> {
    return this._callApi(() =>
      axios.delete(`${this.apiBaseUrl}/storage/${id}`),
    );
  }

  async setDefaultStorage(id: number): Promise<Response<Storage>> {
    return this._callApi(() =>
      axios.put(`${this.apiBaseUrl}/storage/${id}/default`),
    );
  }

  async initFileUpload(
    filename: string,
    description: string,
    fileSize: number,
    resourceId: number,
    storageId: number,
    tag: string,
  ): Promise<Response<UploadingFile>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/files/upload/init`, {
        filename,
        description,
        file_size: fileSize,
        resource_id: resourceId,
        storage_id: storageId,
        tag,
      }),
    );
  }

  async uploadFileBlock(
    fileId: number,
    index: number,
    blockData: ArrayBuffer,
  ): Promise<Response<any>> {
    return this._callApi(() =>
      axios.post(
        `${this.apiBaseUrl}/files/upload/block/${fileId}/${index}`,
        blockData,
        {
          headers: {
            "Content-Type": "application/octet-stream",
          },
        },
      ),
    );
  }

  async finishFileUpload(
    fileId: number,
    md5: string,
  ): Promise<Response<RFile>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/files/upload/finish/${fileId}?md5=${md5}`),
    );
  }

  async cancelFileUpload(fileId: number): Promise<Response<void>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/files/upload/cancel/${fileId}`),
    );
  }

  async createRedirectFile(
    filename: string,
    description: string,
    resourceId: number,
    redirectUrl: string,
    fileSize: number,
    md5: string,
    tag: string,
  ): Promise<Response<RFile>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/files/redirect`, {
        filename,
        description,
        resource_id: resourceId,
        redirect_url: redirectUrl,
        file_size: fileSize,
        md5,
        tag,
      }),
    );
  }

  async createServerDownloadTask(
    url: string,
    filename: string,
    description: string,
    resourceId: number,
    storageId: number,
    tag: string,
  ): Promise<Response<RFile>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/files/upload/url`, {
        url,
        filename,
        description,
        resource_id: resourceId,
        storage_id: storageId,
        tag,
      }),
    );
  }

  async createFileMigrationTask(
    fileId: string,
    targetStorageId: number,
  ): Promise<Response<ServerTask>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/files/migrate`, {
        file_id: fileId,
        target_storage_id: targetStorageId,
      }),
    );
  }

  async listServerTasks(): Promise<Response<ServerTask[]>> {
    return this._callApi(() => axios.get(`${this.apiBaseUrl}/files/tasks`));
  }

  async getServerTask(taskId: string): Promise<Response<ServerTask>> {
    return this._callApi(() => axios.get(`${this.apiBaseUrl}/files/tasks/${taskId}`));
  }

  async stopServerTask(taskId: string): Promise<Response<ServerTask>> {
    return this._callApi(() => axios.post(`${this.apiBaseUrl}/files/tasks/${taskId}/stop`));
  }

  async getFile(fileId: string): Promise<Response<RFile>> {
    return this._callApi(() => axios.get(`${this.apiBaseUrl}/files/${fileId}`));
  }

  async updateFile(
    fileId: string,
    filename: string,
    description: string,
    tag: string,
  ): Promise<Response<RFile>> {
    return this._callApi(() =>
      axios.put(`${this.apiBaseUrl}/files/${fileId}`, {
        filename,
        description,
        tag,
      }),
    );
  }

  async deleteFile(fileId: string): Promise<Response<void>> {
    return this._callApi(() =>
      axios.delete(`${this.apiBaseUrl}/files/${fileId}`),
    );
  }

  async getUserFiles(
    username: string,
    page: number = 1,
  ): Promise<PageResponse<RFile>> {
    return this._callApi(() =>
      axios.get(
        `${this.apiBaseUrl}/files/user/${encodeURIComponent(username)}`,
        {
          params: { page },
        },
      ),
    );
  }

  async createFileDownloadToken(fileId: string): Promise<Response<DownloadTokenResponse>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/files/download/token/${fileId}`),
    );
  }

  getFileDownloadLink(
    fileId: string,
    cfToken: string = "",
    downloadToken: string = "",
  ): string {
    const params = new URLSearchParams();
    if (cfToken) {
      params.set("cf_token", cfToken);
    }
    if (downloadToken) {
      params.set("download_token", downloadToken);
    }
    const query = params.toString();
    if (!query) {
      return `${this.apiBaseUrl}/files/download/${fileId}`;
    }
    return `${this.apiBaseUrl}/files/download/${fileId}?${query}`;
  }

  async createResourceComment(
    resourceID: number,
    content: string,
  ): Promise<Response<any>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/comments/resource/${resourceID}`, {
        content,
      }),
    );
  }

  async updateComment(
    commentID: number,
    content: string,
  ): Promise<Response<any>> {
    return this._callApi(() =>
      axios.put(`${this.apiBaseUrl}/comments/${commentID}`, {
        content,
      }),
    );
  }

  async replyToComment(
    commentID: number,
    content: string,
  ): Promise<Response<any>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/comments/reply/${commentID}`, {
        content,
      }),
    );
  }

  async listResourceComments(
    resourceID: number,
    page: number = 1,
  ): Promise<PageResponse<Comment>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/comments/resource/${resourceID}`, {
        params: { page },
      }),
    );
  }

  async listCommentsByUser(
    username: string,
    page: number = 1,
  ): Promise<PageResponse<CommentWithResource>> {
    return this._callApi(() =>
      axios.get(
        `${this.apiBaseUrl}/comments/user/${encodeURIComponent(username)}`,
        {
          params: { page },
        },
      ),
    );
  }

  async listCommentReplies(
    commentID: number,
    page: number = 1,
  ): Promise<PageResponse<Comment>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/comments/reply/${commentID}`, {
        params: { page },
      }),
    );
  }

  async getComment(commentID: number): Promise<Response<CommentWithRef>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/comments/${commentID}`),
    );
  }

  async deleteComment(commentID: number): Promise<Response<void>> {
    return this._callApi(() =>
      axios.delete(`${this.apiBaseUrl}/comments/${commentID}`),
    );
  }

  async getServerConfig(): Promise<Response<ServerConfig>> {
    return this._callApi(() => axios.get(`${this.apiBaseUrl}/config`));
  }

  async setServerConfig(config: ServerConfig): Promise<Response<void>> {
    return this._callApi(() => axios.post(`${this.apiBaseUrl}/config`, config));
  }

  async getActivities(page: number = 1): Promise<PageResponse<Activity>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/activity`, {
        params: { page },
      }),
    );
  }

  async getUserNotifications(page: number = 1, cookie?: string): Promise<PageResponse<Activity>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/notification`, {
        params: { page },
        headers: this.getHeaders(cookie ? { Cookie: cookie } : undefined),
      }),
    );
  }

  async resetUserNotificationsCount(): Promise<Response<void>> {
    return this._callApi(() =>
      axios.post(`${this.apiBaseUrl}/notification/reset`),
    );
  }

  async getUserNotificationsCount(): Promise<Response<number>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/notification/count`),
    );
  }

  async createCollection(
    title: string,
    article: string,
    isPublic: boolean,
  ): Promise<Response<Collection>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/collection/create`, {
        title,
        article,
        public: isPublic,
      }),
    );
  }

  async updateCollection(
    id: number,
    title: string,
    article: string,
    isPublic: boolean,
  ): Promise<Response<any>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/collection/update`, {
        id,
        title,
        article,
        public: isPublic,
      }),
    );
  }

  async deleteCollection(id: number): Promise<Response<any>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/collection/delete`, {
        id,
      }),
    );
  }

  async getCollection(id: number, cookie?: string): Promise<Response<Collection>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/collection/${id}`, {
        headers: this.getHeaders(cookie ? { Cookie: cookie } : undefined),
      }),
    );
  }

  async listUserCollections(
    username: string,
    page: number = 1,
  ): Promise<PageResponse<Collection>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/collection/list`, {
        params: { username, page },
      }),
    );
  }

  async listCollectionResources(
    collectionId: number,
    page: number = 1,
    cookie?: string,
  ): Promise<PageResponse<Resource>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/collection/${collectionId}/resources`, {
        params: { page },
        headers: this.getHeaders(cookie ? { Cookie: cookie } : undefined),
      }),
    );
  }

  async addResourceToCollection(
    collectionId: number,
    resourceId: number,
  ): Promise<Response<any>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/collection/add_resource`, {
        collection_id: collectionId,
        resource_id: resourceId,
      }),
    );
  }

  async removeResourceFromCollection(
    collectionId: number,
    resourceId: number,
  ): Promise<Response<any>> {
    return this._callApi(() =>
      axios.postForm(`${this.apiBaseUrl}/collection/remove_resource`, {
        collection_id: collectionId,
        resource_id: resourceId,
      }),
    );
  }

  async searchUserCollections(
    username: string,
    keyword: string,
    excludedRID?: number,
  ): Promise<Response<Collection[]>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/collection/search`, {
        params: { username, keyword, excludedRID },
      }),
    );
  }

  async getStatistic(): Promise<Response<Statistics>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/config/statistics`),
    );
  }

  async getInfoFromVNDB(vnID: string): Promise<Response<VndbInfo>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/resource/vndb/info`, {
        params: { vnid: vnID },
      }),
    );
  }

  async getResourcePrefillFromVNDB(vnID: string, sections?: string[]): Promise<Response<VndbResourcePrefill>> {
    return this._callApi(() =>
      axios.get(`${this.apiBaseUrl}/resource/vndb/prefill`, {
        params: { vnid: vnID, sections: sections?.join(",") },
      }),
    );
  }

  async getFrontendConfig(cookie?: string): Promise<Response<Config>> {
    return this._callApi(() => axios.get(`${this.apiBaseUrl}/config/frontend`, {
      headers: this.getHeaders(cookie ? { Cookie: cookie } : undefined),
    }));
  }

  async getSiteInfo(): Promise<Response<any>> {
    return this._callApi(() => axios.get(`${this.apiBaseUrl}/config/site-info`));
  }
}

export const network = new Network();
