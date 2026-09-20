import { network } from "~/network/network";
import type { Route } from "./+types/resource.$id";
import { configFromMatches, useConfig, isAdmin, canUpload } from "~/hook/config";
import { Children, createContext, createRef, isValidElement, useCallback, useContext, useEffect, useMemo, useRef, useState, type ComponentPropsWithoutRef, type ReactElement, type ReactNode } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { MdAdd, MdOutlineAccessTime, MdOutlineAdd, MdOutlineArchive, MdOutlineArticle, MdOutlineCloud, MdOutlineComment, MdOutlineContentCopy, MdOutlineDataset, MdOutlineDelete, MdOutlineDownload, MdOutlineEdit, MdOutlineFolderSpecial, MdOutlineInfo, MdOutlineKeyboardArrowDown, MdOutlineLink, MdOutlineOpenInNew, MdOutlineStar, MdOutlineSwapHoriz, MdOutlineVerifiedUser } from "react-icons/md";
import { useTranslation } from "~/hook/i18n";
import { NavLink, useNavigate } from "react-router";
import type { CharacterParams, Collection, RelationView, Resource, ResourceDetails, RFile, RLink, Tag, Storage as RStorage, Comment as RComment } from "~/network/models";
import Badge from "~/components/badge";
import { BiLogoSteam } from "react-icons/bi";
import showToast from "~/components/toast";
import { Debounce } from "~/utils/debounce";
import Button from "~/components/button";
import Loading from "~/components/loading";
import Gallery from "~/components/gallery";
import Markdown from "react-markdown";
import remarkDirective from "remark-directive";
import remarkGfm from "remark-gfm";
import { fileSizeToString } from "~/utils/file_size";
import showPopup, { useClosePopup } from "~/components/popup";
import { Turnstile } from "@marsidev/react-turnstile";
import { uploadingManager } from "~/network/uploading";
import { ErrorAlert } from "~/components/alert";
import Input, { TextArea } from "~/components/input";
import KunApi, { kunLanguageToString, kunPlatformToString, kunResourceTypeToString, type KunPatchResourceResponse, type KunPatchResponse } from "~/network/kun";
import { CommentTile } from "~/components/comment_tile";
import { CommentInput } from "~/components/comment_input";
import Pagination from "~/components/pagination";
import { useSetBackground } from "~/components/background";
import { ValidateHtml } from "~/utils/html";
import { removeMarkdown } from "~/utils/markdown";

export function meta({ loaderData, matches }: Route.MetaArgs) {
  const config = configFromMatches(matches);
  const resource = loaderData?.resource;
  if (!resource) {
    return [
      { title: config.server_name },
    ];
  }

  const title = resource.title;
  const plainText = removeMarkdown(resource.article).replace(/\s+/g, ' ').trim();
  const description = plainText.length > 160
    ? plainText.substring(0, 157) + '...'
    : plainText;
  let cover = null;
  if (resource.images != null && resource.images.length > 0) {
    cover = resource.images[0].id;
  }
  if (resource.coverId != null) {
    cover = resource.coverId;
  }

  const meta = [
    { title: title },
    { name: "description", content: description },
    { property: "og:title", content: title },
    { property: "og:description", content: description },
    { property: "og:type", content: "article" },
    { property: "og:url", content: typeof window !== "undefined" ? window.location.href : `${process.env.SERVER_BASE_URL}/resources/${resource.id}` },
    { property: "twitter:card", content: "summary_large_image" },
  ];
  if (cover != null) {
    let url = network.getResampledImageUrl(cover);
    if (url.startsWith("/")) {
      if (typeof window !== "undefined") {
        url = window.location.origin + url;
      } else {
        let serverBaseUrl = process.env.SERVER_BASE_URL!;
        url = serverBaseUrl + url
      }
    }
    meta.push({ property: "og:image", content: url });
  }
  return meta;
}

export async function loader({ params, request }: Route.LoaderArgs) {
  const id = parseInt(params.id);
  if (isNaN(id)) {
    throw new Error("Invalid resource ID");
  }
  const cookie = request.headers.get("Cookie");
  const res = await network.getResourceDetails(id, cookie || undefined);
  if (!res.success || !res.data) {
    throw new Error("Failed to load resource");
  }
  return { resource: res.data };
}

