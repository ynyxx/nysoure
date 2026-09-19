import { useEffect, useRef, useState } from "react";
import {
  MdAdd,
  MdClose,
  MdContentCopy,
  MdDelete,
  MdOutlineInfo,
} from "react-icons/md";
import type { CharacterParams, Resource, Tag, VndbResourcePrefill } from "../network/models";
import { network } from "../network/network";
import { useTranslation } from "../hook/i18n";
import { Debounce } from "../utils/debounce";
import { ErrorAlert } from "./alert";
import TagInput, { QuickAddTagDialog } from "./tag_input";
import TagTemplatePanel from "./tag_template";
import {
  ImageDropArea,
  SelectAndUploadImageButton,
  UploadClipboardImageButton,
} from "./image_selector";
import CharacterEditor, { FetchVndbCharactersButton } from "./character_editor";
import { parseVndbId } from "../utils/vndb";

const ALL_SECTIONS = [
  "basic",
  "article",
  "tags_basic",
  "tags_staff",
  "tags_content",
  "images",
  "characters",
] as const;
type PrefillSection = (typeof ALL_SECTIONS)[number];

const TAG_SECTIONS: PrefillSection[] = ["tags_basic", "tags_staff", "tags_content"];

const SECTION_LABELS: Record<PrefillSection, string> = {
  basic: "Basic Info",
  article: "Article",
  tags_basic: "Basic Tags",
  tags_staff: "Personnel Tags",
  tags_content: "Content Tags",
  images: "Images",
  characters: "Characters",
};

export interface RelationFormItem {
  toId: number;
  toTitle: string;
  description: string;
}

export interface ResourceFormData {
  title: string;
  altTitles: string[];
  releaseDate?: string;
  skipUpdateTime?: boolean;
  tags: Tag[];
  article: string;
  images: number[];
  coverId?: number;
  links: { label: string; url: string }[];
  galleryImages: number[];
  galleryNsfw: number[];
  characters: CharacterParams[];
  relations: RelationFormItem[];
}

interface ResourceFormProps {
  initialData: ResourceFormData;
  onSubmit: (data: ResourceFormData) => Promise<void>;
  submitButtonText: string;
  title: string;
  storageKey?: string;
  canUploadCheck?: boolean;
  excludeId?: number;
  showSkipUpdateTimeOption?: boolean;
}

