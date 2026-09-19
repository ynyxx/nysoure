import { MdAccessTime, MdArrowDownward, MdArrowUpward, MdDownload, MdEvent, MdStar, MdVisibility } from "react-icons/md";
import type { ReactNode } from "react";
import { useTranslation } from "../hook/i18n";
import { RSort } from "../network/models";

type SortDirection = "asc" | "desc";
type SortField = "relevance" | "time" | "views" | "downloads" | "releaseDate";

interface SortOption {
  field: SortField;
  asc: RSort;
  desc: RSort;
  label: string;
  icon: ReactNode;
  ascLabel: string;
  descLabel: string;
  directional?: boolean;
}

export default function ResourceSortControl({
  value,
  onChange,
  className = "",
  includeRelevance = false,
}: {
  value: RSort;
  onChange: (sort: RSort) => void;
  className?: string;
  includeRelevance?: boolean;
}) {
  const { t } = useTranslation();
  const options: SortOption[] = [
    ...(includeRelevance
      ? [
          {
            field: "relevance" as const,
            asc: RSort.Relevance,
            desc: RSort.Relevance,
            label: t("Relevance"),
            icon: <MdStar size={18} />,
            ascLabel: t("Relevance"),
            descLabel: t("Relevance"),
            directional: false,
          },
        ]
      : []),
    {
      field: "time",
      asc: RSort.TimeAsc,
      desc: RSort.TimeDesc,
      label: t("Created At"),
      icon: <MdAccessTime size={18} />,
      ascLabel: t("Time Ascending"),
      descLabel: t("Time Descending"),
    },
    {
      field: "views",
      asc: RSort.ViewsAsc,
      desc: RSort.ViewsDesc,
      label: t("View Count"),
      icon: <MdVisibility size={18} />,
      ascLabel: t("Views Ascending"),
      descLabel: t("Views Descending"),
    },
    {
      field: "downloads",
      asc: RSort.DownloadsAsc,
      desc: RSort.DownloadsDesc,
      label: t("Download Count"),
      icon: <MdDownload size={18} />,
      ascLabel: t("Downloads Ascending"),
      descLabel: t("Downloads Descending"),
    },
    {
      field: "releaseDate",
      asc: RSort.ReleaseDateAsc,
      desc: RSort.ReleaseDateDesc,
      label: t("Release Date"),
      icon: <MdEvent size={18} />,
      ascLabel: t("Release Date Ascending"),
      descLabel: t("Release Date Descending"),
    },
  ];

  const activeOption =
    options.find((option) => option.asc === value || option.desc === value) ??
    options[0];
  const isDirectional = activeOption.directional !== false;
  const direction: SortDirection = activeOption.asc === value ? "asc" : "desc";
  const directionLabel = direction === "asc" ? "ASC" : "DESC";
  const directionAriaLabel =
    direction === "asc" ? activeOption.ascLabel : activeOption.descLabel;

  const selectOptions = options.flatMap((option) =>
    option.directional === false
      ? [{ value: option.asc, label: option.ascLabel }]
      : [
          { value: option.asc, label: option.ascLabel },
          { value: option.desc, label: option.descLabel },
        ],
  );

  const setField = (option: SortOption) => {
    if (option.directional === false) {
      onChange(option.asc);
      return;
    }
    const nextDirection = isDirectional ? direction : "desc";
    onChange(nextDirection === "asc" ? option.asc : option.desc);
  };

  const toggleDirection = () => {
    if (!isDirectional) {
      return;
    }
    onChange(direction === "asc" ? activeOption.desc : activeOption.asc);
  };

  return (
    <div className={`w-full ${className}`}>
      <select
        value={value}
        className="select select-primary w-full max-w-72 bg-base-100/85! shadow-xs backdrop-blur-sm md:hidden"
        onChange={(e) => onChange(Number(e.target.value) as RSort)}
        aria-label={t("Select a Order")}
      >
        {selectOptions.map((option) => (
          <option key={option.value} value={option.value}>
            {option.label}
          </option>
        ))}
      </select>

      <div className="hidden md:flex w-full items-center rounded-box border border-base-300/70 bg-base-100/80 p-1.5 shadow-sm backdrop-blur-sm leading-none">
        <div className="join flex min-w-0 flex-1">
          {options.map((option) => {
            const isActive = option.field === activeOption.field;
            return (
              <button
                key={option.field}
                type="button"
                className={`join-item btn btn-sm min-w-0 flex-1 gap-1 sm:gap-2 border-base-300/70 px-2 ${
                  isActive
                    ? "btn-primary shadow-sm"
                    : "bg-base-100/50 hover:bg-base-200/80"
                }`}
                onClick={() => setField(option)}
                aria-pressed={isActive}
                title={option.label}
              >
                {option.icon}
                <span className="font-medium truncate">{option.label}</span>
              </button>
            );
          })}
        </div>
        {isDirectional && (
          <>
            <div className="mx-1.5 h-6 w-px bg-base-300" />
            <button
              type="button"
              className="btn btn-sm btn-outline btn-primary w-28 shrink-0 gap-2"
              onClick={toggleDirection}
              aria-label={directionAriaLabel}
              title={directionAriaLabel}
            >
              {direction === "asc" ? (
                <MdArrowUpward size={18} />
              ) : (
                <MdArrowDownward size={18} />
              )}
              <span>{directionLabel}</span>
            </button>
          </>
        )}
      </div>
    </div>
  );
}