export default function ResourcePage({ loaderData }: Route.ComponentProps) {
  const { resource } = loaderData;
  const [page, setPage] = useState(0);
  const [visitedTabs, setVisitedTabs] = useState<Set<number>>(new Set([0]));
  const config = useConfig();
  const { t } = useTranslation();
  const navigate = useNavigate();
  // 标签页与hash的映射
  const tabHashList = ["description", "files", "comments"];
  // 读取hash对应的tab索引
  function getPageFromHash(hash: string) {
    const h = hash.replace(/^#/, "");
    const idx = tabHashList.indexOf(h);
    return idx === -1 ? 0 : idx;
  }
  // 设置hash
  function setHashByPage(idx: number) {
    window.location.hash = "#" + tabHashList[idx];
  }
  // 初始状态读取hash
  useEffect(() => {
    setPage(getPageFromHash(window.location.hash));
    setVisitedTabs(new Set([getPageFromHash(window.location.hash)]));
    // 监听hash变化
    const onHashChange = () => {
      setPage(getPageFromHash(window.location.hash));
    };
    window.addEventListener("hashchange", onHashChange);
    return () => window.removeEventListener("hashchange", onHashChange);
    // eslint-disable-next-line
  }, []);

  // 切换标签页时同步hash
  const handleTabChange = (idx: number) => {
    setPage(idx);
    setHashByPage(idx);
    // Mark tab as visited when switched to
    setVisitedTabs((prev) => new Set(prev).add(idx));
  };

  const setBackground = useSetBackground();
  useEffect(() => {
    let sfwImages = [];
    for (const image of resource.images ?? []) {
      if (resource.galleryNsfw?.includes(image.id) ?? false) {
        continue;
      }
      sfwImages.push(image.id);
    }
    console.log('SFW Images:', sfwImages);
    if (sfwImages.length > 0) {
      // random one
      const selectedImage = sfwImages[Math.floor(Math.random() * sfwImages.length)];
      setBackground(selectedImage);
    } else if (resource.coverId != null) {
      setBackground(resource.coverId);
    }
  }, [resource]);

  useEffect(() => {
	  network.addResourceView(resource.id);
  }, [resource.id]);

  return <div>
    <div className="flex bg-base-100/60 backdrop-blur-sm rounded-box mt-4 shadow mb-2 p-2">
      <div className="flex-1">
        <h1 className={"text-2xl font-bold px-4 py-2"}>{resource.title}</h1>
        {resource.alternativeTitles && resource.alternativeTitles.map((e, i) => {
          return (
            <h2
              key={i}
              className={
                "text-lg px-4 py-1 text-gray-700 dark:text-gray-300"
              }
            >
              {e}
            </h2>
          );
        })}
        {
          resource.releaseDate ? (
            <div className={"px-4 py-1 text-sm text-gray-600 dark:text-gray-400 flex items-center"}>
              <MdOutlineAccessTime size={18} className={"inline-block mr-1"} />
              {t("Release Date")}: {resource.releaseDate.split("T")[0]}
            </div>
          ) : null
        }
        <button
          onClick={() => {
            navigate(
              `/user/${encodeURIComponent(resource.author.username)}`,
            );
          }}
          className="border-b-2 mx-4 py-1 cursor-pointer border-transparent hover:border-primary transition-colors duration-200 ease-in-out"
        >
          <div className="flex items-center ">
            <div className="avatar">
              <div className="w-6 rounded-full">
                <img
                  src={network.getUserAvatar(resource.author)}
                  alt={"avatar"}
                />
              </div>
            </div>
            <div className="w-2"></div>
            <div className="text-sm">{resource.author.username}</div>
          </div>
        </button>
        <Tags tags={resource.tags} />
        <div className={"px-3 mt-2 flex flex-wrap"}>
          {resource.links &&
            resource.links.map((l, index) => {
              return <Link key={index} link={l} ratings={resource.ratings} />
            })}
          <CollectionDialog rid={resource.id} />
        </div>
      </div>
      <div className="w-96 md:w-md lg:w-lg xl:w-xl p-4 hidden sm:flex items-center justify-center">
        <Gallery images={resource.gallery} nsfw={resource.galleryNsfw} />
      </div>
    </div>

    <div className="w-full block sm:hidden">
      <Gallery images={resource.gallery} nsfw={resource.galleryNsfw} />
    </div>

    <div
      className="tabs tabs-box my-2 p-4 shadow rounded-box! bg-base-100/60 backdrop-blur-sm"
    >
      <label className="tab transition-all">
        <input
          type="radio"
          name="my_tabs"
          checked={page === 0}
          onChange={() => handleTabChange(0)}
        />
        <MdOutlineArticle className="text-xl mr-2" />
        <span className="text-sm">{t("Description")}</span>
      </label>
      <div key={"article"} className="tab-content p-2">
        {visitedTabs.has(0) && <Article resource={resource} />}
      </div>

      <label className="tab transition-all">
        <input
          type="radio"
          name="my_tabs"
          checked={page === 1}
          onChange={() => handleTabChange(1)}
        />
        <MdOutlineDataset className="text-xl mr-2" />
        <span className="text-sm">{t("Files")}</span>
      </label>
      <div key={"files"} className="tab-content p-2">
        {visitedTabs.has(1) && (
          <Files files={resource.files} resource={resource} />
        )}
      </div>

      <label className="tab transition-all">
        <input
          type="radio"
          name="my_tabs"
          checked={page === 2}
          onChange={() => handleTabChange(2)}
        />
        <MdOutlineComment className="text-xl mr-2" />
        <span className="text-sm">{t("Comments")}</span>
        {resource.comments ? (
          <span
            className={`px-1.5 py-0.5 ml-1 rounded-full text-xs ${page === 2 ? "bg-accent text-accent-content" : "text-base-content/60"}`}
          >
            {resource.comments}
          </span>
        ) : null}
      </label>
      <div key={"comments"} className="tab-content p-2">
        {visitedTabs.has(2) && <Comments resourceId={resource.id} />}
      </div>

      <div className={"grow"}></div>
      {isAdmin(config) || config.user?.id === resource.author.id ? (
        <Button
          className={"btn-ghost btn-circle"}
          onClick={() => {
            navigate(`/resource/edit/${resource.id}`, { replace: true });
          }}
        >
          <MdOutlineEdit size={20} />
        </Button>
      ) : null}
      <DeleteResourceDialog
        resourceId={resource.id}
        uploaderId={resource.author.id}
      />
    </div>
  </div>;
}

const unverifiedDownloadSizeLimit = 10 * 1024 * 1024;

function Tags({ tags }: { tags: Tag[] }) {
  const tagsMap = new Map<string, Tag[]>();

  const { t } = useTranslation();

  for (const tag of tags || []) {
    const type = tag.type;
    if (!tagsMap.has(type)) {
      tagsMap.set(type, []);
    }
    tagsMap.get(type)?.push(tag);
  }

  const compactMode = tags.length > 10;

  return (
    <>
      {Array.from(tagsMap.entries()).map(([type, tags]) => (
        <p key={type} className={"px-4"}>
          <Badge className="shadow-xs mr-0.5" key={type}>
            {type == "" ? t("Other") : type}
          </Badge>
          {tags.map((tag) => (
            <NavLink key={tag.name} to={`/tag/${encodeURIComponent(tag.name)}`}>
              <Badge
                className={
                  `${compactMode ? "m-0.5" : "m-1"} cursor-pointer badge-soft badge-primary shadow-xs`
                }
              >
                {tag.name}
              </Badge>
            </NavLink>
          ))}
        </p>
      ))}
    </>
  );
}

function Link({ link, ratings }: { link: RLink, ratings: Record<string, number> }) {
  return (
    <a href={link.url} target={"_blank"}>
      <span
        className={
          "py-1 px-3 inline-flex items-center m-1 border border-base-300 bg-base-100 opacity-90 rounded-2xl hover:bg-base-200 transition-colors cursor-pointer select-none"
        }
      >
        {link.url.includes("steampowered.com") ? (
          <BiLogoSteam size={20} />
        ) : (
          <MdOutlineLink size={20} />
        )}
        <span className={"ml-2 text-sm"}>{link.label}</span>
        {ratings[link.label] && ratings[link.label] > 0 ? (
          <>
            <MdOutlineStar size={16} className={"inline-block ml-2 mr-0.5 text-yellow-500 dark:text-yellow-400"} />
            <span className={"text-sm text-yellow-500 dark:text-yellow-400"}>{(ratings[link.label] / 10).toFixed(1)}</span>
          </>
        ) : null}
      </span>
    </a>
  );
}

function CollectionDialog({ rid }: { rid: number }) {
  const { t } = useTranslation();

  const [searchKeyword, setSearchKeyword] = useState("");

  const [realSearchKeyword, setRealSearchKeyword] = useState("");

  const [dialogVisited, setDialogVisited] = useState(false);

  const [selectedCID, setSelectedCID] = useState<number | null>(null);

  const debounce = new Debounce(500);

  const navigate = useNavigate();

  const config = useConfig();

  const delayedSetSearchKeyword = (keyword: string) => {
    setSearchKeyword(keyword);
    debounce.run(() => {
      setSelectedCID(null);
      setRealSearchKeyword(keyword);
    });
  };

  const handleAddToCollection = () => {
    if (selectedCID == null) {
      return;
    }
    network.addResourceToCollection(selectedCID, rid).then((res) => {
      if (res.success) {
        showToast({
          message: t("Resource added to collection successfully"),
          type: "success",
        });
        setSelectedCID(null);
        setRealSearchKeyword("");
        setSearchKeyword("");
        setDialogVisited(false);
        const dialog = document.getElementById(
          "collection_dialog",
        ) as HTMLDialogElement;
        dialog.close();
      } else {
        showToast({
          message: res.message,
          type: "error",
          parent: document.getElementById("collection_dialog_content"),
        });
      }
    });
  };

  if (!config.isLoggedIn) {
    return <></>;
  }

  return (
    <>
      <span
        className={
          "py-1 px-3 inline-flex items-center m-1 border border-base-300 bg-base-100 opacity-90 rounded-2xl hover:bg-base-200 transition-colors cursor-pointer select-none"
        }
        onClick={() => {
          setDialogVisited(true);
          const dialog = document.getElementById(
            "collection_dialog",
          ) as HTMLDialogElement;
          dialog.showModal();
        }}
      >
        <MdOutlineFolderSpecial size={20} />
        <span className={"ml-2 text-sm"}>{t("Collect")}</span>
      </span>
      <dialog id="collection_dialog" className="modal">
        <div className="modal-box" id="collection_dialog_content">
          <h3 className="font-bold text-lg mb-2">{t("Add to Collection")}</h3>
          <input
            type="text"
            placeholder="Search"
            className="input input-bordered w-full max-w-2xs mr-2"
            value={searchKeyword}
            onChange={(e) => delayedSetSearchKeyword(e.target.value)}
          />
          {dialogVisited && (
            <CollectionSelector
              resourceId={rid}
              keyword={realSearchKeyword}
              seletedID={selectedCID}
              selectCallback={(collection) => {
                if (selectedCID === collection.id) {
                  setSelectedCID(null);
                } else {
                  setSelectedCID(collection.id);
                }
              }}
              key={realSearchKeyword}
            />
          )}
          <div className="modal-action">
            <Button
              className="btn-ghost"
              onClick={() => {
                const dialog = document.getElementById(
                  "collection_dialog",
                ) as HTMLDialogElement;
                dialog.close();
                navigate("/create-collection");
              }}
            >
              <div className="flex items-center">
                <MdOutlineAdd size={20} className={"inline-block mr-1"} />
                {t("Create")}
              </div>
            </Button>
            <span className="flex-1"></span>
            <form method="dialog">
              <Button className="btn">{t("Cancel")}</Button>
            </form>
            <Button
              className="btn-primary"
              disabled={selectedCID == null}
              onClick={handleAddToCollection}
            >
              {t("Add")}
            </Button>
          </div>
        </div>
      </dialog>
    </>
  );
}

function CollectionSelector({
  resourceId,
  keyword,
  seletedID: selectedID,
  selectCallback,
}: {
  resourceId: number;
  keyword: string;
  seletedID?: number | null;
  selectCallback: (collection: Collection) => void;
}) {
  const [collections, setCollections] = useState<Collection[] | null>(null);
  const config = useConfig();

  useEffect(() => {
    setCollections(null);
    network
      .searchUserCollections(config.user!.username, keyword, resourceId)
      .then((res) => {
        if (res.success) {
          setCollections(res.data! || []);
        } else {
          showToast({
            message: res.message,
            type: "error",
          });
        }
      });
  }, [keyword]);

  if (collections == null) {
    return (
      <div className={"w-full"}>
        <Loading />
      </div>
    );
  }

  return (
    <div className="py-2 max-h-80 overflow-y-auto w-full overflow-x-clip">
      {collections.map((collection) => {
        return (
          <div
            className={`${selectedID === collection.id && "bg-base-200 shadow"} rounded-lg transition-all p-2 hover:bg-base-200 w-full overflow-ellipsis hover:cursor-pointer`}
            key={collection.id}
            onClick={() => {
              selectCallback(collection);
            }}
          >
            <input
              type="checkbox"
              className="checkbox checkbox-primary mr-2"
              checked={selectedID === collection.id}
              readOnly
            />
            {collection.title}
          </div>
        );
      })}
    </div>
  );
}

function Article({ resource }: { resource: ResourceDetails }) {
  const article = useMemo<string>(() => normalizeArticle(resource.article), [resource.article])

  return (
    <>
      <article>
        <Markdown
          remarkPlugins={[remarkGfm, remarkDirective, remarkCollapseDirective, remarkTabViewDirective]}
          components={{
            details: CollapseDetails,
            div: ({ node, className, children, ...props }) => {
              if (className === "tab-view") {
                const labels: string[] = JSON.parse(
                  (props as any)["data-tab-labels"] || "[]",
                );
                return <TabView labels={labels}>{children}</TabView>;
              }
              if (className === "tab-panel") {
                return <div className="tab-panel">{children}</div>;
              }
              return (
                <div className={className} {...props}>
                  {children}
                </div>
              );
            },
            hr: () => <hr className="my-4 border-base-300" />,
            p: ({ node, ...props }) => {
              if (
                typeof props.children === "object" &&
                (props.children as ReactElement).type === "strong"
              ) {
                // @ts-ignore
                const children = (
                  props.children as ReactElement
                ).props.children;
                let plainChild = "";
                if (Array.isArray(children)) {
                  plainChild = children.map((c) => {
                    if (typeof c === "object" && c.type === "text") {
                      return c.props.children;
                    } else if (typeof c === "object" && c.props.href) {
                      return c.props.href;
                    }
                    return c.toString();
                  }).join("");
                } else {
                  plainChild = children.toString();
                }
                if (plainChild.startsWith("<iframe")) {
                  // @ts-ignore
                  let html = plainChild;
                  // remove width, height, class, style attributes
                  html = html.replace(/width="[^"]*"/g, "");
                  html = html.replace(/height="[^"]*"/g, "");
                  html = html.replace(/class="[^"]*"/g, "");
                  html = html.replace(/style="[^"]*"/g, "");
                  if (!ValidateHtml(html)) {
                    return <></>;
                  }
                  return (
                    <div
                      className={`w-full my-3 max-w-xl rounded-xl overflow-clip ${html.includes("youtube") ? "aspect-video" : "h-48 sm:h-64"}`}
                      dangerouslySetInnerHTML={{
                        __html: html,
                      }}
                    ></div>
                  );
                }
              } else if (
                typeof props.children === "object" &&
                // @ts-ignore
                props.children?.props &&
                // @ts-ignore
                props.children?.props.href
              ) {
                const a = props.children as ReactElement;
                const childProps = a.props as any;
                const href = childProps.href as string;
                // @ts-ignore
                if (childProps.children?.length === 2) {
                  // @ts-ignore
                  const first = childProps.children[0] as ReactNode;
                  // @ts-ignore
                  const second = childProps.children[1] as ReactNode;

                  if (
                    typeof first === "object" &&
                    (typeof second === "string" || typeof second === "object")
                  ) {
                    const img = first as ReactElement;
                    // @ts-ignore
                    if (img.type === "img") {
                      return (
                        <a
                          className={
                            "inline-block card shadow bg-base-100 no-underline hover:shadow-md transition-shadow my-2"
                          }
                          target={"_blank"}
                          href={href}
                        >
                          <figure className={"max-h-96 min-w-48 min-h-24"}>
                            {img}
                          </figure>
                          <div className={"card-body text-base-content text-lg"}>
                            <div className={"flex items-center"}>
                              <span className={"flex-1"}>{second}</span>
                              <span>
                                <MdOutlineOpenInNew size={24} />
                              </span>
                            </div>
                          </div>
                        </a>
                      );
                    }
                  }
                }
                if (href.startsWith("https://store.steampowered.com/app/")) {
                  const appId = href
                    .substring("https://store.steampowered.com/app/".length)
                    .split("/")[0];
                  if (!Number.isNaN(Number(appId))) {
                    return (
                      <div className={"max-w-xl h-52 sm:h-48 my-2"}>
                        <iframe
                          className={"scheme-light"}
                          src={`https://store.steampowered.com/widget/${appId}/`}
                        ></iframe>
                      </div>
                    );
                  }
                }
              }
              return <p {...props}>{props.children}</p>;
            },
            a: ({ node, ...props }) => {
              const href = props.href as string;
              const origin = typeof window !== "undefined" ? window.location.origin : process.env.SERVER_BASE_URL!;

              if (href.startsWith(origin) || href.startsWith("/")) {
                let path = href;
                if (path.startsWith(origin)) {
                  path = path.substring(origin.length);
                }
                const content = props.children?.toString();
                if (path.startsWith("/resources/")) {
                  const id = path.substring("/resources/".length);
                  for (const r of resource.related ?? []) {
                    if (r.id.toString() === id) {
                      return <RelatedResourceCard r={r} content={content} />;
                    }
                  }
                }
              }

              return <a target={"_blank"} {...props}></a>;
            },
          }}
        >
          {article}
        </Markdown>
      </article>
      {resource.characters.length > 0 && <div className="divider" />}
      <Characters characters={resource.characters} tags={resource.tags} />
      {resource.relations.length > 0 && <div className="divider" />}
      <ResourceRelations relations={resource.relations} />
    </>
  );
}

function TabView({
  labels,
  children,
}: {
  labels: string[];
  children: React.ReactNode;
}) {
  const [activeTab, setActiveTab] = useState(0);
  const panels = Children.toArray(children).filter(
    (child) =>
      isValidElement(child) &&
      (child.props as any).className === "tab-panel",
  );

  const isInCollapse = useContext(collapseContext);

  return (
    <div className={`${isInCollapse ? "" : "my-4 overflow-hidden rounded-xl border border-base-300 bg-base-100/70 shadow-sm"}`}>
      <div role="tablist" className={`tabs tabs-bordered ${isInCollapse ? "" : "px-4 pt-2"}`}>
        {labels.map((label, i) => (
          <button
            key={i}
            type="button"
            role="tab"
            aria-selected={activeTab === i}
            className={`tab${
              activeTab === i ? " tab-active font-semibold" : ""
            }`}
            onClick={() => setActiveTab(i)}
          >
            {label}
          </button>
        ))}
      </div>
      {isInCollapse && <div className="divider my-0 px-2"></div>}
      <div className={`${isInCollapse ? "px-2" : "px-4 pb-4 pt-2"}`}>{panels[activeTab]}</div>
    </div>
  );
}

type CollapseDetailsProps = ComponentPropsWithoutRef<"details"> & {
  "data-collapse-label"?: string;
};

const collapseContext = createContext(false);

function CollapseDetails({ children, className, open, id, title, ...props }: CollapseDetailsProps) {
  const [isOpen, setIsOpen] = useState(Boolean(open));
  const summary = props["data-collapse-label"];
  const label = Array.isArray(summary) ? summary.join("") : summary?.toString() || "Details";

  return (
    <collapseContext.Provider value={true}>
      <div
      id={id}
      title={title}
      className={`my-4 overflow-hidden rounded-xl border border-base-300 bg-base-100/70 shadow-sm ${className || ""}`.trim()}
    >
      <button
        type="button"
        className="flex w-full items-center gap-2 px-4 py-3 text-left font-medium transition-colors duration-200 hover:bg-base-200/60"
        aria-expanded={isOpen}
        onClick={() => setIsOpen((current) => !current)}
      >
        <motion.span
          initial={false}
          animate={{ rotate: isOpen ? 180 : 0 }}
          transition={{ duration: 0.2, ease: "easeInOut" }}
          className="shrink-0"
        >
          <MdOutlineKeyboardArrowDown size={20} />
        </motion.span>
        <span>{label}</span>
      </button>

      <AnimatePresence initial={false}>
        {isOpen ? (
          <motion.div
            key="content"
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: "auto", opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.24, ease: "easeInOut" }}
            className="overflow-hidden"
          >
            <div className="px-4 pb-4 pt-1">{children}</div>
          </motion.div>
        ) : null}
      </AnimatePresence>
    </div>
    </collapseContext.Provider>
  );
}

