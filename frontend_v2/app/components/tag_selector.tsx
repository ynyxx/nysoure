import { useRef, useState } from "react";
import { MdClose, MdSearch } from "react-icons/md";
import type { Tag, TagWithCount } from "../network/models";
import { network } from "../network/network";
import { useTranslation } from "../hook/i18n";
import { Debounce } from "../utils/debounce";
import Badge from "./badge";
import Button from "./button";

export default function TagSelector({
  selected,
  onChange,
  allTags,
}: {
  selected: string[];
  onChange: (tags: string[]) => void;
  allTags: TagWithCount[];
}) {
  const { t } = useTranslation();
  const [keyword, setKeyword] = useState("");
  const [suggestions, setSuggestions] = useState<Tag[]>([]);
  const [isLoading, setLoading] = useState(false);
  const [draft, setDraft] = useState<string[]>(selected);
  const [browseKeyword, setBrowseKeyword] = useState("");
  const [browseSuggestions, setBrowseSuggestions] = useState<Tag[]>([]);
  const debounce = useRef(new Debounce(300));
  const browseDebounce = useRef(new Debounce(300));
  const dialogRef = useRef<HTMLDialogElement>(null);

  const searchTags = async (value: string) => {
    if (value.length === 0) {
      setSuggestions([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    const res = await network.searchTags(value, true);
    if (!res.success) {
      setSuggestions([]);
      setLoading(false);
      return;
    }
    setSuggestions(res.data ?? []);
    setLoading(false);
  };

  const handleKeywordChange = (value: string) => {
    setKeyword(value);
    if (value.length === 0) {
      debounce.current.cancel();
      setSuggestions([]);
      setLoading(false);
      return;
    }
    setLoading(true);
    debounce.current.run(() => searchTags(value));
  };

  const addTag = (name: string) => {
    if (!name || selected.includes(name)) {
      setKeyword("");
      setSuggestions([]);
      return;
    }
    onChange([...selected, name]);
    setKeyword("");
    setSuggestions([]);
  };

  const removeTag = (name: string) => {
    onChange(selected.filter((tag) => tag !== name));
  };

  const openBrowse = () => {
    setDraft(selected);
    setBrowseKeyword("");
    setBrowseSuggestions([]);
    dialogRef.current?.showModal();
  };

  const handleBrowseKeywordChange = (value: string) => {
    setBrowseKeyword(value);
    const trimmed = value.trim();
    if (trimmed.length === 0) {
      browseDebounce.current.cancel();
      setBrowseSuggestions([]);
      return;
    }
    browseDebounce.current.run(async () => {
      const res = await network.searchTags(trimmed, true);
      if (!res.success) {
        setBrowseSuggestions([]);
        return;
      }
      setBrowseSuggestions(res.data ?? []);
    });
  };

  const toggleDraft = (name: string) => {
    setDraft((prev) =>
      prev.includes(name) ? prev.filter((tag) => tag !== name) : [...prev, name],
    );
  };

  const confirmDraft = () => {
    onChange(draft);
    dialogRef.current?.close();
  };

  const filteredBrowseTags = (() => {
    const q = browseKeyword.trim().toLowerCase();
    const local = q
      ? allTags.filter(
          (tag) =>
            tag.name.toLowerCase().includes(q) ||
            tag.type.toLowerCase().includes(q),
        )
      : allTags;
    if (!q) {
      return local;
    }
    const known = new Set(local.map((tag) => tag.name));
    const extra: TagWithCount[] = [];
    for (const tag of browseSuggestions) {
      if (known.has(tag.name)) {
        continue;
      }
      known.add(tag.name);
      extra.push({
        ...tag,
        resources_count: 0,
      });
    }
    return [...extra, ...local];
  })();

  const tagsMap = new Map<string, TagWithCount[]>();
  for (const tag of filteredBrowseTags) {
    const type = tag.type;
    if (!tagsMap.has(type)) {
      tagsMap.set(type, []);
    }
    tagsMap.get(type)?.push(tag);
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-2">
        {selected.length === 0 ? (
          <span className="text-sm text-base-content/60">
            {t("No tags selected")}
          </span>
        ) : (
          selected.map((name) => (
            <Badge
              key={name}
              className="badge-soft badge-primary cursor-pointer"
              onClick={() => removeTag(name)}
            >
              {name}
              <MdClose size={14} className="ml-1" />
            </Badge>
          ))
        )}
      </div>
      <div className="flex flex-col sm:flex-row lg:flex-col gap-2">
        <div className={`dropdown flex-1 ${keyword ? "dropdown-open" : ""}`}>
          <label className="input w-full">
            <MdSearch size={18} />
            <input
              autoComplete="off"
              type="text"
              className="grow"
              placeholder={t("Search Tags")}
              value={keyword}
              onChange={(e) => handleKeywordChange(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") {
                  e.preventDefault();
                  if (suggestions.length > 0) {
                    addTag(suggestions[0].name);
                  }
                }
              }}
            />
          </label>
          {(keyword.length > 0 || isLoading) && (
            <ul
              tabIndex={0}
              className="dropdown-content menu bg-base-100 rounded-box z-20 w-full p-2 shadow mt-2 border border-base-300"
            >
              {isLoading ? (
                <li>
                  <span>
                    <span className="loading loading-spinner loading-sm" />
                    {t("Searching...")}
                  </span>
                </li>
              ) : suggestions.length === 0 ? (
                <li>
                  <span>{t("No matching tags")}</span>
                </li>
              ) : (
                suggestions.map((tag) => (
                  <li key={tag.id}>
                    <button
                      type="button"
                      onClick={() => addTag(tag.name)}
                      className="flex justify-between"
                    >
                      <span>{tag.name}</span>
                      {tag.type && (
                        <span className="badge badge-secondary badge-sm">
                          {tag.type}
                        </span>
                      )}
                    </button>
                  </li>
                ))
              )}
            </ul>
          )}
        </div>
        <Button className="btn-soft btn-primary w-full sm:w-auto lg:w-full" type="button" onClick={openBrowse}>
          {t("Select Tags")}
        </Button>
      </div>

      <dialog ref={dialogRef} className="modal">
        <div className="modal-box max-w-4xl w-full">
          <h3 className="font-bold text-lg">{t("Select Tags")}</h3>
          <p className="text-sm text-base-content/60 mt-1">
            {t("Selected Tags")}: {draft.length}
          </p>
          <label className="input w-full mt-3">
            <MdSearch size={18} />
            <input
              autoComplete="off"
              type="text"
              className="grow"
              placeholder={t("Search Tags")}
              value={browseKeyword}
              onChange={(e) => handleBrowseKeywordChange(e.target.value)}
            />
          </label>
          <div className="flex flex-wrap gap-2 mt-3 min-h-8">
            {draft.map((name) => (
              <Badge
                key={name}
                className="badge-primary cursor-pointer"
                onClick={() => toggleDraft(name)}
              >
                {name}
                <MdClose size={14} className="ml-1" />
              </Badge>
            ))}
          </div>
          <div className="max-h-[50vh] overflow-y-auto mt-4 flex flex-col gap-4">
            {tagsMap.size === 0 ? (
              <span className="text-sm text-base-content/60">
                {t("No matching tags")}
              </span>
            ) : (
              Array.from(tagsMap.entries()).map(([type, tagList]) => (
                <div key={type} className="flex flex-col gap-2">
                  <h4 className="text-sm font-bold">
                    {type === "" ? t("Other") : type}
                  </h4>
                  <div className="flex flex-wrap gap-2">
                    {tagList.map((tag) => {
                      const active = draft.includes(tag.name);
                      return (
                        <Badge
                          key={tag.name}
                          className={`cursor-pointer ${
                            active
                              ? "badge-primary"
                              : "badge-soft badge-primary"
                          }`}
                          onClick={() => toggleDraft(tag.name)}
                        >
                          {tag.name}
                          {tag.resources_count > 0
                            ? ` (${tag.resources_count})`
                            : ""}
                        </Badge>
                      );
                    })}
                  </div>
                </div>
              ))
            )}
          </div>
          <div className="modal-action">
            <form method="dialog">
              <Button className="btn">{t("Cancel")}</Button>
            </form>
            <Button className="btn-primary" type="button" onClick={confirmDraft}>
              {t("Confirm")}
            </Button>
          </div>
        </div>
        <form method="dialog" className="modal-backdrop">
          <button>close</button>
        </form>
      </dialog>
    </div>
  );
}
