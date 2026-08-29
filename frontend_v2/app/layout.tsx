import { useEffect, useState } from "react";
import { Outlet, NavLink, useNavigate, useLocation } from "react-router";
import { MdArrowUpward, MdClose, MdCloudUpload, MdMenu, MdOutlinePerson, MdOutlinePublish, MdShuffle, MdTimeline, MdInfoOutline, MdOutlineLabel, MdSearch, MdLogout, MdNotifications, MdOutlineSettings } from "react-icons/md";
import { useTranslation } from "./hook/i18n.js";
import { useConfig } from "./hook/config.js";
import { Permission } from "./network/models.ts";
import { ThemeSwitcher } from "./components/theme_switcher.js";
import { network } from "./network/network.js";
import { Background } from "./components/background.js";
import { uploadingManager, UploadingTask, UploadingStatus } from "./network/uploading.js";

export default function Layout() {
  const { server_name } = useConfig();

  return (
    <Background>
      <Navigator appName={server_name} />
      <div className="pt-20 max-w-8xl mx-auto px-2">
        <Outlet />
      </div>
    </Background>
  )
}

function Navigator({ appName }: { appName: string }) {
  const { t } = useTranslation();

  return (
      <div
        style={{
          position: "relative",
          zIndex: 2,
        }}
      >
        <FloatingToTopButton />
        <div className="z-3 fixed top-2 left-2 right-2 backdrop-blur-xs h-16 rounded-box max-w-8xl mx-auto" />
        <div className="z-4 fixed top-2 left-2 right-2 h-16 bg-base-100/90 rounded-box max-w-8xl mx-auto" />
        <div
          className="shadow-lg fixed top-2 left-2 right-2 z-5 lg:z-10 bg-transparent h-16 rounded-box px-2 lg:px-4 flex items-center max-w-8xl mx-auto"
        >
          <div className={"flex-1 flex items-center w-full"}>
            <div className="dropdown">
              <div
                tabIndex={0}
                role="button"
                className="btn btn-ghost btn-circle lg:hidden"
              >
                <MdMenu size={24} />
              </div>
              <ul
                tabIndex={0}
                className="menu menu-md dropdown-content bg-base-100 rounded-box z-2 mt-3 w-52 p-2 shadow"
              >
                <li
                  onClick={() => {
                    (document.activeElement as HTMLElement)?.blur();
                  }}
                >
                  <NavLink to="/tags">
                    <MdOutlineLabel size={18} />
                    {t("Tags")}
                  </NavLink>
                </li>
                <li
                  onClick={() => {
                    (document.activeElement as HTMLElement)?.blur();
                  }}
                >
                  <NavLink to="/activity">
                    <MdTimeline size={18} />
                    {t("Activity")}
                  </NavLink>
                </li>
                <li
                  onClick={() => {
                    (document.activeElement as HTMLElement)?.blur();
                  }}
                >
                  <NavLink to="/random">
                    <MdShuffle size={18} />
                    {t("Random")}
                  </NavLink>
                </li>
                <li
                  onClick={() => {
                    (document.activeElement as HTMLElement)?.blur();
                  }}
                >
                  <NavLink to="/about">
                    <MdInfoOutline size={18} />
                    {t("About")}
                  </NavLink>
                </li>
              </ul>
            </div>
            <div>
              <NavLink
                to="/"
                replace
                className="btn btn-ghost text-xl"
              >
                {appName}
              </NavLink>
            </div>
            <div className="hidden lg:flex">
              <ul className="menu menu-horizontal px-1">
                <li>
                  <NavLink to="/tags">
                    <MdOutlineLabel size={18} />
                    {t("Tags")}
                  </NavLink>
                </li>
                <li>
                  <NavLink to="/random">
                    <MdShuffle size={18} />
                    {t("Random")}
                  </NavLink>
                </li>
                <li>
                  <NavLink to="/activity">
                    <MdTimeline size={18} />
                    {t("Activity")}
                  </NavLink>
                </li>
                <li>
                  <NavLink to="/about">
                    <MdInfoOutline size={18} />
                    {t("About")}
                  </NavLink>
                </li>
              </ul>
            </div>
            <div className={"flex-1"}></div>
            <div className="flex gap-2">
              <SearchBar />
              <ThemeSwitcher />
              <UploadingButton />
              <PublishButton />
              <UserButton />
            </div>
          </div>
        </div>
      </div>
  )
}