function normalizeArticle(article: string) {
  article = normalizeCollapseDirectiveSyntax(article);
  const lines = article.split("\n");
  let result = "";
  let lastIsRelatedResource = false;
  const lineSeparator = "    \n";

  const isResourceLine = (text: string): boolean => {
    let currentHost = "";
    if (typeof window !== "undefined") {
      currentHost = window.location.host;
    } else {
      // 这里的 process.env 处理通常需要确保是完整的协议+主机名，否则 URL 构造函数会报错
      currentHost = process.env.SERVER_BASE_URL || "http://localhost";
    }
    const trimmed = text.trim();
    const match = trimmed.match(/^\[.*\]\((.*)\)$/);
    if (!match) return false;

    try {
      const urlContent = match[1];
      // 如果 urlContent 是相对路径，需要 base 包含协议
      const base = currentHost.startsWith('http') ? currentHost : `https://${currentHost}`;
      const url = new URL(urlContent, base);
      return url.host === (new URL(base).host) && url.pathname.startsWith("/resources/");
    } catch (e) {
      return false;
    }
  };

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i].trimEnd();
    const isResource = isResourceLine(line);
    const isEmpty = line.trim() === "";

    if (isResource) {
      if (lastIsRelatedResource) {
        result += " " + line.trim();
      } else {
        result += (result ? lineSeparator : "") + line.trim();
      }
      lastIsRelatedResource = true;
    } else if (isEmpty && lastIsRelatedResource) {
      continue; 
    } else {
      if (result && !result.endsWith(lineSeparator)) {
        result += lineSeparator;
      }
      result += line + lineSeparator;
      lastIsRelatedResource = false;
    }
  }

  return result.trimEnd();
}

