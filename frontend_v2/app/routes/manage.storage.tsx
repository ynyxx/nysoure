import { useEffect, useState, useCallback } from "react";
import type { Storage } from "~/network/models";
import { network } from "~/network/network";
import showToast from "~/components/toast";
import Loading from "~/components/loading";
import { MdAdd, MdMoreHoriz } from "react-icons/md";
import { ErrorAlert } from "~/components/alert";
import { useTranslation } from "~/hook/i18n";
import { configFromMatches, useConfig, isAdmin } from "~/hook/config";
import showPopup, { PopupMenuItem } from "~/components/popup";
import Badge from "~/components/badge";
import type { Route } from "./+types/manage.storage";

export function meta({ matches }: Route.MetaArgs) {
  const config = configFromMatches(matches);
  return [
    { title: config.server_name },
    { name: "description", content: config.site_description },
  ];
}
export default function StorageView() {
  const { t } = useTranslation();
  const config = useConfig();
  const [storages, setStorages] = useState<Storage[] | null>(null);
  const [loadingId, setLoadingId] = useState<number | null>(null);

  const updateStorages = useCallback(async () => {
    setStorages(null);
    const response = await network.listStorages();
    if (response.success) {
      setStorages(response.data!);
    } else {
      showToast({
        message: response.message,
        type: "error",
      });
    }
  }, []);

  useEffect(() => {
    if (config.user == null || !isAdmin(config)) {
      return;
    }
    network.listStorages().then((response) => {
      if (response.success) {
        setStorages(response.data!);
      } else {
        showToast({
          message: response.message,
          type: "error",
        });
      }
    });
  }, [config.user]);

  const handleDelete = useCallback(async (id: number) => {
    if (loadingId != null) {
      return;
    }
    setLoadingId(id);
    try {
      const response = await network.deleteStorage(id);
      if (response.success) {
        showToast({
          message: t("Storage deleted successfully"),
        });
        await updateStorages();
      } else {
        showToast({
          message: response.message,
          type: "error",
        });
      }
    } finally {
      setLoadingId(null);
    }
  }, [loadingId, t, updateStorages]);

  const handleSetDefault = useCallback(async (id: number) => {
    if (loadingId != null) {
      return;
    }
    setLoadingId(id);
    try {
      const response = await network.setDefaultStorage(id);
      if (response.success) {
        showToast({
          message: t("Storage set as default successfully"),
        });
        await updateStorages();
      } else {
        showToast({
          message: response.message,
          type: "error",
        });
      }
    } finally {
      setLoadingId(null);
    }
  }, [loadingId, t, updateStorages]);

  if (!config.user) {
    return (
      <ErrorAlert
        className={"m-4"}
        message={t("You are not logged in. Please log in to access this page.")}
      />
    );
  }

  if (!isAdmin(config)) {
    return (
      <ErrorAlert
        className={"m-4"}
        message={t("You are not authorized to access this page.")}
      />
    );
  }

  if (storages == null) {
    return <Loading />;
  }

  return (
    <>
      <div
        role="alert"
        className={`alert alert-info alert-outline ${storages.length !== 0 && "hidden"} mx-4 mb-4`}
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
          className="h-6 w-6 shrink-0 stroke-current"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth="2"
            d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
          ></path>
        </svg>
        <span>{t("No storage found. Please create a new storage.")}</span>
      </div>
      <div
        className={`rounded-box border border-base-content/10 bg-base-100 mx-4 mb-4 overflow-x-auto ${storages.length === 0 ? "hidden" : ""}`}
      >
        <table className={"table"}>
          <thead>
            <tr>
              <td>{t("Name")}</td>
              <td>{t("Created At")}</td>
              <td>{t("Space")}</td>
              <td>{t("Action")}</td>
            </tr>
          </thead>
          <tbody>
            {storages.map((s) => {
              return (
                <tr key={s.id} className={"hover"}>
                  <td>
                    {s.name}
                    {s.isDefault && (
                      <Badge className={"ml-1"}>{t("Default")}</Badge>
                    )}
                  </td>
                  <td>{new Date(s.createdAt).toLocaleString()}</td>
                  <td>
                    {(s.currentSize / 1024 / 1024).toFixed(2)} /{" "}
                    {s.maxSize / 1024 / 1024} MB
                  </td>
                  <td>
                    <button
                      id={`set_default_button_${s.id}`}
                      className={"btn btn-square"}
                      type={"button"}
                      onClick={() => {
                        showPopup(
                          <ul className="menu bg-base-100 rounded-box z-1 w-64 p-2 shadow-sm">
                            <h4 className="text-sm font-bold px-3 py-1 text-primary">
                              {t("Actions")}
                            </h4>
                            <PopupMenuItem
                              onClick={() => {
                                const dialog = document.getElementById(
                                  `confirm_delete_dialog_${s.id}`,
                                ) as HTMLDialogElement;
                                dialog.showModal();
                              }}
                            >
                              <a>{t("Delete")}</a>
                            </PopupMenuItem>
                            {!s.isDefault && (
                              <PopupMenuItem
                                onClick={() => {
                                  handleSetDefault(s.id);
                                }}
                              >
                                <a>{t("Set as Default")}</a>
                              </PopupMenuItem>
                            )}
                          </ul>,
                          document.getElementById(
                            `set_default_button_${s.id}`,
                          )!,
                        );
                      }}
                    >
                      {loadingId === s.id ? (
                        <span
                          className={"loading loading-spinner loading-sm"}
                        ></span>
                      ) : (
                        <MdMoreHoriz size={24} />
                      )}
                    </button>
                    <dialog
                      id={`confirm_delete_dialog_${s.id}`}
                      className="modal"
                    >
                      <div className="modal-box">
                        <h3 className="text-lg font-bold">
                          {t("Delete Storage")}
                        </h3>
                        <p className="py-4">
                          {t(
                            "Are you sure you want to delete this storage? This action cannot be undone.",
                          )}
                        </p>
                        <div className="modal-action">
                          <form method="dialog">
                            <button className="btn">{t("Cancel")}</button>
                          </form>
                          <button
                            className="btn btn-error"
                            onClick={() => {
                              handleDelete(s.id);
                            }}
                          >
                            {t("Delete")}
                          </button>
                        </div>
                      </div>
                    </dialog>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      <div className={"flex flex-row-reverse px-4"}>
        <NewStorageDialog onAdded={updateStorages} />
      </div>
    </>
  );
}

enum StorageType {
  local,
  s3,
  ftp,
  gdrive,
}

function NewStorageDialog({ onAdded }: { onAdded: () => void }) {
  const { t } = useTranslation();
  const [storageType, setStorageType] = useState<StorageType | null>(null);

  const [params, setParams] = useState({
    name: "",
    path: "",
    endPoint: "",
    accessKeyID: "",
    secretAccessKey: "",
    bucketName: "",
    maxSizeInMB: 0,
    domain: "",
    host: "",
    username: "",
    password: "",
    basePath: "",
    clientID: "",
    clientSecret: "",
    refreshToken: "",
    folderID: "",
  });

  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState("");

  const handleSubmit = async () => {
    if (storageType == null) {
      return;
    }
    setError("");
    setIsLoading(true);

    let response;
    if (storageType === StorageType.local) {
      if (params.path === "" || params.name === "" || params.maxSizeInMB <= 0) {
        setError(t("All fields are required"));
        setIsLoading(false);
        return;
      }
      response = await network.createLocalStorage(
        params.name,
        params.path,
        params.maxSizeInMB,
      );
    } else if (storageType === StorageType.s3) {
      if (
        params.endPoint === "" ||
        params.accessKeyID === "" ||
        params.secretAccessKey === "" ||
        params.bucketName === "" ||
        params.name === "" ||
        params.maxSizeInMB <= 0
      ) {
        setError(t("All fields are required"));
        setIsLoading(false);
        return;
      }
      response = await network.createS3Storage(
        params.name,
        params.endPoint,
        params.accessKeyID,
        params.secretAccessKey,
        params.bucketName,
        params.maxSizeInMB,
        params.domain,
      );
    } else if (storageType === StorageType.ftp) {
      if (
        params.host === "" ||
        params.username === "" ||
        params.password === "" ||
        params.domain === "" ||
        params.name === "" ||
        params.maxSizeInMB <= 0
      ) {
        setError(t("All fields are required"));
        setIsLoading(false);
        return;
      }
      response = await network.createFTPStorage(
        params.name,
        params.host,
        params.username,
        params.password,
        params.basePath,
        params.domain,
        params.maxSizeInMB,
      );
    } else if (storageType === StorageType.gdrive) {
      if (
        params.name === "" ||
        params.clientID === "" ||
        params.clientSecret === "" ||
        params.refreshToken === "" ||
        params.maxSizeInMB <= 0
      ) {
        setError(t("All fields are required"));
        setIsLoading(false);
        return;
      }
      response = await network.createGoogleDriveStorage(
        params.name,
        params.clientID,
        params.clientSecret,
        params.refreshToken,
        params.folderID,
        params.maxSizeInMB,
      );
    }

    if (response!.success) {
      showToast({
        message: t("Storage created successfully"),
      });
      onAdded();
      const dialog = document.getElementById(
        "new_storage_dialog",
      ) as HTMLDialogElement;
      dialog.close();
    } else {
      setError(response!.message);
    }
    setIsLoading(false);
  };

  return (
    <>
      <button
        className="btn"
        onClick={() => {
          const dialog = document.getElementById(
            "new_storage_dialog",
          ) as HTMLDialogElement;
          dialog.showModal();
        }}
      >
        <MdAdd />
        {t("New Storage")}
      </button>
      <dialog id="new_storage_dialog" className="modal">
        <div className="modal-box">
          <h3 className="font-bold text-lg pb-4">{t("New Storage")}</h3>

          <p className={"text-sm font-bold p-2"}>{t("Type")}</p>
          <form className="filter mb-2">
            <input
              className="btn btn-square"
              type="reset"
              value="×"
              onClick={() => {
                setStorageType(null);
              }}
            />
            <input
              className="btn"
              type="radio"
              name="type"
              aria-label={t("Local")}
              onInput={() => {
                setStorageType(StorageType.local);
              }}
            />
            <input
              className="btn"
              type="radio"
              name="type"
              aria-label={t("S3")}
              onInput={() => {
                setStorageType(StorageType.s3);
              }}
            />
            <input
              className="btn"
              type="radio"
              name="type"
              aria-label={t("FTP")}
              onInput={() => {
                setStorageType(StorageType.ftp);
              }}
            />
            <input
              className="btn"
              type="radio"
              name="type"
              aria-label={t("Google Drive")}
              onInput={() => {
                setStorageType(StorageType.gdrive);
              }}
            />
          </form>

          {storageType === StorageType.local && (
            <>
              <label className="input w-full my-2">
                {t("Name")}
                <input
                  type="text"
                  className="w-full"
                  value={params.name}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      name: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Path")}
                <input
                  type="text"
                  className="w-full"
                  value={params.path}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      path: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Max Size (MB)")}
                <input
                  type="number"
                  className="validator"
                  required
                  min="0"
                  value={params.maxSizeInMB.toString()}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      maxSizeInMB: parseInt(e.target.value),
                    });
                  }}
                />
              </label>
            </>
          )}

          {storageType === StorageType.s3 && (
            <>
              <label className="input w-full my-2">
                {t("Name")}
                <input
                  type="text"
                  className="w-full"
                  value={params.name}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      name: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Endpoint")}
                <input
                  type="text"
                  className="w-full"
                  value={params.endPoint}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      endPoint: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Access Key ID")}
                <input
                  type="text"
                  className="w-full"
                  value={params.accessKeyID}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      accessKeyID: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Secret Access Key")}
                <input
                  type="text"
                  className="w-full"
                  value={params.secretAccessKey}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      secretAccessKey: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Bucket Name")}
                <input
                  type="text"
                  className="w-full"
                  value={params.bucketName}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      bucketName: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Domain")}
                <input
                  type="text"
                  placeholder={t("Optional")}
                  className="w-full"
                  value={params.domain}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      domain: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Max Size (MB)")}
                <input
                  type="number"
                  className="validator"
                  required
                  min="0"
                  value={params.maxSizeInMB.toString()}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      maxSizeInMB: parseInt(e.target.value),
                    });
                  }}
                />
              </label>
            </>
          )}

          {storageType === StorageType.ftp && (
            <>
              <label className="input w-full my-2">
                {t("Name")}
                <input
                  type="text"
                  className="w-full"
                  value={params.name}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      name: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Host")}
                <input
                  type="text"
                  placeholder="ftp.example.com:21"
                  className="w-full"
                  value={params.host}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      host: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Username")}
                <input
                  type="text"
                  className="w-full"
                  value={params.username}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      username: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Password")}
                <input
                  type="password"
                  className="w-full"
                  value={params.password}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      password: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Base Path")}
                <input
                  type="text"
                  placeholder="/uploads"
                  className="w-full"
                  value={params.basePath}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      basePath: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Domain")}
                <input
                  type="text"
                  placeholder="files.example.com"
                  className="w-full"
                  value={params.domain}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      domain: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Max Size (MB)")}
                <input
                  type="number"
                  className="validator"
                  required
                  min="0"
                  value={params.maxSizeInMB.toString()}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      maxSizeInMB: parseInt(e.target.value),
                    });
                  }}
                />
              </label>
            </>
          )}

          {storageType === StorageType.gdrive && (
            <>
              <label className="input w-full my-2">
                {t("Name")}
                <input
                  type="text"
                  className="w-full"
                  value={params.name}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      name: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Client ID")}
                <input
                  type="text"
                  className="w-full"
                  value={params.clientID}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      clientID: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Client Secret")}
                <input
                  type="password"
                  className="w-full"
                  value={params.clientSecret}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      clientSecret: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Refresh Token")}
                <input
                  type="password"
                  className="w-full"
                  value={params.refreshToken}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      refreshToken: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Folder ID")}
                <input
                  type="text"
                  placeholder={t("Optional")}
                  className="w-full"
                  value={params.folderID}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      folderID: e.target.value,
                    });
                  }}
                />
              </label>
              <label className="input w-full my-2">
                {t("Max Size (MB)")}
                <input
                  type="number"
                  className="validator"
                  required
                  min="0"
                  value={params.maxSizeInMB.toString()}
                  onChange={(e) => {
                    setParams({
                      ...params,
                      maxSizeInMB: parseInt(e.target.value),
                    });
                  }}
                />
              </label>
            </>
          )}

          {error !== "" && <ErrorAlert message={error} className={"my-2"} />}

          <div className="modal-action">
            <form method="dialog">
              <button className="btn btn-ghost">{t("Close")}</button>
            </form>
            <button
              className={"btn btn-primary"}
              onClick={handleSubmit}
              type={"button"}
            >
              {isLoading && (
                <span
                  className={"loading loading-spinner loading-sm mr-2"}
                ></span>
              )}
              {t("Submit")}
            </button>
          </div>
        </div>
      </dialog>
    </>
  );
}