export default function ResourceForm({
  initialData,
  onSubmit,
  submitButtonText,
  title: pageTitle,
  storageKey,
  canUploadCheck = false,
  excludeId,
  showSkipUpdateTimeOption = false,
}: ResourceFormProps) {
  const [title, setTitle] = useState<string>(initialData.title);
  const [altTitles, setAltTitles] = useState<string[]>(initialData.altTitles);
  const [releaseDate, setReleaseDate] = useState<string | undefined>(initialData.releaseDate);
  const [skipUpdateTime, setSkipUpdateTime] = useState<boolean>(initialData.skipUpdateTime ?? false);
  const [tags, setTags] = useState<Tag[]>(initialData.tags);
  const [article, setArticle] = useState<string>(initialData.article);
  const [images, setImages] = useState<number[]>(initialData.images);
  const [coverId, setCoverId] = useState<number | undefined>(initialData.coverId);
  const [links, setLinks] = useState<{ label: string; url: string }[]>(initialData.links);
  const [galleryImages, setGalleryImages] = useState<number[]>(initialData.galleryImages);
  const [galleryNsfw, setGalleryNsfw] = useState<number[]>(initialData.galleryNsfw);
  const [characters, setCharacters] = useState<CharacterParams[]>(initialData.characters);
  const [relations, setRelations] = useState<RelationFormItem[]>(initialData.relations ?? []);
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setSubmitting] = useState(false);
  const articleRef = useRef<HTMLTextAreaElement>(null);
  const imageListRef = useRef<HTMLDivElement>(null);
  const previousImageCountRef = useRef(initialData.images.length);

  // VNDB import state
  const [vnID, setVNID] = useState("");
  const [importError, setImportError] = useState<string | null>(null);
  const [isImporting, setImporting] = useState(false);
  const [selectedSections, setSelectedSections] = useState<PrefillSection[]>([...ALL_SECTIONS]);

  const { t } = useTranslation();

  const toggleSection = (section: PrefillSection) => {
    setSelectedSections((prev) =>
      prev.includes(section) ? prev.filter((s) => s !== section) : [...prev, section],
    );
  };

  const handleImportFromVNDB = async () => {
    const parsedVnID = parseVndbId(vnID);
    if (!parsedVnID) {
      setImportError(t("Invalid VNDB ID format"));
      return;
    }
    setImportError(null);
    setImporting(true);
    const res = await network.getResourcePrefillFromVNDB(parsedVnID, selectedSections);
    setImporting(false);
    if (!res.success || !res.data) {
      setImportError(res.message || t("Failed to fetch resource params from VNDB"));
      return;
    }
    const data: VndbResourcePrefill = res.data;
    if (selectedSections.includes("basic")) {
      setTitle(data.title || "");
      setAltTitles(data.alternative_titles || []);
      setReleaseDate(data.release_date || undefined);
      setLinks(data.links || []);
    }
    if (selectedSections.includes("article")) {
      setArticle(data.article || "");
    }
    if (TAG_SECTIONS.some((section) => selectedSections.includes(section))) {
      setTags((prev) => {
        const merged = [...prev];
        for (const newTag of data.tags || []) {
          if (!merged.find((t) => t.id === newTag.id)) {
            merged.push(newTag);
          }
        }
        return merged;
      });
    }
    if (selectedSections.includes("images")) {
      setImages(data.images || []);
      setCoverId(data.cover_id ?? undefined);
    }
    if (selectedSections.includes("characters")) {
      setCharacters(data.characters || []);
    }
  };

  // Auto-save to localStorage if storageKey is provided
  useEffect(() => {
    if (!storageKey || typeof localStorage === "undefined") return;

    const data = {
      title,
      alternative_titles: altTitles,
      tags,
      article,
      images,
      cover_id: coverId,
      links,
      gallery: galleryImages,
      gallery_nsfw: galleryNsfw,
      characters,
      release_date: releaseDate,
      relations,
    };
    const dataString = JSON.stringify(data);
    localStorage.setItem(storageKey, dataString);
  }, [
    altTitles,
    article,
    images,
    coverId,
    tags,
    title,
    links,
    galleryImages,
    galleryNsfw,
    characters,
    releaseDate,
    relations,
    storageKey,
  ]);

  useEffect(() => {
    const previousImageCount = previousImageCountRef.current;
    if (images.length > previousImageCount) {
      imageListRef.current?.scrollTo({
        top: imageListRef.current.scrollHeight,
        behavior: "smooth",
      });
    }
    previousImageCountRef.current = images.length;
  }, [images]);

  const handleSubmit = async () => {
    if (isSubmitting) {
      return;
    }
    if (!title) {
      setError(t("Title cannot be empty"));
      return;
    }
    for (let i = 0; i < altTitles.length; i++) {
      if (!altTitles[i]) {
        setError(t("Alternative title cannot be empty"));
        return;
      }
    }
    for (let i = 0; i < links.length; i++) {
      if (!links[i].label || !links[i].url) {
        setError(t("Link cannot be empty"));
        return;
      }
    }
    if (!tags || tags.length === 0) {
      setError(t("At least one tag required"));
      return;
    }
    if (!article) {
      setError(t("Description cannot be empty"));
      return;
    }
    setSubmitting(true);
    try {
      await onSubmit({
        title,
        altTitles,
        releaseDate,
        skipUpdateTime,
        tags,
        article,
        images,
        coverId,
        links,
        galleryImages,
        galleryNsfw,
        characters,
        relations,
      });
      if (storageKey) {
        localStorage.removeItem(storageKey);
      }
    } catch (err: any) {
      setError(err.message || "An error occurred");
      setSubmitting(false);
    }
  };

  return (
    <ImageDropArea
      onUploaded={(images) => {
        setImages((prev) => [...prev, ...images]);
      }}
    >
      <div className={"p-4 bg-base-100/80 backdrop-blur-sm rounded-box mt-4 shadow mb-4"}>
        <div className="mb-4 p-4 card bg-base-100 shadow border border-base-200">
          <h2 className="text-sm font-bold mb-3">{t("Import from VNDB")}</h2>
          <div className="flex flex-wrap gap-x-4 gap-y-1 mb-3">
            {ALL_SECTIONS.map((section) => (
              <label
                key={section}
                className="flex items-center gap-1.5 cursor-pointer select-none text-sm"
              >
                <input
                  type="checkbox"
                  className="checkbox checkbox-primary checkbox-sm"
                  checked={selectedSections.includes(section)}
                  onChange={() => toggleSection(section)}
                />
                {t(SECTION_LABELS[section])}
              </label>
            ))}
          </div>
          <div className="flex flex-col gap-2 md:flex-row">
            <input
              type="text"
              className="input input-sm flex-1"
              placeholder={t("v12345 or https://vndb.org/v12345")}
              value={vnID}
              onChange={(e) => setVNID(e.target.value.trim())}
            />
            <button
              type="button"
              className="btn btn-sm btn-primary"
              onClick={handleImportFromVNDB}
              disabled={isImporting || selectedSections.length === 0}
            >
              {isImporting && <span className="loading loading-spinner loading-xs"></span>}
              <span>{t("Import from VNDB")}</span>
            </button>
          </div>
          {importError && <ErrorAlert className="mt-2" message={importError} />}
        </div>
        <h1 className={"text-2xl font-bold my-4"}>{pageTitle}</h1>
        <div role="alert" className="alert alert-info mb-2 alert-dash">
          <MdOutlineInfo size={24} />
          <span>{t("All information can be modified after publishing")}</span>
        </div>
        <p className={"my-1"}>{t("Title")}</p>
        <input
          type="text"
          className="input w-full"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
        <div className={"h-4"}></div>
        <p className={"my-1"}>{t("Alternative Titles")}</p>
        {altTitles.map((title, index) => {
          return (
            <div key={index} className={"flex items-center my-2"}>
              <input
                type="text"
                className="input w-full"
                value={title}
                onChange={(e) => {
                  const newAltTitles = [...altTitles];
                  newAltTitles[index] = e.target.value;
                  setAltTitles(newAltTitles);
                }}
              />
              <button
                className={"btn btn-square btn-error ml-2"}
                type={"button"}
                onClick={() => {
                  const newAltTitles = [...altTitles];
                  newAltTitles.splice(index, 1);
                  setAltTitles(newAltTitles);
                }}
              >
                <MdDelete size={24} />
              </button>
            </div>
          );
        })}
        <button
          className={"btn my-2"}
          type={"button"}
          onClick={() => {
            setAltTitles([...altTitles, ""]);
          }}
        >
          <MdAdd />
          {t("Add Alternative Title")}
        </button>
        <div className={"h-2"}></div>
        <p className={"my-1"}>{t("Release Date")}</p>
        <input
          type="date"
          className="input"
          value={releaseDate || ""}
          onChange={(e) => setReleaseDate(e.target.value || undefined)}
        />
        <div className={"h-4"}></div>
        <p className={"my-1"}>{t("Links")}</p>
        <div className={"flex flex-col"}>
          {links.map((link, index) => {
            return (
              <div key={index} className={"flex items-center my-2"}>
                <input
                  type="text"
                  className="input"
                  placeholder={t("Label")}
                  value={link.label}
                  onChange={(e) => {
                    const newLinks = [...links];
                    newLinks[index].label = e.target.value;
                    setLinks(newLinks);
                  }}
                />
                <input
                  type="text"
                  className="input w-full ml-2"
                  placeholder={t("URL")}
                  value={link.url}
                  onChange={(e) => {
                    const newLinks = [...links];
                    newLinks[index].url = e.target.value;
                    setLinks(newLinks);
                  }}
                />
                <button
                  className={"btn btn-square btn-error ml-2"}
                  type={"button"}
                  onClick={() => {
                    const newLinks = [...links];
                    newLinks.splice(index, 1);
                    setLinks(newLinks);
                  }}
                >
                  <MdDelete size={24} />
                </button>
              </div>
            );
          })}
          <div className={"flex"}>
            <button
              className={"btn my-2"}
              type={"button"}
              onClick={() => {
                setLinks([...links, { label: "", url: "" }]);
              }}
            >
              <MdAdd />
              {t("Add Link")}
            </button>
          </div>
        </div>
        <div className={"h-2"}></div>
        <p className={"my-1"}>{t("Tags")}</p>
        <p className={"my-1 pb-1"}>
          {tags.map((tag, index) => {
            return (
              <span key={index} className={"badge badge-primary mr-2 text-sm"}>
                {tag.name}
                <span
                  onClick={() => {
                    const newTags = [...tags];
                    newTags.splice(index, 1);
                    setTags(newTags);
                  }}
                >
                  <MdClose size={18} />
                </span>
              </span>
            );
          })}
        </p>
        <div className={"flex flex-wrap items-center gap-2"}>
          <TagInput
            onAdd={(tag) => {
              setTags((prev) => {
                const existingTag = prev.find((t) => t.id === tag.id);
                if (existingTag) {
                  return prev;
                }
                return [...prev, tag];
              });
            }}
          />
          <QuickAddTagDialog
            onAdded={(tags) => {
              setTags((prev) => {
                const newTags = [...prev];
                for (const tag of tags) {
                  const existingTag = newTags.find((t) => t.id === tag.id);
                  if (!existingTag) {
                    newTags.push(tag);
                  }
                }
                return newTags;
              });
            }}
          />
          <TagTemplatePanel
            onAdded={(tags) => {
              setTags((prev) => {
                const newTags = [...prev];
                for (const tag of tags) {
                  const existingTag = newTags.find((t) => t.id === tag.id);
                  if (!existingTag) {
                    newTags.push(tag);
                  }
                }
                return newTags;
              });
            }}
          />
        </div>
        <div className={"h-4"}></div>
        <p className={"my-1"}>{t("Description")}</p>
        <div className="flex flex-wrap gap-2 mb-2">
          <button
            type="button"
            className="btn btn-sm btn-outline"
            onClick={() => {
              const snippet = ":::collapse \u6807\u9898\n\u5185\u5bb9\n:::";
              setArticle((prev) => prev + "\n" + snippet);
            }}
          >
            + Collapse
          </button>
          <button
            type="button"
            className="btn btn-sm btn-outline"
            onClick={() => {
              const snippet = ":::collapse+ \u6807\u9898\n\u5185\u5bb9\n:::";
              setArticle((prev) => prev + "\n" + snippet);
            }}
          >
            + Collapse (Open)
          </button>
          <button
            type="button"
            className="btn btn-sm btn-outline"
            onClick={() => {
              const snippet =
                ":::tab_view Tab1/Tab2/Tab3\nTab1 \u5185\u5bb9\n---\nTab2 \u5185\u5bb9\n---\nTab3 \u5185\u5bb9\n:::";
              setArticle((prev) => prev + "\n" + snippet);
            }}
          >
            + Tab View
          </button>
          <button
            type="button"
            className="btn btn-sm btn-outline"
            onClick={() => {
              const snippet =
                ":::collapse+ \u7b80\u4ecb\n\n:::tab_view \u7ffb\u8bd1/\u539f\u6587\nTab1 \u5185\u5bb9\n---\nTab2 \u5185\u5bb9\n:::\n\n:::";
              setArticle((prev) => prev + "\n" + snippet);
            }}
          >
            + 折叠的简介
          </button>
        </div>
        <textarea
          ref={articleRef}
          className="textarea w-full min-h-80 p-4"
          value={article}
          onChange={(e) => setArticle(e.target.value)}
        />
        <div className={"flex items-center py-1 "}>
          <MdOutlineInfo className={"inline mr-1"} />
          <span className={"text-sm"}>{t("Use markdown format")}</span>
        </div>
        <div className={"h-4"}></div>
        <p className={"my-1"}>{t("Images")}</p>
        <div role="alert" className="alert alert-info alert-soft my-2">
          <MdOutlineInfo size={24} />
          <div>
            <p>
              {t(
                "Images will not be displayed automatically, you need to reference them in the description",
              )}
            </p>
            <p>{t("You can select a cover image using the radio button in the Cover column")}</p>
          </div>
        </div>
        <div
          ref={imageListRef}
          className={`rounded-box border border-base-content/5 bg-base-100 max-h-112 overflow-y-auto ${images.length === 0 ? "hidden" : ""}`}
        >
          <table className={"table"}>
            <thead>
              <tr>
                <td>{t("Preview")}</td>
                <td>{"Markdown"}</td>
                <td>{t("Cover")}</td>
                <td>{t("Gallery")}</td>
                <td>{"Nsfw"}</td>
                <td>{t("Action")}</td>
              </tr>
            </thead>
            <tbody>
              {images.map((image, index) => {
                return (
                  <tr key={index} className={"hover"}>
                    <td>
                      <img
                        src={network.getImageUrl(image)}
                        className={"w-16 h-16 object-cover card"}
                        alt={"image"}
                      />
                    </td>
                    <td>
                      <span>{`![](${network.getImageUrl(image)})`}</span>
                      <button
                        className={"btn btn-sm btn-circle btn-ghost ml-1"}
                        onClick={() => {
                          navigator.clipboard.writeText(`![](${network.getImageUrl(image)})`);
                        }}
                      >
                        <MdContentCopy />
                      </button>
                    </td>
                    <td>
                      <input
                        type="radio"
                        name="cover"
                        className="radio radio-accent"
                        checked={coverId === image}
                        onChange={() => setCoverId(image)}
                      />
                    </td>
                    <td>
                      <input
                        type="checkbox"
                        className="checkbox checkbox-accent"
                        checked={galleryImages.includes(image)}
                        onChange={(e) => {
                          if (e.target.checked) {
                            setGalleryImages((prev) => [...prev, image]);
                          } else {
                            setGalleryImages((prev) => prev.filter((id) => id !== image));
                          }
                        }}
                      />
                    </td>
                    <td>
                      <input
                        type="checkbox"
                        className="checkbox checkbox-accent"
                        checked={galleryNsfw.includes(image)}
                        onChange={(e) => {
                          if (e.target.checked) {
                            setGalleryNsfw((prev) => [...prev, image]);
                          } else {
                            setGalleryNsfw((prev) => prev.filter((id) => id !== image));
                          }
                        }}
                      />
                    </td>
                    <td>
                      <button
                        className={"btn btn-square"}
                        type={"button"}
                        onClick={() => {
                          const id = images[index];
                          const newImages = [...images];
                          newImages.splice(index, 1);
                          setImages(newImages);
                          if (coverId === id) {
                            setCoverId(undefined);
                          }
                          network.deleteImage(id);
                        }}
                      >
                        <MdDelete size={24} />
                      </button>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        <div className={"flex"}>
          <SelectAndUploadImageButton
            onUploaded={(images) => {
              setImages((prev) => [...prev, ...images]);
            }}
          />
          <span className={"w-4"}></span>
          <UploadClipboardImageButton
            onUploaded={(images) => {
              setImages((prev) => [...prev, ...images]);
            }}
          />
        </div>
        <div className={"h-4"}></div>
        <div>
          <p className={"my-1"}>{t("Characters")}</p>
          <div className="grid grid-cols-1 md:grid-cols-2 my-2 gap-4">
            {characters.map((character, index) => {
              return (
                <CharacterEditor
                  key={index}
                  character={character}
                  setCharacter={(newCharacter) => {
                    const newCharacters = [...characters];
                    newCharacters[index] = newCharacter;
                    setCharacters(newCharacters);
                  }}
                  onDelete={() => {
                    const newCharacters = [...characters];
                    newCharacters.splice(index, 1);
                    setCharacters(newCharacters);
                  }}
                />
              );
            })}
          </div>
          <div className="flex">
            <button
              className={"btn my-2"}
              type={"button"}
              onClick={() => {
                setCharacters([
                  ...characters,
                  { name: "", alias: [], cv: "", image: 0, role: "primary" },
                ]);
              }}
            >
              <MdAdd />
              {t("Add Character")}
            </button>
            {links.find((link) => link.label.toLowerCase() === "vndb") && (
              <div className="ml-4 my-2">
                <FetchVndbCharactersButton
                  vnID={
                    parseVndbId(
                      links.find((link) => link.label.toLowerCase() === "vndb")
                        ?.url ?? "",
                    ) ?? ""
                  }
                  onFetch={(fetchedCharacters, fetchedReleaseDate) => {
                    setCharacters(fetchedCharacters);
                    if (fetchedReleaseDate) {
                      setReleaseDate(fetchedReleaseDate);
                    }
                  }}
                />
              </div>
            )}
          </div>
        </div>
        <div className={"h-4"}></div>
        <p className={"my-1"}>{t("Related Resources")}</p>
        <RelationEditor relations={relations} setRelations={setRelations} excludeId={excludeId} />
        <div className={"h-4"}></div>
        {error && (
          <div role="alert" className="alert alert-error my-2 shadow">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              className="h-6 w-6 shrink-0 stroke-current"
              fill="none"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth="2"
                d="M10 14l2-2m0 0l2-2m-2 2l-2-2m2 2l2 2m7-2a9 9 0 11-18 0 9 9 0 0118 0z"
              />
            </svg>
            <span>
              {t("Error")}: {error}
            </span>
          </div>
        )}
        <div className={"flex flex-row-reverse mt-4 items-center"}>
          <button className={"btn btn-accent shadow"} onClick={handleSubmit}>
            {isSubmitting && <span className="loading loading-spinner"></span>}
            {submitButtonText}
          </button>
          <div className="w-2"></div>
          {showSkipUpdateTimeOption && (
            <label className="label cursor-pointer justify-start gap-3 mb-2 rounded-box border border-base-300 px-2 py-2">
              <input
                type="checkbox"
                className="checkbox checkbox-primary checkbox-sm"
                checked={skipUpdateTime}
                onChange={(e) => setSkipUpdateTime(e.target.checked)}
              />
              <span className="text-sm">{t("Skip updating modified time")}</span>
            </label>
          )}
          <div className="flex-1"></div>
        </div>
      </div>
    </ImageDropArea>
  );
}

function RelationEditor({
  relations,
  setRelations,
  excludeId,
}: {
  relations: RelationFormItem[];
  setRelations: React.Dispatch<React.SetStateAction<RelationFormItem[]>>;
  excludeId?: number;
}) {
  const [keyword, setKeyword] = useState("");
  const [results, setResults] = useState<Resource[] | null>(null);
  const [isSearching, setSearching] = useState(false);
  const debounceRef = useRef(new Debounce(400));
  const { t } = useTranslation();

  const handleSearch = async (kw: string) => {
    if (!kw.trim()) {
      setResults(null);
      return;
    }
    setSearching(true);
    setResults(null);
    const res = await network.searchResources(kw.trim(), 1);
    setSearching(false);
    if (res.success && res.data) {
      setResults(res.data);
    } else {
      setResults([]);
    }
  };

  useEffect(() => {
    if (!keyword.trim()) {
      debounceRef.current.cancel();
      setResults(null);
      return;
    }
    debounceRef.current.run(() => handleSearch(keyword));
  }, [keyword]);

  const addRelation = (r: Resource) => {
    if (relations.find((rel) => rel.toId === r.id)) return;
    setRelations((prev) => [...prev, { toId: r.id, toTitle: r.title, description: "" }]);
  };

  const removeRelation = (toId: number) => {
    setRelations((prev) => prev.filter((rel) => rel.toId !== toId));
  };

  const updateDescription = (toId: number, description: string) => {
    setRelations((prev) =>
      prev.map((rel) => (rel.toId === toId ? { ...rel, description } : rel)),
    );
  };

  return (
    <div className="space-y-2">
      {relations.length > 0 && (
        <div className="rounded-box border border-base-200 bg-base-100 divide-y divide-base-200 overflow-hidden">
          {relations.map((rel) => (
            <div key={rel.toId} className="flex items-center gap-3 px-3 py-2">
              <span className="text-xs font-medium shrink-0 whitespace-normal text-left w-48">
                {rel.toTitle}
              </span>
              <input
                type="text"
                className="input input-sm flex-1 bg-base-200/50 border-none focus:bg-base-100"
                placeholder={t("Description (optional)")}
                value={rel.description}
                onChange={(e) => updateDescription(rel.toId, e.target.value)}
              />
              <button
                className="btn btn-sm btn-ghost btn-square text-error hover:bg-error/10"
                type="button"
                onClick={() => removeRelation(rel.toId)}
              >
                <MdDelete size={16} />
              </button>
            </div>
          ))}
        </div>
      )}
      <div className="relative">
        <input
          type="text"
          className="input input-sm w-full pr-14"
          placeholder={t("Search resource by title")}
          value={keyword}
          onChange={(e) => setKeyword(e.target.value)}
        />
        <div className="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-1">
          {isSearching && <span className="loading loading-spinner loading-xs text-primary" />}
          {keyword && (
            <button
              type="button"
              className="btn btn-xs btn-ghost btn-circle"
              onClick={() => setKeyword("")}
            >
              <MdClose size={14} />
            </button>
          )}
        </div>
      </div>
      {results != null && results.filter((r) => !relations.find((rel) => rel.toId === r.id) && r.id !== excludeId).length > 0 && (
        <div className="rounded-box border border-base-200 bg-base-100 shadow-sm mt-1 max-h-60 overflow-y-auto divide-y divide-base-200">
          {results
            .filter((r) => !relations.find((rel) => rel.toId === r.id) && r.id !== excludeId)
            .map((r) => (
              <div
                key={r.id}
                className="flex items-center gap-2 px-3 py-2.5 hover:bg-primary/5 cursor-pointer transition-colors"
                onClick={() => addRelation(r)}
              >
                <MdAdd size={16} className="shrink-0 text-primary" />
                <span className="text-sm">{r.title}</span>
              </div>
            ))}
        </div>
      )}
      {results != null && results.length === 0 && (
        <p className="text-sm text-base-content/50 text-center py-3">{t("No results found")}</p>
      )}
    </div>
  );
}