function normalizeCollapseDirectiveSyntax(article: string): string {
  const lines = article.split("\n");
  const normalizedLines: string[] = [];
  let fence: string | null = null;
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];
    const trimmed = line.trimStart();
    const fenceMatch = trimmed.match(/^(```+|~~~+)/);

    if (fenceMatch) {
      const marker = fenceMatch[1][0];
      if (fence == null) fence = marker;
      else if (fence === marker) fence = null;
      normalizedLines.push(line);
      i++;
      continue;
    }

    if (fence == null) {
      const collapseMatch = line.match(/^(\s*):::collapse(\+?)\s+(.+?)\s*$/);
      if (collapseMatch) {
        const isOpen = collapseMatch[2] === "+";
        const collapseLabel = collapseMatch[3];
        i++;
        // Collect body until matching :::
        const bodyLines: string[] = [];
        let colDepth = 1;
        let colFence: string | null = null;
        while (i < lines.length && colDepth > 0) {
          const bl = lines[i];
          const bt = bl.trimStart();
          const bfm = bt.match(/^(```+|~~~+)/);
          if (bfm) {
            const m = bfm[1][0];
            if (colFence == null) colFence = m;
            else if (colFence === m) colFence = null;
          }
          if (colFence == null) {
            if (/^:{3,}[a-z_]/.test(bt)) colDepth++;
            else if (bt === ":::") {
              colDepth--;
              if (colDepth === 0) { i++; break; }
            }
          }
          bodyLines.push(bl);
          i++;
        }
        // Recursively normalize the body
        const normalizedBody = normalizeCollapseDirectiveSyntax(bodyLines.join("\n"));
        // Use 5 colons so collapse can safely contain tab_view (4 colons) and tab_panel (3 colons)
        const attrs = isOpen ? "{open}" : "";
        normalizedLines.push(`${collapseMatch[1]}:::::collapse[${collapseLabel}]${attrs}`);
        normalizedLines.push(normalizedBody);
        normalizedLines.push(":::::");
        continue;
      }

      const tabViewMatch = line.match(/^(\s*):::tab_view\s+(.+?)\s*$/);
      if (tabViewMatch) {
        i++;
        // Collect lines until the matching closing :::
        const blockLines: string[] = [];
        let depth = 1;
        let innerFence: string | null = null;
        while (i < lines.length && depth > 0) {
          const bl = lines[i];
          const bt = bl.trimStart();
          const bfm = bt.match(/^(```+|~~~+)/);
          if (bfm) {
            const m = bfm[1][0];
            if (innerFence == null) innerFence = m;
            else if (innerFence === m) innerFence = null;
          }
          if (innerFence == null) {
            if (/^:{3,}[a-z_]/.test(bt)) depth++;
            else if (bt === ":::") {
              depth--;
              if (depth === 0) { i++; break; }
            }
          }
          blockLines.push(bl);
          i++;
        }

        // Split blockLines by --- at top level (outside fences and nested directives)
        const sections: string[][] = [[]];
        let sf: string | null = null;
        let sd = 0;
        for (const bl of blockLines) {
          const bt = bl.trimStart();
          const bfm = bt.match(/^(```+|~~~+)/);
          if (bfm) {
            const m = bfm[1][0];
            if (sf == null) sf = m; else if (sf === m) sf = null;
          }
          if (sf == null) {
            if (/^:{3,}[a-z_]/.test(bt)) sd++;
            else if (bt === ":::") sd--;
          }
          if (sf == null && sd === 0 && bl.trim() === "---") {
            sections.push([]);
          } else {
            sections[sections.length - 1].push(bl);
          }
        }

        // Emit: ::::tab_view[labels] (outer, more colons) + :::tab_panel sections (inner, fewer colons) + ::::
        normalizedLines.push(`${tabViewMatch[1]}::::tab_view[${tabViewMatch[2]}]`);
        for (const section of sections) {
          normalizedLines.push(":::tab_panel");
          // Recursively normalize collapse/tab_view inside each section
          normalizedLines.push(normalizeCollapseDirectiveSyntax(section.join("\n")));
          normalizedLines.push(":::");
        }
        normalizedLines.push("::::");
        continue;
      }
    }

    normalizedLines.push(line);
    i++;
  }

  return normalizedLines.join("\n");
}

function remarkTabViewDirective() {
  return (tree: unknown) => {
    walkMarkdownTree(tree, (node) => {
      if (node.type !== "containerDirective" || node.name !== "tab_view") {
        return;
      }

      const labelNode = Array.isArray(node.children)
        ? node.children.find((child: any) => child?.data?.directiveLabel)
        : undefined;
      const labelText = extractTextContent(labelNode).trim();
      const tabLabels = labelText ? labelText.split("/") : [];

      const data = node.data || (node.data = {});
      data.hName = "div";
      data.hProperties = {
        className: "tab-view",
        "data-tab-labels": JSON.stringify(tabLabels),
      };

      // Remove label node and transform tab_panel children
      const panelChildren = Array.isArray(node.children)
        ? node.children.filter((child: any) => child !== labelNode)
        : [];

      for (const child of panelChildren) {
        if (child.type === "containerDirective" && child.name === "tab_panel") {
          const childData = child.data || (child.data = {});
          childData.hName = "div";
          childData.hProperties = { className: "tab-panel" };
        }
      }

      node.children = panelChildren;
    });
  };
}

function remarkCollapseDirective() {
  return (tree: unknown) => {
    walkMarkdownTree(tree, (node) => {
      if (node.type !== "containerDirective" || node.name !== "collapse") {
        return;
      }

      const labelNode = Array.isArray(node.children)
        ? node.children.find((child: any) => child?.data?.directiveLabel)
        : undefined;
      const label = extractTextContent(labelNode).trim() || "Details";

      const isOpen = "open" in (node.attributes || {});

      const data = node.data || (node.data = {});
      data.hName = "details";
      data.hProperties = {
        ...(data.hProperties || {}),
        "data-collapse-label": label,
        ...(isOpen ? { open: true } : {}),
      };

      if (labelNode && Array.isArray(node.children)) {
        node.children = node.children.filter((child: any) => child !== labelNode);
      }
    });
  };
}

function walkMarkdownTree(node: unknown, visitor: (node: any) => void) {
  if (!node || typeof node !== "object") {
    return;
  }

  visitor(node);

  if (!("children" in node) || !Array.isArray(node.children)) {
    return;
  }

  for (const child of node.children) {
    walkMarkdownTree(child, visitor);
  }
}

function extractTextContent(node: any): string {
  if (!node || typeof node !== "object") {
    return "";
  }

  if (typeof node.value === "string") {
    return node.value;
  }

  if (!Array.isArray(node.children)) {
    return "";
  }

  return node.children.map((child: any) => extractTextContent(child)).join("");
}

function RelatedResourceCard({
  r,
  content,
}: {
  r: Resource;
  content?: string;
}) {
  return (
    <NavLink
      to={"/resources/" + r.id}
      className={
        "inline-block shadow hover:shadow-md transition-shadow relative h-52 w-full max-w-md rounded-sm overflow-clip"
      }
    >
      {r.image != null && (
          <img
              src={network.getResampledImageUrl(r.image.id)}
              alt="cover"
              className="w-full h-full object-cover object-top"
              style={{
                borderRadius: 0,
              }}
            />
        )}
        <span className="block absolute bottom-0 left-0 right-0 p-2 bg-linear-to-t from-black/70 to-transparent">
          <span className="text-lg font-bold line-clamp-2 overflow-hidden text-white">{r.title}</span>
          {content && <span className="text-xs line-clamp-1 overflow-hidden text-white">{content}</span>}
        </span>
    </NavLink>
  );
}

function ResourceRelations({ relations }: { relations?: RelationView[] }) {
  const { t } = useTranslation();

  if (!relations || relations.length === 0) {
    return null;
  }

  return (
    <div className="mt-4">
      <h3 className="text-xl font-bold mb-4">{t("Related Resources")}</h3>
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 gap-4">
        {relations.map((rel) => (
          <RelatedResourceCard key={rel.resource.id} r={rel.resource} content={rel.description || undefined} />
        ))}
      </div>
    </div>
  );
}

function Characters({
  characters,
  tags,
}: {
  characters: CharacterParams[];
  tags: Tag[];
}) {
  const { t } = useTranslation();

  let main = characters.filter((c) => c.role === "primary");
  let other1 = characters.filter((c) => c.role !== "primary" && c.image);
  let other2 = characters.filter((c) => c.role !== "primary" && !c.image);
  characters = [...main, ...other1, ...other2];

  if (!characters || characters.length === 0) {
    return <></>;
  }

  return (
    <div className="mt-4">
      <h3 className="text-xl font-bold mb-4">{t("Characters")}</h3>
      <div className="grid grid-cols-2 xs:grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 gap-2">
        {characters.map((character, index) => (
          <CharacterCard key={index} character={character} tags={tags} />
        ))}
      </div>
    </div>
  );
}

function CharacterCard({
  character,
  tags,
}: {
  character: CharacterParams;
  tags: Tag[];
}) {
  const navigate = useNavigate();
  const cvTag = character.cv?.trim();
  const matchedTag = cvTag
    ? tags.find((tag) => tag.name === cvTag)
    : undefined;

  const handleCVClick = (e: React.MouseEvent) => {
    e.stopPropagation();
    if (cvTag) {
      if (matchedTag) {
        navigate(`/tag/${encodeURIComponent(matchedTag.name)}`);
        return;
      }
      navigate(`/search?keyword=${encodeURIComponent(cvTag)}`);
    }
  };

  return (
    <div className="group relative aspect-3/4 overflow-hidden rounded-lg bg-base-200 shadow-sm">
      <img
        src={character.image ? network.getImageUrl(character.image) : "/cp.webp"}
        alt={character.name}
        className="w-full h-full object-cover object-top"
      />

      <div className="absolute bottom-1 left-1 right-1 px-1 py-1 border border-base-100/40 rounded-lg bg-base-100/60">
        <h4 className="font-semibold text-xs sm:text-sm leading-tight line-clamp border border-transparent">
          {character.name}
          {
            character.role === "primary" ? (
              <span className="bg-primary/80 rounded-lg px-2 py-0.5 text-primary-content ml-1" style={{
                fontSize: "10px",
              }}>
                Main
              </span>
            ) : null
          }
        </h4>

        {character.cv && (
          <button
            onClick={handleCVClick}
            className="hover:bg-base-200/80 border border-transparent hover:border-base-300/50 rounded-sm text-xs transition-colors cursor-pointer"
          >
            CV: {character.cv}
          </button>
        )}
      </div>
    </div>
  );
}

