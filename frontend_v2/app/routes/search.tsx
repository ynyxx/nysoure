import { useEffect, useMemo, useState } from "react";
import type { Route } from "./+types/search";
import { useLoaderData, useSearchParams } from "react-router";
import { MdSearch } from "react-icons/md";
import { useTranslation } from "../hook/i18n";
import { network } from "../network/network";
import { configFromMatches } from "../hook/config";
import ResourcesView from "~/components/resources_view";
import TagSelector from "~/components/tag_selector";
import DatePicker from "~/components/date_picker";
import Button from "~/components/button";
import { InfoAlert } from "~/components/alert";
import type { PageResponse, Resource } from "../network/models";

type SearchFilters = {
  keyword: string;
  tags: string[];
  releaseFrom: string;
  releaseTo: string;
};

function parseTagsParam(value: string | null): string[] {
  if (!value) {
    return [];
  }
  const seen = new Set<string>();
  const tags: string[] = [];
  for (const part of value.split(",")) {
    const tag = part.trim();
    if (tag && !seen.has(tag)) {
      seen.add(tag);
      tags.push(tag);
    }
  }
  return tags;
}

function parseDateParam(value: string | null): string {
  const date = value?.trim() ?? "";
  return /^\d{4}-\d{2}-\d{2}$/.test(date) ? date : "";
}

function parseSearchFilters(params: URLSearchParams): SearchFilters {
  return {
    keyword: params.get("keyword")?.trim() ?? "",
    tags: parseTagsParam(params.get("tags")),
    releaseFrom: parseDateParam(params.get("release_from")),
    releaseTo: parseDateParam(params.get("release_to")),
  };
}

function hasSearchFilters(filters: SearchFilters): boolean {
  return (
    filters.keyword.length > 0 ||
    filters.tags.length > 0 ||
    filters.releaseFrom.length > 0 ||
    filters.releaseTo.length > 0
  );
}

function filtersToSearchParams(filters: SearchFilters): URLSearchParams {
  const params = new URLSearchParams();
  if (filters.keyword) {
    params.set("keyword", filters.keyword);
  }
  if (filters.tags.length > 0) {
    params.set("tags", filters.tags.join(","));
  }
  if (filters.releaseFrom) {
    params.set("release_from", filters.releaseFrom);
  }
  if (filters.releaseTo) {
    params.set("release_to", filters.releaseTo);
  }
  return params;
}

function searchKey(filters: SearchFilters): string {
  return [
    filters.keyword,
    filters.tags.join(","),
    filters.releaseFrom,
    filters.releaseTo,
  ].join("|");
}

export function meta({ matches, location }: Route.MetaArgs) {
  const config = configFromMatches(matches);
  const filters = parseSearchFilters(new URLSearchParams(location.search));
  const titleParts = [
    filters.keyword,
    ...filters.tags,
    filters.releaseFrom && filters.releaseTo
      ? `${filters.releaseFrom} ~ ${filters.releaseTo}`
      : filters.releaseFrom || filters.releaseTo,
  ].filter(Boolean);
  return [
    {
      title: titleParts.length
        ? `${titleParts.join(" / ")} - Search - ${config.server_name}`
        : `Search - ${config.server_name}`,
    },
    { name: "description", content: `Search resources on ${config.server_name}` },
  ];
}

export async function loader({ request }: Route.LoaderArgs) {
  const url = new URL(request.url);
  const filters = parseSearchFilters(url.searchParams);

  let firstPageResults: PageResponse<Resource> | undefined;
  let allTags: Awaited<ReturnType<typeof network.getAllTags>>["data"] = [];

  const tagsResponse = await network.getAllTags();
  if (tagsResponse.success) {
    allTags = tagsResponse.data ?? [];
  }

  if (hasSearchFilters(filters)) {
    const result = await network.searchResources(filters.keyword, 1, {
      tags: filters.tags,
      releaseFrom: filters.releaseFrom,
      releaseTo: filters.releaseTo,
    });
    if (result.success) {
      firstPageResults = result;
    }
  }

  return {
    allTags: allTags ?? [],
    firstPageResults,
  };
}

