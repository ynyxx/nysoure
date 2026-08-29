import { useEffect, useId, useRef, useState } from "react";
import { DayPicker } from "react-day-picker";
import { zhCN, zhTW } from "react-day-picker/locale";
import { MdClose, MdOutlineDateRange } from "react-icons/md";
import { useRouteLoaderData } from "react-router";
import { useTranslation } from "../hook/i18n";

function parseISODate(value: string): Date | undefined {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) {
    return undefined;
  }
  const [year, month, day] = value.split("-").map(Number);
  const date = new Date(year, month - 1, day);
  if (Number.isNaN(date.getTime())) {
    return undefined;
  }
  return date;
}

function formatISODate(date: Date): string {
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function useDayPickerLocale() {
  const data = useRouteLoaderData("app") as { i18n?: Record<string, string> };
  const search = data?.i18n?.["Search"];
  if (search === "搜索") {
    return zhCN;
  }
  if (search === "搜尋") {
    return zhTW;
  }
  return undefined;
}

export default function DatePicker({
  value,
  onChange,
  min,
  max,
  label,
}: {
  value: string;
  onChange: (value: string) => void;
  min?: string;
  max?: string;
  label?: string;
}) {
  const { t } = useTranslation();
  const locale = useDayPickerLocale();
  const reactId = useId().replace(/:/g, "");
  const popoverId = `rdp-${reactId}`;
  const anchorName = `--${popoverId}`;
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [mounted, setMounted] = useState(false);

  useEffect(() => {
    setMounted(true);
    buttonRef.current?.setAttribute("popovertarget", popoverId);
  }, [popoverId]);

  const selected = parseISODate(value);
  const minDate = parseISODate(min ?? "");
  const maxDate = parseISODate(max ?? "");
  const today = new Date();
  const startMonth = minDate ?? new Date(1970, 0);
  const endMonth = maxDate ?? new Date(today.getFullYear() + 5, 11);

  const hidePopover = () => {
    const el = document.getElementById(popoverId);
    el?.hidePopover?.();
  };

  return (
    <div className="relative w-full">
      <button
        ref={buttonRef}
        type="button"
        className="input w-full justify-start"
        style={{ anchorName } as React.CSSProperties}
      >
        {label && (
          <span className="text-sm text-base-content/60 shrink-0">{label}</span>
        )}
        <MdOutlineDateRange size={18} className="shrink-0" />
        <span
          className={`grow text-left truncate ${selected ? "" : "text-base-content/50"}`}
        >
          {selected ? formatISODate(selected) : t("Pick a date")}
        </span>
      </button>
      {selected && (
        <button
          type="button"
          className="btn btn-ghost btn-xs btn-circle absolute right-2 top-1/2 -translate-y-1/2 z-1"
          aria-label={t("Clear")}
          onClick={(e) => {
            e.preventDefault();
            e.stopPropagation();
            onChange("");
          }}
        >
          <MdClose size={14} />
        </button>
      )}
      <div
        popover="auto"
        id={popoverId}
        className="dropdown bg-base-100 rounded-box shadow-lg"
        style={{ positionAnchor: anchorName } as React.CSSProperties}
      >
        {mounted && (
          <DayPicker
            className="react-day-picker"
            mode="single"
            locale={locale}
            selected={selected}
            defaultMonth={selected}
            captionLayout="dropdown"
            reverseYears
            startMonth={startMonth}
            endMonth={endMonth}
            disabled={[
              ...(minDate ? [{ before: minDate }] : []),
              ...(maxDate ? [{ after: maxDate }] : []),
            ]}
            onSelect={(date) => {
              onChange(date ? formatISODate(date) : "");
              hidePopover();
            }}
          />
        )}
      </div>
    </div>
  );
}