function Files({
  files,
  resource,
}: {
  files: RFile[];
  resource: ResourceDetails;
}) {
  const { t } = useTranslation();
  const [selectedTags, setSelectedTags] = useState<Set<string>>(new Set());
  const config = useConfig();
  // Extract unique tags from all files
  const allTags = useMemo(() => {
    const tags = new Set<string>();
    files.forEach((file) => {
      if (file.tag) {
        tags.add(file.tag);
      } 
    });
    return Array.from(tags).sort();
  }, [files]);

  // Filter files based on selected tags
  const filteredFiles = useMemo(() => {
    if (selectedTags.size === 0) {
      return files;
    }
    return files.filter((file) => file.tag && selectedTags.has(file.tag));
  }, [files, selectedTags]);

  const toggleTag = (tag: string) => {
    setSelectedTags((prev) => {
      const newSet = new Set(prev);
      if (newSet.has(tag)) {
        newSet.delete(tag);
      } else {
        newSet.add(tag);
      }
      return newSet;
    });
  };

  return (
    <div className={"pt-3"}>
      {config.support_email_address && (
        <div className="alert alert-info alert-soft mb-4">
          <MdOutlineInfo size={18} className={"inline-block"} />
          <span>
            {t("All files are uploaded by users, if you find any issue, please contact us via email:")}
            <a className="text-primary" href={`mailto:${config.support_email_address}`}>{config.support_email_address}</a>
          </span>
        </div>
      )}
      {allTags.length > 0 && (
        <form className="filter mb-4">
          {allTags.map((tag) => (
            <input
              key={tag}
              className="btn"
              type="checkbox"
              aria-label={tag}
              checked={selectedTags.has(tag)}
              onChange={() => toggleTag(tag)}
            />
          ))}
          {selectedTags.size > 0 && (
            <input
              className="btn btn-square"
              type="reset"
              value="×"
              onClick={() => setSelectedTags(new Set())}
            />
          )}
        </form>
      )}
      {filteredFiles.map((file) => {
        return <FileTile file={file} key={file.id}></FileTile>;
      })}
      {filteredFiles.length === 0 && selectedTags.size > 0 && (
        <div className="text-center text-base-content/60 py-8">
          {t("No files match the selected tags")}
        </div>
      )}
      <div className={"h-2"}></div>
      {(canUpload(config) || (config.allow_normal_user_upload && config.isLoggedIn)) && (
        <div className={"flex flex-row-reverse"}>
          <CreateFileDialog resourceId={resource.id}></CreateFileDialog>
        </div>
      )}
      <KunFiles resource={resource} />
    </div>
  );
}

enum FileType {
  redirect = "redirect",
  upload = "upload",
  serverTask = "server_task",
}

function FileTile({ file }: { file: RFile }) {
  const buttonRef = createRef<HTMLButtonElement>();
  const buttonRef2 = createRef<HTMLButtonElement>();

  const { t } = useTranslation();

  const userLink = `/user/${encodeURIComponent(file.user.username)}`;

  const navigate = useNavigate();

  return (
    <div className={"card shadow bg-base-100/80 mb-4 p-4"}>
      <div className={"flex flex-row items-center"}>
        <div className={"grow"}>
          <h4 className={"font-bold break-all"}>{file.filename}</h4>
          <div className={"text-sm my-1 comment_tile"}>
            <Markdown>{file.description.replaceAll("\n", "  \n")}</Markdown>
          </div>
          <p className={"items-center mt-1"}>
            <a
              href={userLink}
              onClick={(e) => {
                e.preventDefault();
                navigate(userLink);
              }}
            >
              <Badge
                className={
                  "badge-soft badge-primary text-xs mr-2 hover:shadow-xs transition-shadow"
                }
              >
                <img
                  src={network.getUserAvatar(file.user)}
                  className={"w-4 h-4 rounded-full"}
                  alt={"avatar"}
                />
                {file.user.username}
              </Badge>
            </a>
            {
              file.is_redirect && (
                <Badge className={"badge-soft badge-secondary text-xs mr-2"}>
                  <MdOutlineOpenInNew size={16} className={"inline-block"} />
                  {t("Redirect")}
                </Badge>
              )
            }
            {
              file.size > 0 && (
                <Badge className={"badge-soft badge-secondary text-xs mr-2"}>
                  <MdOutlineArchive size={16} className={"inline-block"} />
                  {fileSizeToString(file.size)}
                </Badge>
              )
            }
            {file.hash && (
              <>
                <Badge
                  className={
                    "badge-soft badge-accent text-xs mr-2 break-all hover:shadow-xs cursor-pointer transition-shadow"
                  }
                  onClick={() => {
                    const dialog = document.getElementById(
                      `file_md5_${file.id}`,
                    ) as HTMLDialogElement;
                    dialog.showModal();
                  }}
                >
                  <MdOutlineVerifiedUser size={16} className={"inline-block"} />
                  Md5
                </Badge>
                <dialog id={`file_md5_${file.id}`} className="modal">
                  <div className="modal-box">
                    <h3 className="font-bold text-lg mb-4">Md5</h3>
                    <label className="input input-primary w-full">
                      <input type="text" readOnly value={file.hash} />
                      <button
                        className="btn btn-square btn-ghost btn-sm"
                        onClick={() => {
                          navigator.clipboard.writeText(file.hash!);
                        }}
                      >
                        <MdOutlineContentCopy size={18} />
                      </button>
                    </label>
                    <div className="modal-action">
                      <form method="dialog">
                        <button className="btn">Close</button>
                      </form>
                    </div>
                  </div>
                </dialog>
              </>
            )}
            {file.storage_name && (
              <Badge className={"badge-soft badge-info text-xs mr-2"}>
                <MdOutlineCloud size={16} className={"inline-block"} />
                {file.storage_name}
              </Badge>
            )}
            {file.tag && (
              <Badge className={"badge-soft badge-warning text-xs mr-2"}>
                {file.tag}
              </Badge>
            )}
            <Badge className={"badge-soft badge-info text-xs mr-2"}>
              <MdOutlineAccessTime size={16} className={"inline-block"} />
              {new Date(file.created_at * 1000).toISOString().substring(0, 10)}
            </Badge>
            <DeleteFileDialog fileId={file.id} uploaderId={file.user.id} />
            <UpdateFileInfoDialog file={file} />
            <MigrateFileDialog file={file} />
          </p>
        </div>
        <div className={`flex-row items-center hidden xs:flex`}>
          {!file.is_redirect ? (
            <button
              ref={buttonRef}
              className={"btn btn-primary btn-soft btn-square"}
              onClick={() => {
                if (!import.meta.env.CLOUDFLARE_TURNSTILE_SITE_KEY || file.size <= unverifiedDownloadSizeLimit) {
                  const link = network.getFileDownloadLink(file.id, "");
                  window.open(link, "_blank");
                } else {
                  showPopup(
                    <CloudflarePopup file={file} t={t} />,
                    buttonRef.current!,
                  );
                }
              }}
            >
              <MdOutlineDownload size={24} />
            </button>
          ) : (
            <a
              href={network.getFileDownloadLink(file.id, "")}
              target="_blank"
              className={"btn btn-primary btn-soft btn-square"}
            >
              <MdOutlineDownload size={24} />
            </a>
          )}
        </div>
      </div>
      <div className="flex flex-row-reverse xs:hidden p-2">
        {!file.is_redirect ? (
          <button
            ref={buttonRef2}
            className={"btn btn-primary btn-soft btn-sm"}
            onClick={() => {
              if (!import.meta.env.CLOUDFLARE_TURNSTILE_SITE_KEY || file.size <= unverifiedDownloadSizeLimit) {
                const link = network.getFileDownloadLink(file.id, "");
                window.open(link, "_blank");
              } else {
                showPopup(<CloudflarePopup file={file} t={t} />, buttonRef2.current!);
              }
            }}
          >
            <MdOutlineDownload size={20} />
          </button>
        ) : (
          <a
            href={network.getFileDownloadLink(file.id, "")}
            target="_blank"
            className={"btn btn-primary btn-soft btn-sm"}
          >
            <MdOutlineDownload size={24} />
          </a>
        )}
      </div>
    </div>
  );
}