function FloatingToTopButton() {
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    let lastScrollY = window.scrollY;

    const handleScroll = () => {
      const isScrollingUp = window.scrollY < lastScrollY;
      const isAboveThreshold = window.scrollY > 200;

      setVisible(isScrollingUp && isAboveThreshold);
      lastScrollY = window.scrollY;
    };

    window.addEventListener("scroll", handleScroll);

    return () => {
      window.removeEventListener("scroll", handleScroll);
    };
  }, []);

  return (
    <button
      className={`btn btn-circle btn-soft btn-secondary border shadow-lg btn-lg fixed right-4 ${visible ? "bottom-4" : "-bottom-12"} transition-all z-50`}
      onClick={() => {
        window.scrollTo({ top: 0, behavior: "smooth" });
      }}
    >
      <MdArrowUpward size={20} />
    </button>
  );
}

function PublishButton() {
  const config = useConfig();
  const navigate = useNavigate();
  const { t } = useTranslation();
  if (!config.isLoggedIn) {
    return <></>
  }

  return (
    <>
      <button
        type="button"
        className="btn btn-primary hidden lg:flex"
        onClick={() => navigate("/publish?source=vndb")}
      >
        <MdOutlinePublish size={24} />
        <span>{t("Publish")}</span>
      </button>
      <button
        type="button"
        className="btn btn-primary btn-square lg:hidden"
        onClick={() => navigate("/publish?source=vndb")}
      >
        <MdOutlinePublish size={24} />
      </button>
    </>
  )
}

function UserButton() {
  const config = useConfig();
  const location = useLocation();
  const { t } = useTranslation();
  const [notificationCount, setNotificationCount] = useState(0);

  useEffect(() => {
    const fetchNotificationCount = async () => {
      if (!config.isLoggedIn) {
        return;
      }
      const res = await network.getUserNotificationsCount();
      if (res.success && res.data !== undefined) {
        setNotificationCount(res.data);
      }
    };

    fetchNotificationCount();
    const interval = setInterval(fetchNotificationCount, 60000); // 每分钟请求一次

    return () => clearInterval(interval);
  }, [config.isLoggedIn]);

  // Clear notification count when entering notifications page
  useEffect(() => {
    if (config.isLoggedIn) {
      if (location.pathname === '/notifications') {
        // Immediately clear the count when entering notifications page
        setNotificationCount(0);
      } else {
        // Fetch fresh count when on other pages
        const fetchCount = async () => {
          const res = await network.getUserNotificationsCount();
          if (res.success && res.data !== undefined) {
            setNotificationCount(res.data);
          }
        };
        fetchCount();
      }
    }
  }, [location.pathname, config.isLoggedIn]);

  if (!config.user) {
    return <NavLink
      to="/login"
      className="btn btn-primary btn-square btn-soft"
    >
      <MdOutlinePerson size={24} />
    </NavLink>
  }

  const handleLogout = async () => {
    const result = await network.logout();
    if (result.success) {
      // 退出成功，跳转到首页并刷新
      window.location.replace("/");
    }
  };

  return (
    <div className="dropdown dropdown-end">
      <div className="indicator">
        {notificationCount > 0 && (
          <span className="indicator-item w-2 h-2 bg-error rounded-full"></span>
        )}
        <div
          tabIndex={0}
          role="button"
          className="btn btn-square avatar overflow-clip"
          onMouseDown={(e) => {
            const target = e.currentTarget;
            // 如果已经有焦点（菜单已打开），则阻止默认行为并手动关闭
            if (document.activeElement === target) {
              e.preventDefault();
              target.blur();
            }
          }}
        >
          <img alt="Avatar" className="w-10 object-cover" src={network.getUserAvatar(config.user!)} />
        </div>
      </div>
      <ul
        tabIndex={0}
        className="menu menu-md dropdown-content bg-base-100 rounded-box z-50 mt-3 w-52 p-2 shadow"
      >
        <li
          onClick={() => {
            (document.activeElement as HTMLElement)?.blur();
          }}
        >
          <NavLink to={`/user/${encodeURIComponent(config.user!.username)}`}>
            <MdOutlinePerson size={18} />
            {t("My Profile")}
          </NavLink>
        </li>
        {config.user?.permission === Permission.Admin && (
          <li
            onClick={() => {
              (document.activeElement as HTMLElement)?.blur();
            }}
          >
            <NavLink to="/manage/tasks">
              <MdTimeline size={18} />
              {t("Server Tasks")}
            </NavLink>
          </li>
        )}
        <li
          onClick={() => {
            (document.activeElement as HTMLElement)?.blur();
          }}
        >
          <NavLink to="/notifications">
            <MdNotifications size={18} />
            {t("Notifications")}
            {notificationCount > 0 && (
              <span className="badge badge-error badge-sm ml-auto">
                {notificationCount > 99 ? "99+" : notificationCount}
              </span>
            )}
          </NavLink>
        </li>
        <li
          onClick={() => {
            (document.activeElement as HTMLElement)?.blur();
          }}
        >
          <NavLink to="/manage/me">
            <MdOutlineSettings size={18} />
            {t("Settings")}
          </NavLink>
        </li>
        <li
          onClick={() => {
            (document.activeElement as HTMLElement)?.blur();
            handleLogout();
          }}
        >
          <a>
            <MdLogout size={18} />
            {t("Log out")}
          </a>
        </li>
      </ul>
    </div>
  )
}

