import { useEffect, useRef, useState } from "react";
import { MdAdd, MdDelete, MdEdit, MdPlaylistAdd } from "react-icons/md";
import type { Tag, TagTemplate } from "../network/models";
import { network } from "../network/network";
import { useTranslation } from "../hook/i18n";
import {
  extractTagTemplateParams,
  previewTagTemplate,
} from "../utils/tag_template";
import Button from "./button";
import Input, { TextArea } from "./input";
import { ErrorAlert } from "./alert";

type Mode = "list" | "edit" | "apply" | "confirm-delete";

export default function TagTemplatePanel({
  onAdded,
}: {
  onAdded: (tags: Tag[]) => void;
}) {
  const { t } = useTranslation();
  const dialogRef = useRef<HTMLDialogElement>(null);
  const [templates, setTemplates] = useState<TagTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [mode, setMode] = useState<Mode>("list");
  const [editing, setEditing] = useState<TagTemplate | null>(null);
  const [applying, setApplying] = useState<TagTemplate | null>(null);
  const [deleting, setDeleting] = useState<TagTemplate | null>(null);
  const [name, setName] = useState("");
  const [content, setContent] = useState("");
  const [paramValues, setParamValues] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  const loadTemplates = async () => {
    setLoading(true);
    const res = await network.listTagTemplates();
    if (!res.success) {
      setError(res.message || t("Failed to load tag templates"));
      setTemplates([]);
      setLoading(false);
      return;
    }
    setError(null);
    setTemplates(res.data ?? []);
    setLoading(false);
  };

  useEffect(() => {
    loadTemplates();
  }, []);

  const ensureOpen = () => {
    const dialog = dialogRef.current;
    if (dialog && !dialog.open) {
      dialog.showModal();
    }
  };

  const openManager = () => {
    setMode("list");
    setError(null);
    ensureOpen();
  };

  const closeManager = () => {
    dialogRef.current?.close();
    setMode("list");
    setEditing(null);
    setApplying(null);
    setDeleting(null);
    setError(null);
  };

  const openCreate = () => {
    setEditing(null);
    setName("");
    setContent("");
    setError(null);
    setMode("edit");
    ensureOpen();
  };

  const openEdit = (template: TagTemplate) => {
    setEditing(template);
    setName(template.name);
    setContent(template.content);
    setError(null);
    setMode("edit");
    ensureOpen();
  };

  const openApply = (template: TagTemplate) => {
    const values: Record<string, string> = {};
    for (const param of template.params) {
      values[param] = "";
    }
    setApplying(template);
    setParamValues(values);
    setError(null);
    setMode("apply");
    ensureOpen();
  };

  const handleQuickApply = async (template: TagTemplate) => {
    if (template.params.length > 0) {
      openApply(template);
      return;
    }
    setError(null);
    const res = await network.applyTagTemplate(template.id, {});
    if (!res.success || !res.data) {
      setError(res.message || t("Failed to apply tag template"));
      setMode("list");
      ensureOpen();
      return;
    }
    onAdded(res.data);
  };

  const handleSave = async () => {
    if (saving) {
      return;
    }
    const trimmedName = name.trim();
    const trimmedContent = content.trim();
    if (!trimmedName) {
      setError(t("Template name is required"));
      return;
    }
    if (!trimmedContent) {
      setError(t("Template content is required"));
      return;
    }
    const parsed = extractTagTemplateParams(trimmedContent);
    if (parsed.error) {
      setError(t("Invalid tag template"));
      return;
    }
    const preview = previewTagTemplate(trimmedContent, {});
    if (preview.error === "template produces no tags") {
      setError(t("Invalid tag template"));
      return;
    }
    setSaving(true);
    const res = editing
      ? await network.updateTagTemplate(editing.id, trimmedName, trimmedContent)
      : await network.createTagTemplate(trimmedName, trimmedContent);
    setSaving(false);
    if (!res.success) {
      setError(res.message || t("Unknown error"));
      return;
    }
    await loadTemplates();
    setMode("list");
    setEditing(null);
    setError(null);
  };

  const handleDelete = async () => {
    if (!deleting || saving) {
      return;
    }
    setSaving(true);
    const res = await network.deleteTagTemplate(deleting.id);
    setSaving(false);
    if (!res.success) {
      setError(res.message || t("Unknown error"));
      setMode("list");
      return;
    }
    await loadTemplates();
    setDeleting(null);
    setMode("list");
    setError(null);
  };

  const handleApply = async () => {
    if (!applying || saving) {
      return;
    }
    for (const param of applying.params) {
      if (!paramValues[param]?.trim()) {
        setError(t("Parameter cannot be empty"));
        return;
      }
    }
    setSaving(true);
    const res = await network.applyTagTemplate(applying.id, paramValues);
    setSaving(false);
    if (!res.success || !res.data) {
      setError(res.message || t("Failed to apply tag template"));
      return;
    }
    onAdded(res.data);
    closeManager();
  };

  const editorParsed = extractTagTemplateParams(content);
  const editorPreview = previewTagTemplate(content, {});
  const applyPreview = applying
    ? previewTagTemplate(applying.content, paramValues)
    : { tags: [] as string[] };

  return (
    <>
      <Button className="btn-soft btn-primary" type="button" onClick={openManager}>
        <span className="inline-flex items-center gap-1">
          <MdPlaylistAdd size={18} />
          {t("Template Tag List")}
        </span>
      </Button>
      {templates.map((template) => (
        <button
          key={template.id}
          type="button"
          className="btn btn-xs btn-outline"
          onClick={() => handleQuickApply(template)}
          title={template.content}
        >
          {template.name}
          {template.params.length > 0 ? ` (${template.params.length})` : ""}
        </button>
      ))}

      <dialog ref={dialogRef} className="modal">
        <div className="modal-box max-w-2xl w-full">
          {mode === "list" && (
            <>
              <h3 className="font-bold text-lg">{t("Template Tag List")}</h3>
              <p className="text-sm text-base-content/60 mt-1">
                {t("Use commas to separate tags, and {param} for parameters.")}
              </p>
              <p className="text-xs text-base-content/50 mt-1 font-mono">
                tag1,tag2,released-at-{"{released_time}"},tag4
              </p>
              {error && <ErrorAlert className="mt-3" message={error} />}
              <div className="mt-4 min-h-24 max-h-[50vh] overflow-y-auto flex flex-col gap-2">
                {loading ? (
                  <div className="flex items-center gap-2 py-6 justify-center">
                    <span className="loading loading-spinner loading-sm" />
                    <span>{t("Loading")}</span>
                  </div>
                ) : templates.length === 0 ? (
                  <p className="text-sm text-base-content/60 text-center py-6">
                    {t("No tag templates")}
                  </p>
                ) : (
                  templates.map((template) => (
                    <div
                      key={template.id}
                      className="rounded-box border border-base-200 px-3 py-2"
                    >
                      <div className="flex items-start gap-2">
                        <div className="flex-1 min-w-0">
                          <div className="font-medium text-sm">{template.name}</div>
                          <div className="text-xs text-base-content/60 font-mono break-all mt-0.5">
                            {template.content}
                          </div>
                          <div className="text-xs text-base-content/50 mt-1">
                            {template.params.length === 0
                              ? t("No parameters")
                              : `${t("Parameters")}: ${template.params.join(", ")}`}
                          </div>
                        </div>
                        <div className="flex shrink-0 gap-1">
                          <button
                            type="button"
                            className="btn btn-xs btn-primary"
                            onClick={() => handleQuickApply(template)}
                          >
                            {t("Apply")}
                          </button>
                          <button
                            type="button"
                            className="btn btn-xs btn-ghost btn-square"
                            onClick={() => openEdit(template)}
                          >
                            <MdEdit size={14} />
                          </button>
                          <button
                            type="button"
                            className="btn btn-xs btn-ghost btn-square text-error"
                            onClick={() => {
                              setDeleting(template);
                              setMode("confirm-delete");
                            }}
                          >
                            <MdDelete size={14} />
                          </button>
                        </div>
                      </div>
                    </div>
                  ))
                )}
              </div>
              <div className="modal-action">
                <Button className="btn" type="button" onClick={closeManager}>
                  {t("Cancel")}
                </Button>
                <Button className="btn-primary" type="button" onClick={openCreate}>
                  <span className="inline-flex items-center gap-1">
                    <MdAdd size={16} />
                    {t("New Template")}
                  </span>
                </Button>
              </div>
            </>
          )}

          {mode === "edit" && (
            <>
              <h3 className="font-bold text-lg">
                {editing ? t("Edit Template") : t("New Template")}
              </h3>
              <Input
                label={t("Template Name")}
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
              <TextArea
                label={t("Template Content")}
                value={content}
                onChange={(e) => setContent(e.target.value)}
                height={120}
              />
              <p className="text-xs text-base-content/60 mt-1">
                {t("Use commas to separate tags, and {param} for parameters.")}
              </p>
              {content.trim() && (
                <div className="mt-3">
                  <p className="text-xs font-medium mb-1">
                    {editorParsed.params.length === 0
                      ? t("No parameters")
                      : `${t("Parameters")}: ${editorParsed.params.join(", ")}`}
                  </p>
                  <div className="flex flex-wrap gap-1">
                    {(editorPreview.tags.length > 0
                      ? editorPreview.tags
                      : content
                          .split(",")
                          .map((part) => part.trim())
                          .filter(Boolean)
                    ).map((tag, index) => (
                      <span key={`${tag}-${index}`} className="badge badge-soft badge-primary">
                        {tag}
                      </span>
                    ))}
                  </div>
                </div>
              )}
              {error && <ErrorAlert className="mt-3" message={error} />}
              {editorParsed.error && content.trim() && (
                <ErrorAlert className="mt-3" message={t("Invalid tag template")} />
              )}
              <div className="modal-action">
                <Button
                  className="btn"
                  type="button"
                  onClick={() => {
                    setMode("list");
                    setError(null);
                  }}
                >
                  {t("Cancel")}
                </Button>
                <Button
                  className="btn-primary"
                  type="button"
                  isLoading={saving}
                  disabled={!name.trim() || !content.trim()}
                  onClick={handleSave}
                >
                  {t("Save")}
                </Button>
              </div>
            </>
          )}

          {mode === "apply" && applying && (
            <>
              <h3 className="font-bold text-lg">{applying.name}</h3>
              <p className="text-xs text-base-content/60 font-mono break-all mt-1">
                {applying.content}
              </p>
              {applying.params.length === 0 ? (
                <p className="text-sm text-base-content/60 mt-3">{t("No parameters")}</p>
              ) : (
                <div className="mt-3 flex flex-col gap-2">
                  <p className="text-sm">{t("Fill in template parameters")}</p>
                  {applying.params.map((param) => (
                    <label key={param} className="input w-full">
                      <span className="font-mono text-xs">{param}</span>
                      <input
                        type="text"
                        className="grow"
                        value={paramValues[param] ?? ""}
                        onChange={(e) =>
                          setParamValues((prev) => ({
                            ...prev,
                            [param]: e.target.value,
                          }))
                        }
                      />
                    </label>
                  ))}
                </div>
              )}
              {applyPreview.tags.length > 0 && (
                <div className="mt-3">
                  <p className="text-xs font-medium mb-1">{t("Preview")}</p>
                  <div className="flex flex-wrap gap-1">
                    {applyPreview.tags.map((tag) => (
                      <span key={tag} className="badge badge-primary">
                        {tag}
                      </span>
                    ))}
                  </div>
                </div>
              )}
              {error && <ErrorAlert className="mt-3" message={error} />}
              {applyPreview.error && applying.params.every((p) => paramValues[p]?.trim()) && (
                <ErrorAlert className="mt-3" message={applyPreview.error} />
              )}
              <div className="modal-action">
                <Button
                  className="btn"
                  type="button"
                  onClick={() => {
                    setMode("list");
                    setApplying(null);
                    setError(null);
                  }}
                >
                  {t("Cancel")}
                </Button>
                <Button
                  className="btn-primary"
                  type="button"
                  isLoading={saving}
                  disabled={applying.params.some((p) => !paramValues[p]?.trim())}
                  onClick={handleApply}
                >
                  {t("Insert Tags")}
                </Button>
              </div>
            </>
          )}

          {mode === "confirm-delete" && deleting && (
            <>
              <h3 className="font-bold text-lg">{t("Delete")}</h3>
              <p className="py-3">
                {t(
                  "Are you sure you want to delete this template? This action cannot be undone.",
                )}
              </p>
              <p className="text-sm font-medium">{deleting.name}</p>
              {error && <ErrorAlert className="mt-3" message={error} />}
              <div className="modal-action">
                <Button
                  className="btn"
                  type="button"
                  onClick={() => {
                    setDeleting(null);
                    setMode("list");
                    setError(null);
                  }}
                >
                  {t("Cancel")}
                </Button>
                <Button
                  className="btn-error"
                  type="button"
                  isLoading={saving}
                  onClick={handleDelete}
                >
                  {t("Delete")}
                </Button>
              </div>
            </>
          )}
        </div>
        <form method="dialog" className="modal-backdrop">
          <button>close</button>
        </form>
      </dialog>
    </>
  );
}