function CloudflarePopup({ file, t }: { file: RFile, t: (key: string) => string }) {
  const closePopup = useClosePopup();

  const [isLoading, setLoading] = useState(true);
  const [downloadToken, setDownloadToken] = useState<string | null>(null);
  const [cfToken, setCfToken] = useState<string | null>(null);
  const [showTurnstile, setShowTurnstile] = useState(false);

  useEffect(() => {
    let active = true;

    async function requestDownloadToken() {
      const res = await network.createFileDownloadToken(file.id);
      if (!active) {
        return;
      }
      if (res.success && res.data?.token) {
        setDownloadToken(res.data.token);
        setLoading(false);
        return;
      }
      setShowTurnstile(true);
      setLoading(false);
    }

    void requestDownloadToken();

    return () => {
      active = false;
    };
  }, [file.id]);

  const href = downloadToken
    ? network.getFileDownloadLink(file.id, "", downloadToken)
    : cfToken
      ? network.getFileDownloadLink(file.id, cfToken)
      : null;

  return (
    <div
      className={"menu bg-base-100 rounded-box z-1 w-80 p-2 shadow-sm relative"}
    >
      {isLoading ? (
        <div
          className={
            "absolute top-0 bottom-8 left-0 right-0 flex items-center justify-center"
          }
        >
          <span className={"loading loading-spinner loading-lg"}></span>
        </div>
      ) : null}
      <h3 className={"font-bold m-2"}>
        {href ? t("Verification successful") : t("Verifying your request")}
      </h3>
      {
        downloadToken == null && (
          <div className={"h-20 w-full"}>
            {showTurnstile ? (
              <Turnstile
                siteKey={import.meta.env.CLOUDFLARE_TURNSTILE_SITE_KEY!}
                onSuccess={(token) => {
                  setCfToken(token);
                }}
              ></Turnstile>
            ) : null}
          </div>
        )
      }
      {href ? (
        <div className="p-2">
          <a
            href={href}
            target="_blank"
            className="btn btn-primary btn-sm w-full"
            onClick={() => {
              closePopup();
            }}
          >
            <MdOutlineDownload size={20} />
            {t("Download")}
          </a>
        </div>
      ) : showTurnstile ? <p className={"text-xs text-base-content/80 m-2"}>
        {t(
          "Please check your network if the verification takes too long or the captcha does not appear.",
        )}
      </p> : null}
    </div>
  );
}


function CreateFileDialog({ resourceId }: { resourceId: number }) {
  const { t } = useTranslation();
  const [isLoading, setLoading] = useState(false);
  const storages = useRef<RStorage[] | null>(null);
  const mounted = useRef(true);
  const config = useConfig();

  const [fileType, setFileType] = useState<FileType | null>(null);

  const [filename, setFilename] = useState<string>("");
  const [redirectUrl, setRedirectUrl] = useState<string>("");
  const [storage, setStorage] = useState<RStorage | null>(null);
  const [file, setFile] = useState<File | null>(null);
  const [description, setDescription] = useState<string>("");
  const [tag, setTag] = useState<string>("");
  const [fileSize, setFileSize] = useState<string>("");
  const [fileSizeUnit, setFileSizeUnit] = useState<string>("MB");
  const [md5, setMd5] = useState<string>("");

  const [fileUrl, setFileUrl] = useState<string>("");

  const reload = function () {
    window.location.reload();
  }

  const [isSubmitting, setSubmitting] = useState(false);

  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  const submit = async () => {
    if (isSubmitting) {
      return;
    }
    if (!fileType) {
      setError(t("Please select a file type"));
      return;
    }
    setSubmitting(true);
    if (fileType === FileType.redirect) {
      if (!redirectUrl || !filename || !description) {
        setError(t("Please fill in all fields"));
        setSubmitting(false);
        return;
      }
      let fileSizeNum = 0;
      if (fileSize) {
        const size = parseFloat(fileSize);
        if (isNaN(size)) {
          setError(t("File size must be a number"));
          setSubmitting(false);
          return;
        }
        // Convert to bytes based on unit
        switch (fileSizeUnit) {
          case "B":
            fileSizeNum = size;
            break;
          case "KB":
            fileSizeNum = size * 1024;
            break;
          case "MB":
            fileSizeNum = size * 1024 * 1024;
            break;
          case "GB":
            fileSizeNum = size * 1024 * 1024 * 1024;
            break;
        }
      }
      const res = await network.createRedirectFile(
        filename,
        description,
        resourceId,
        redirectUrl,
        fileSizeNum,
        md5,
        tag,
      );
      if (res.success) {
        setSubmitting(false);
        const dialog = document.getElementById(
          "upload_dialog",
        ) as HTMLDialogElement;
        dialog.close();
        showToast({ message: t("File created successfully"), type: "success" });
        reload();
      } else {
        setError(res.message);
        setSubmitting(false);
      }
    } else if (fileType === FileType.upload) {
      if (!file || !storage) {
        setError(t("Please select a file and storage"));
        setSubmitting(false);
        return;
      }
      const res = await uploadingManager.addTask(
        file,
        resourceId,
        storage.id,
        description,
        tag,
        () => {
          if (mounted.current) {
            reload();
          }
        },
      );
      if (res.success) {
        setSubmitting(false);
        const dialog = document.getElementById(
          "upload_dialog",
        ) as HTMLDialogElement;
        dialog.close();
        showToast({
          message: t("Successfully create uploading task."),
          type: "success",
        });
      } else {
        setError(res.message);
        setSubmitting(false);
      }
    } else if (fileType === FileType.serverTask) {
      if (!fileUrl || !filename || !storage) {
        setError(t("Please fill in all fields"));
        setSubmitting(false);
        return;
      }
      const res = await network.createServerDownloadTask(
        fileUrl,
        filename,
        description,
        resourceId,
        storage.id,
        tag,
      );
      if (res.success) {
        setSubmitting(false);
        const dialog = document.getElementById(
          "upload_dialog",
        ) as HTMLDialogElement;
        dialog.close();
        showToast({ message: t("File created successfully"), type: "success" });
        reload();
      } else {
        setError(res.message);
        setSubmitting(false);
      }
    }
  };

  return (
    <>
      <button
        className={"btn btn-accent shadow"}
        onClick={() => {
          if (isLoading) {
            return;
          }
          if (storages.current == null) {
            setLoading(true);
            network.listStorages().then((res) => {
              if (!mounted.current) {
                return;
              }
              if (!res.success) {
                showToast({ message: res.message, type: "error" });
              } else {
                storages.current = res.data!;
                let defaultStorage = storages.current.find((s) => s.isDefault);
                if (!defaultStorage && storages.current.length > 0) {
                  defaultStorage = storages.current[0];
                }
                console.log("defaultStorage", defaultStorage);
                setStorage(defaultStorage || null);
                setLoading(false);
                const dialog = document.getElementById(
                  "upload_dialog",
                ) as HTMLDialogElement;
                dialog.showModal();
              }
            });
            return;
          }
          const dialog = document.getElementById(
            "upload_dialog",
          ) as HTMLDialogElement;
          dialog.showModal();
        }}
      >
        {isLoading ? (
          <span className={"loading loading-spinner loading-sm"}></span>
        ) : (
          <MdAdd size={24} />
        )}
        <span className={"text-sm"}>{t("Upload")}</span>
      </button>
      <dialog id="upload_dialog" className="modal">
        <div className="modal-box">
          <h3 className="font-bold text-lg mb-2">{t("Create File")}</h3>

          {config.upload_prompt && (
            <p className={"text-sm p-2"}>{config.upload_prompt}</p>
          )}

          <p className={"text-sm font-bold p-2"}>{t("Type")}</p>
          <form className="filter mb-2">
            <input
              className="btn"
              type="radio"
              name="type"
              aria-label={t("Redirect")}
              onInput={() => {
                setFileType(FileType.redirect);
              }}
            />
            <input
              className="btn"
              type="radio"
              name="type"
              aria-label={t("Upload")}
              onInput={() => {
                setFileType(FileType.upload);
              }}
            />
            <input
              className="btn"
              type="radio"
              name="type"
              aria-label={t("File Url")}
              onInput={() => {
                setFileType(FileType.serverTask);
              }}
            />
            <input
              className="btn btn-square"
              type="reset"
              value="×"
              onClick={() => {
                setFileType(null);
              }}
            />
          </form>

          {fileType === FileType.redirect && (
            <>
              <p className={"text-sm p-2"}>
                {t("User who click the file will be redirected to the URL")}
              </p>
              <input
                type="text"
                className="input w-full my-2"
                placeholder={t("File Name")}
                onChange={(e) => {
                  setFilename(e.target.value);
                }}
              />
              <input
                type="text"
                className="input w-full my-2"
                placeholder={t("URL")}
                onChange={(e) => {
                  setRedirectUrl(e.target.value);
                }}
              />
              <textarea
                className="textarea w-full my-2"
                placeholder={t("Description" + " (Markdown)")}
                onChange={(e) => {
                  setDescription(e.target.value);
                }}
              />
              <input
                type="text"
                className="input w-full my-2"
                placeholder={t("Tag") + " (" + t("Optional") + ")"}
                onChange={(e) => {
                  setTag(e.target.value);
                }}
              />
              <div className="join w-full">
                <input
                  type="number"
                  className="input flex-1 join-item"
                  placeholder={t("File Size") + " (" + t("Optional") + ")"}
                  value={fileSize}
                  onChange={(e) => {
                    setFileSize(e.target.value);
                  }}
                />
                <select
                  className="select w-24 join-item"
                  value={fileSizeUnit}
                  onChange={(e) => {
                    setFileSizeUnit(e.target.value);
                  }}
                >
                  <option value="B">B</option>
                  <option value="KB">KB</option>
                  <option value="MB">MB</option>
                  <option value="GB">GB</option>
                </select>
              </div>
              <input
                type="text"
                className="input w-full my-2"
                placeholder={"MD5" + " (" + t("Optional") + ")"}
                onChange={(e) => {
                  setMd5(e.target.value);
                }}
              />
            </>
          )}

          {fileType === FileType.upload && (
            <>
              <p className={"text-sm p-2"}>
                {t(
                  "Upload a file to server, then the file will be moved to the selected storage.",
                )}
              </p>
              <select
                disabled={!canUpload(config)} // normal user cannot choose storage
                className="select select-primary w-full my-2"
                value={storage?.id || ""}
                onChange={(e) => {
                  const id = parseInt(e.target.value);
                  if (isNaN(id)) {
                    setStorage(null);
                  } else {
                    const s = storages.current?.find((s) => s.id == id);
                    if (s) {
                      setStorage(s);
                    }
                  }
                }}
              >
                <option value={""} disabled>
                  {t("Select Storage")}
                </option>
                {storages.current?.map((s) => {
                  return (
                    <option key={s.id} value={s.id}>
                      {s.name}({(s.currentSize / 1024 / 1024).toFixed(2)}/
                      {s.maxSize / 1024 / 1024}MB)
                    </option>
                  );
                })}
              </select>

              <input
                type="file"
                className="file-input w-full my-2"
                onChange={(e) => {
                  if (e.target.files) {
                    setFile(e.target.files[0]);
                  }
                }}
              />

              <textarea
                className="textarea w-full my-2"
                placeholder={t("Description" + " (Markdown)")}
                onChange={(e) => {
                  setDescription(e.target.value);
                }}
              />
              <input
                type="text"
                className="input w-full my-2"
                placeholder={t("Tag") + " (" + t("Optional") + ")"}
                onChange={(e) => {
                  setTag(e.target.value);
                }}
              />
            </>
          )}

          {fileType === FileType.serverTask && !canUpload(config) && (
            <p className={"text-sm p-2"}>
              {t(
                "You do not have permission to upload files, please contact the administrator.",
              )}
            </p>
          )}

          {fileType === FileType.serverTask && canUpload(config) && (
            <>
              <p className={"text-sm p-2"}>
                {t(
                  "Provide a file url for the server to download, and the file will be moved to the selected storage.",
                )}
              </p>
              <select
                disabled={!canUpload(config)}
                className="select select-primary w-full my-2"
                value={storage?.id || ""}
                onChange={(e) => {
                  const id = parseInt(e.target.value);
                  if (isNaN(id)) {
                    setStorage(null);
                  } else {
                    const s = storages.current?.find((s) => s.id == id);
                    if (s) {
                      setStorage(s);
                    }
                  }
                }}
              >
                <option value={""} disabled>
                  {t("Select Storage")}
                </option>
                {storages.current?.map((s) => {
                  return (
                    <option key={s.id} value={s.id}>
                      {s.name}({(s.currentSize / 1024 / 1024).toFixed(2)}/
                      {s.maxSize / 1024 / 1024}MB)
                    </option>
                  );
                })}
              </select>

              <input
                type="text"
                className="input w-full my-2"
                placeholder={t("File Name")}
                onChange={(e) => {
                  setFilename(e.target.value);
                }}
              />

              <input
                type="text"
                className="input w-full my-2"
                placeholder={t("File URL")}
                onChange={(e) => {
                  setFileUrl(e.target.value);
                }}
              />

              <textarea
                className="textarea w-full my-2"
                placeholder={t("Description" + " (Markdown)")}
                onChange={(e) => {
                  setDescription(e.target.value);
                }}
              />
              <input
                type="text"
                className="input w-full my-2"
                placeholder={t("Tag") + " (" + t("Optional") + ")"}
                onChange={(e) => {
                  setTag(e.target.value);
                }}
              />
            </>
          )}

          {error && <ErrorAlert className={"my-2"} message={error} />}

          <div className="modal-action">
            <form method="dialog">
              <button className="btn text-sm">{t("Cancel")}</button>
            </form>
            <button className={"btn btn-primary text-sm"} onClick={submit}>
              {isSubmitting ? (
                <span className={"loading loading-spinner loading-sm"}></span>
              ) : null}
              {t("Submit")}
            </button>
          </div>
        </div>
      </dialog>
    </>
  );
}