function UploadingButton() {
  const [hasTasks, setHasTasks] = useState(uploadingManager.hasTasks());
  const [panelOpen, setPanelOpen] = useState(false);
  const { t } = useTranslation();

  useEffect(() => {
    const update = () => setHasTasks(uploadingManager.hasTasks());
    uploadingManager.addListener(update);
    return () => uploadingManager.removeListener(update);
  }, []);

  useEffect(() => {
    if (!hasTasks) {
      setPanelOpen(false);
    }
  }, [hasTasks]);

  if (!hasTasks) return null;

  return (
    <>
      <button
        type="button"
        className="btn btn-square btn-soft"
        title={t("Uploading")}
        onClick={() => setPanelOpen(true)}
      >
        <MdCloudUpload size={24} className="animate-pulse" />
      </button>
      {panelOpen && (
        <UploadingPanel onClose={() => setPanelOpen(false)} />
      )}
    </>
  );
}

function UploadingPanel({ onClose }: { onClose: () => void }) {
  const [tasks, setTasks] = useState(() => uploadingManager.getTasks());
  const { t } = useTranslation();

  useEffect(() => {
    const update = () => setTasks(uploadingManager.getTasks());
    uploadingManager.addListener(update);
    return () => uploadingManager.removeListener(update);
  }, []);

  return (
    <>
      <div
        className="fixed inset-0 z-40 bg-black/30"
        onClick={onClose}
      />
      <div className="fixed right-0 top-0 h-full w-80 bg-base-100 shadow-2xl z-50 flex flex-col">
        <div className="flex items-center justify-between p-4 border-b border-base-200">
          <div className="flex items-center gap-2">
            <MdCloudUpload size={20} className="animate-pulse text-primary" />
            <h2 className="text-lg font-bold">{t("Uploading")}</h2>
          </div>
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-circle"
            onClick={onClose}
          >
            <MdClose size={18} />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto p-4 flex flex-col gap-3">
          {tasks.length === 0 ? (
            <p className="text-sm text-base-content/50 text-center mt-8">
              {t("No uploading tasks")}
            </p>
          ) : (
            tasks.map((task) => (
              <UploadingTaskTile key={task.id} task={task} />
            ))
          )}
        </div>
      </div>
    </>
  );
}

function UploadingTaskTile({ task }: { task: UploadingTask }) {
  const [, forceUpdate] = useState(0);
  const { t } = useTranslation();

  useEffect(() => {
    const update = () => forceUpdate((n) => n + 1);
    task.addListener(update);
    return () => task.removeListener(update);
  }, [task]);

  const progressPercent = Math.round(task.progress * 100);

  const statusLabel = () => {
    if (task.status === UploadingStatus.UPLOADING) return `${progressPercent}%`;
    if (task.status === UploadingStatus.DONE) return t("Done");
    if (task.status === UploadingStatus.ERROR) return task.errorMessage || t("Error");
    return t("Pending");
  };

  const progressClass = () => {
    if (task.status === UploadingStatus.ERROR) return "progress progress-error w-full";
    if (task.status === UploadingStatus.DONE) return "progress progress-success w-full";
    return "progress progress-primary w-full";
  };

  return (
    <div className="card bg-base-200 p-3 gap-2">
      <div className="text-sm font-medium truncate" title={task.filename}>
        {task.filename}
      </div>
      <progress
        className={progressClass()}
        value={task.progress}
        max={1}
      />
      <div className="flex items-center justify-between">
        <span className="text-xs text-base-content/60">{statusLabel()}</span>
        {task.status === UploadingStatus.UPLOADING && (
          <button
            type="button"
            className="btn btn-xs btn-ghost text-error"
            onClick={() => task.cancel()}
          >
            {t("Cancel")}
          </button>
        )}
      </div>
    </div>
  );
}

function SearchBar() {
  const navigate = useNavigate();
  const { t } = useTranslation();

  return (
    <button
      type="button"
      className="btn btn-ghost btn-circle"
      onClick={() => navigate("/search")}
      aria-label={t("Search")}
    >
      <MdSearch size={24} />
    </button>
  );
}