export default function SearchPage() {
  const { t } = useTranslation();
  const { allTags, firstPageResults } = useLoaderData<typeof loader>();
  const [searchParams, setSearchParams] = useSearchParams();
  const applied = useMemo(
    () => parseSearchFilters(searchParams),
    [searchParams],
  );

  const [keyword, setKeyword] = useState(applied.keyword);
  const [tags, setTags] = useState<string[]>(applied.tags);
  const [releaseFrom, setReleaseFrom] = useState(applied.releaseFrom);
  const [releaseTo, setReleaseTo] = useState(applied.releaseTo);
  const [dateError, setDateError] = useState<string | null>(null);

  useEffect(() => {
    setKeyword(applied.keyword);
    setTags(applied.tags);
    setReleaseFrom(applied.releaseFrom);
    setReleaseTo(applied.releaseTo);
    setDateError(null);
  }, [applied]);

  const draft: SearchFilters = {
    keyword: keyword.trim(),
    tags,
    releaseFrom,
    releaseTo,
  };
  const canSearch = hasSearchFilters(draft);
  const appliedHasFilters = hasSearchFilters(applied);
  const emptyResults =
    appliedHasFilters &&
    firstPageResults != null &&
    (firstPageResults.data?.length ?? 0) === 0;

  const submitSearch = (filters = draft) => {
    if (filters.releaseFrom && filters.releaseTo && filters.releaseFrom > filters.releaseTo) {
      setDateError(t("Invalid date range"));
      return;
    }
    setDateError(null);
    setSearchParams(filtersToSearchParams(filters), { preventScrollReset: true });
  };

  const clearFilters = () => {
    setKeyword("");
    setTags([]);
    setReleaseFrom("");
    setReleaseTo("");
    setDateError(null);
    setSearchParams(new URLSearchParams(), { preventScrollReset: true });
  };

  return (
    <div className="flex flex-col lg:flex-row lg:items-start gap-4 mt-4 mb-4">
      <aside className="w-full lg:w-80 xl:w-96 shrink-0 lg:sticky lg:top-24">
        <div className="flex flex-col gap-4 p-4 bg-base-100/80 backdrop-blur-sm rounded-box shadow">
          <h1 className="text-2xl font-bold">{t("Search")}</h1>
          <form
            className="flex flex-col gap-4"
            onSubmit={(e) => {
              e.preventDefault();
              submitSearch();
            }}
          >
            <fieldset className="fieldset w-full p-0">
              <legend className="fieldset-legend">{t("Keyword")}</legend>
              <label className="input w-full">
                <MdSearch size={18} />
                <input
                  type="search"
                  className="w-full"
                  autoComplete="off"
                  placeholder={t("Search by title, alias or character")}
                  value={keyword}
                  onChange={(e) => setKeyword(e.target.value)}
                />
              </label>
            </fieldset>

            <fieldset className="fieldset w-full p-0">
              <legend className="fieldset-legend">{t("Tags")}</legend>
              <TagSelector selected={tags} onChange={setTags} allTags={allTags} />
            </fieldset>

            <fieldset className="fieldset w-full p-0">
              <legend className="fieldset-legend">{t("Release Date")}</legend>
              <div className="flex flex-col sm:flex-row lg:flex-col sm:items-center lg:items-stretch gap-2">
                <DatePicker
                  label={t("From")}
                  value={releaseFrom}
                  max={releaseTo || undefined}
                  onChange={(next) => {
                    setReleaseFrom(next);
                    setDateError(null);
                  }}
                />
                <span className="hidden sm:block lg:hidden text-base-content/60">
                  —
                </span>
                <DatePicker
                  label={t("To")}
                  value={releaseTo}
                  min={releaseFrom || undefined}
                  onChange={(next) => {
                    setReleaseTo(next);
                    setDateError(null);
                  }}
                />
              </div>
              <p className="text-xs text-base-content/60 mt-2">
                {t(
                  "Works without a release date are hidden when a date filter is applied",
                )}
              </p>
              {dateError && (
                <p className="text-error text-sm mt-1">{dateError}</p>
              )}
            </fieldset>

            <div className="flex flex-row-reverse gap-2">
              <Button className="btn-primary" type="submit" disabled={!canSearch}>
                {t("Search")}
              </Button>
              <Button
                className="btn-ghost"
                type="button"
                disabled={!canSearch && !appliedHasFilters}
                onClick={clearFilters}
              >
                {t("Clear Filters")}
              </Button>
            </div>
          </form>
        </div>
      </aside>

      <div className="flex-1 min-w-0">
        {!appliedHasFilters && (
          <div className="px-1 mb-4">
            <InfoAlert message={t("Add search filters to find resources")} />
          </div>
        )}

        {emptyResults && (
          <div className="px-1 mb-4">
            <InfoAlert message={t("No resources found")} />
          </div>
        )}

        {appliedHasFilters && (
          <ResourcesView
            key={searchKey(applied)}
            storageKey={`search-${searchKey(applied)}`}
            loader={(page) =>
              network.searchResources(applied.keyword, page, {
                tags: applied.tags,
                releaseFrom: applied.releaseFrom,
                releaseTo: applied.releaseTo,
              })
            }
            initialData={firstPageResults}
          />
        )}
      </div>
    </div>
  );
}