function MigrateFileDialog({ file }: { file: RFile }) {
  const [isLoading, setLoading] = useState(false);
  const [storages, setStorages] = useState<RStorage[] | null>(null);
  const [selectedStorageID, setSelectedStorageID] = useState<number | null>(null);
  const config = useConfig();
  const { t } = useTranslation();

  const id = `migrate_file_dialog_${file.id}`;

  const reload = function () {
    window.location.reload();
  }

  const openDialog = async () => {
    if (isLoading) {
      return;
    }
    if (storages == null) {
      setLoading(true);
      const res = await network.listStorages();
      setLoading(false);
      if (!res.success || !res.data) {
        showToast({ message: res.message, type: "error" });
        return;
      }
      setStorages(res.data);
      let defaultStorage = res.data.find((s) => s.isDefault);
      if (!defaultStorage && res.data.length > 0) {
        defaultStorage = res.data[0];
      }
      setSelectedStorageID(defaultStorage?.id ?? null);
    }
    const dialog = document.getElementById(id) as HTMLDialogElement;
    dialog.showModal();
  };

  const handleMigrate = async () => {
    if (isLoading || selectedStorageID == null) {
      return;
    }
    setLoading(true);
    const res = await network.createFileMigrationTask(file.id, selectedStorageID);
    setLoading(false);
    if (!res.success) {
      showToast({ message: res.message, type: "error" });
      return;
    }
    const dialog = document.getElementById(id) as HTMLDialogElement;
    dialog.close();
    showToast({ message: t("Migration task created successfully"), type: "success" });
    reload();
  };

  if (file.is_redirect) {
    return <></>;
  }
  if (!isAdmin(config) && config.user?.id !== file.user.id) {
    return <></>;
  }

  return (
    <>
      <button
        className={"btn btn-info btn-ghost btn-circle btn-sm ml-1"}
        onClick={openDialog}
      >
        <MdOutlineSwapHoriz size={16} className={"inline-block"} />
      </button>
      <dialog id={id} className="modal">
        <div className="modal-box">
          <h3 className="font-bold text-lg">{t("Migrate File")}</h3>
          <p className="py-2 text-sm">
            {t("Move this file from current storage to another storage asynchronously.")}
          </p>
          <select
            className="select select-primary w-full my-2"
            value={selectedStorageID ?? ""}
            onChange={(e) => {
              const value = parseInt(e.target.value);
              if (isNaN(value)) {
                setSelectedStorageID(null);
              } else {
                setSelectedStorageID(value);
              }
            }}
          >
            <option value={""} disabled>
              {t("Select Storage")}
            </option>
            {storages?.map((s) => {
              return (
                <option key={s.id} value={s.id}>
                  {s.name}({(s.currentSize / 1024 / 1024).toFixed(2)}/{s.maxSize / 1024 / 1024}MB)
                </option>
              );
            })}
          </select>

          <div className="modal-action">
            <form method="dialog">
              <button className="btn btn-ghost">{t("Close")}</button>
            </form>
            <button className="btn btn-info" onClick={handleMigrate}>
              {isLoading ? <span className={"loading loading-spinner loading-sm"}></span> : null}
              {t("Start Migration")}
            </button>
          </div>
        </div>
      </dialog>
    </>
  );
}

function UpdateFileInfoDialog({ file }: { file: RFile }) {
  const [isLoading, setLoading] = useState(false);

  const [filename, setFilename] = useState(file.filename);

  const [description, setDescription] = useState(file.description);

  const [tag, setTag] = useState(file.tag || "");

  const { t } = useTranslation();

  const config = useConfig();

  const reload = function () {
    window.location.reload();
  }

  const handleUpdate = async () => {
    if (isLoading) {
      return;
    }
    setLoading(true);
    const res = await network.updateFile(file.id, filename, description, tag);
    const dialog = document.getElementById(
      `update_file_info_dialog_${file.id}`,
    ) as HTMLDialogElement;
    dialog.close();
    if (res.success) {
      showToast({
        message: t("File info updated successfully"),
        type: "success",
      });
      reload();
    } else {
      showToast({ message: res.message, type: "error" });
    }
    setLoading(false);
  };

  if (!isAdmin(config) && config.user?.id !== file.user.id) {
    return <></>;
  }

  return (
    <>
      <button
        className={"btn btn-primary btn-ghost btn-circle btn-sm ml-1"}
        onClick={() => {
          const dialog = document.getElementById(
            `update_file_info_dialog_${file.id}`,
          ) as HTMLDialogElement;
          dialog.showModal();
        }}
      >
        <MdOutlineEdit size={16} className={"inline-block"} />
      </button>
      <dialog id={`update_file_info_dialog_${file.id}`} className="modal">
        <div className="modal-box">
          <h3 className="font-bold text-lg">{t("Update File Info")}</h3>
          <Input
            type={"text"}
            label={t("File Name")}
            value={filename}
            onChange={(e) => setFilename(e.target.value)}
          />
          <TextArea
            label={t("Description")}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
          />
          <Input
            type={"text"}
            label={t("Tag") + " (" + t("Optional") + ")"}
            value={tag}
            onChange={(e) => setTag(e.target.value)}
          />
          <div className="modal-action">
            <form method="dialog">
              <button className="btn btn-ghost">{t("Close")}</button>
            </form>
            <button className="btn btn-primary" onClick={handleUpdate}>
              {t("Update")}
            </button>
          </div>
        </div>
      </dialog>
    </>
  );
}

function DeleteFileDialog({
  fileId,
  uploaderId,
}: {
  fileId: string;
  uploaderId: number;
}) {
  const [isLoading, setLoading] = useState(false);
  const config = useConfig();

  const id = `delete_file_dialog_${fileId}`;

  const reload = function () {
    window.location.reload();
  }

  const { t } = useTranslation();

  const handleDelete = async () => {
    if (isLoading) {
      return;
    }
    setLoading(true);
    const res = await network.deleteFile(fileId);
    const dialog = document.getElementById(id) as HTMLDialogElement;
    dialog.close();
    if (res.success) {
      showToast({ message: t("File deleted successfully"), type: "success" });
      reload();
    } else {
      showToast({ message: res.message, type: "error" });
    }
    setLoading(false);
  };

  if (!isAdmin(config) && config.user?.id !== uploaderId) {
    return <></>;
  }

  return (
    <>
      <button
        className={"btn btn-error btn-ghost btn-circle btn-sm ml-1"}
        onClick={() => {
          const dialog = document.getElementById(id) as HTMLDialogElement;
          dialog.showModal();
        }}
      >
        <MdOutlineDelete size={16} className={"inline-block"} />
      </button>
      <dialog id={id} className="modal">
        <div className="modal-box">
          <h3 className="font-bold text-lg">{t("Delete File")}</h3>
          <p className="py-4">
            {t(
              "Are you sure you want to delete the file? This action cannot be undone.",
            )}
          </p>
          <div className="modal-action">
            <form method="dialog">
              <button className="btn btn-ghost">{t("Close")}</button>
            </form>
            <button className="btn btn-error" onClick={handleDelete}>
              {t("Delete")}
            </button>
          </div>
        </div>
      </dialog>
    </>
  );
}

function KunFiles({ resource }: { resource: ResourceDetails }) {
  let vnid = "";
  for (const link of resource.links ?? []) {
    if (link.label.toLowerCase() === "vndb") {
      vnid = (link.url.split("/").pop() || "").split(/[?#]/)[0];
      break;
    }
  }

  const [data, setData] = useState<KunPatchResponse | null>(null);

  const [isLoading, setLoading] = useState<boolean>(true);

  const [error, setError] = useState<string | null>(null);

  const { t } = useTranslation();

  useEffect(() => {
    if (!vnid || !KunApi.isAvailable()) {
      setLoading(false);
      return;
    }
    KunApi.getPatch(vnid).then((res) => {
      if (res.success) {
        setData(res.data!);
      } else if (res.message === "404") {
        // ignore
      } else {
        setError(res.message);
      }
      setLoading(false);
    });
  }, [vnid]);

  if (error) {
    return <ErrorAlert className={"my-2"} message={error} />;
  }

  if (isLoading) {
    return <Loading />;
  }

  if (!vnid || !KunApi.isAvailable() || data === null) {
    return <></>;
  }

  return (
    <>
      <div className="mx-2 my-4 flex">
        <a href={data.web_url || "https://www.moyu.moe"} target="_blank">
          <div className="border-b-2 pb-1 border-transparent hover:border-primary select-none cursor-pointer transition-all flex items-center gap-2">
            <img src="/kun.webp" className="h-8 w-8 rounded-full" />
            <span className="text-xl font-bold">鲲补丁</span>
          </div>
        </a>
      </div>
      {data && (
        <div className={"flex flex-col gap-2"}>
          {(data.resources ?? []).map((file) => {
            return <KunFile file={file} key={file.id} />;
          })}
          {(data.resources ?? []).length === 0 && (
            <p className={"text-sm text-base-content/80"}>
              {t("No patches found for this VN.")}
            </p>
          )}
        </div>
      )}
    </>
  );
}

function KunFile({
  file,
}: {
  file: KunPatchResourceResponse;
}) {
  const tags: string[] = [];
  if (file.model_name) {
    tags.push(file.model_name);
  }
  if (file.localization_group_name) {
    tags.push(file.localization_group_name);
  }
  tags.push(...(file.platform ?? []).map((p) => kunPlatformToString(p)));
  tags.push(...(file.language ?? []).map((l) => kunLanguageToString(l)));
  tags.push(...(file.type ?? []).map((t) => kunResourceTypeToString(t)));

  return (
    <div className={"card shadow bg-base-100/80 mb-4"}>
      <div className={"p-4 flex flex-row items-center"}>
          <div className={"grow min-w-0"}>
          <h4 className={"font-bold break-all"}>{file.name}</h4>
          <div className={"text-sm my-1 comment_tile"}>
            <Markdown>{(file.note ?? "").replaceAll("\n", "  \n")}</Markdown>
          </div>
          <p className={"items-center mt-1"}>
            {file.publisher && (
              <Badge
                className={
                  "badge-soft badge-primary text-xs mr-2 hover:shadow-xs transition-shadow"
                }
              >
                {file.publisher.avatar_url && (
                  <img
                    src={file.publisher.avatar_url}
                    className={"w-4 h-4 rounded-full"}
                    alt={"avatar"}
                  />
                )}
                {file.publisher.name}
              </Badge>
            )}
            {file.size && (
              <Badge className={"badge-soft badge-secondary text-xs mr-2"}>
                <MdOutlineArchive size={16} className={"inline-block"} />
                {file.size}
              </Badge>
            )}
            {tags.map((p, index) => (
              <Badge className={"badge-soft badge-info text-xs mr-2"} key={`${p}-${index}`}>
                {p}
              </Badge>
            ))}
          </p>
        </div>
        {file.web_url && (
          <div className={"flex flex-row items-center"}>
            <a
              href={file.web_url}
              target="_blank"
              className={"btn btn-primary btn-soft btn-square"}
            >
              <MdOutlineOpenInNew size={24} />
            </a>
          </div>
        )}
      </div>
    </div>
  );
}

function DeleteResourceDialog({
  resourceId,
  uploaderId,
}: {
  resourceId: number;
  uploaderId?: number;
}) {
  const [isLoading, setLoading] = useState(false);

  const { t } = useTranslation();

  const config = useConfig();

  const handleDelete = async () => {
    if (isLoading) {
      return;
    }
    setLoading(true);
    const res = await network.deleteResource(resourceId);
    const dialog = document.getElementById(
      "delete_resource_dialog",
    ) as HTMLDialogElement;
    dialog.close();
    if (res.success) {
      showToast({
        message: t("Resource deleted successfully"),
        type: "success",
      });
      window.location.href = "/";
    } else {
      showToast({ message: res.message, type: "error" });
    }
    setLoading(false);
  };

  if (!isAdmin(config) && config.user?.id !== uploaderId) {
    return <></>;
  }

  return (
    <>
      <Button
        className={"btn-error btn-ghost btn-circle"}
        onClick={() => {
          const dialog = document.getElementById(
            "delete_resource_dialog",
          ) as HTMLDialogElement;
          dialog.showModal();
        }}
      >
        <MdOutlineDelete size={20} className={"inline-block"} />
      </Button>
      <dialog id={`delete_resource_dialog`} className="modal">
        <div className="modal-box">
          <h3 className="font-bold text-lg">{t("Delete Resource")}</h3>
          <p className="py-4">
            {t("Are you sure you want to delete the resource")}?{" "}
            {t("This action cannot be undone.")}
          </p>
          <div className="modal-action">
            <form method="dialog">
              <button className="btn btn-ghost">{t("Close")}</button>
            </form>
            <Button
              className="btn btn-error"
              isLoading={isLoading}
              onClick={handleDelete}
            >
              {t("Delete")}
            </Button>
          </div>
        </div>
      </dialog>
    </>
  );
}

function Comments({ resourceId }: { resourceId: number }) {
  const [page, setPage] = useState(1);
  const [maxPage, setMaxPage] = useState(0);
  const [listKey, setListKey] = useState(0);

  const reload = useCallback(() => {
    setPage(1);
    setMaxPage(0);
    setListKey((prev) => prev + 1);
  }, []);

  return (
    <div>
      <CommentInput resourceId={resourceId} reload={reload} />
      <CommentsList
        resourceId={resourceId}
        page={page}
        maxPageCallback={setMaxPage}
        reload={reload}
        key={listKey}
      />
      {maxPage ? (
        <div className={"w-full flex justify-center"}>
          <Pagination page={page} setPage={setPage} totalPages={maxPage} />
        </div>
      ) : null}
    </div>
  );
}

function CommentsList({
  resourceId,
  page,
  maxPageCallback,
  reload,
}: {
  resourceId: number;
  page: number;
  maxPageCallback: (maxPage: number) => void;
  reload: () => void;
}) {
  const [comments, setComments] = useState<RComment[] | null>(null);

  useEffect(() => {
    network.listResourceComments(resourceId, page).then((res) => {
      if (res.success) {
        setComments(res.data!);
        maxPageCallback(res.totalPages || 1);
      } else {
        showToast({
          message: res.message,
          type: "error",
        });
      }
    });
  }, [maxPageCallback, page, resourceId]);

  if (comments == null) {
    return (
      <div className={"w-full"}>
        <Loading />
      </div>
    );
  }

  return (
    <>
      {comments.map((comment) => {
        return (
          <CommentTile comment={comment} key={comment.id} onUpdated={reload} />
        );
      })}
    </>
  );
}