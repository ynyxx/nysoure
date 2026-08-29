import { useState } from "react";
import type { CharacterParams, CharacterRole } from "../network/models";
import { network } from "../network/network";
import showToast from "./toast";
import { useTranslation } from "../hook/i18n";
import Button from "./button";
import { parseVndbId } from "../utils/vndb";

export default function CharacterEditor({
  character,
  setCharacter,
  onDelete,
}: {
  character: CharacterParams;
  setCharacter: (character: CharacterParams) => void;
  onDelete: () => void;
}) {
  const { t } = useTranslation();
  const [isUploading, setUploading] = useState(false);

  const uploadImage = async () => {
    const input = document.createElement("input");
    input.type = "file";
    input.accept = "image/*";
    input.onchange = async () => {
      if (!input.files || input.files.length === 0) {
        return;
      }
      setUploading(true);
      const file = input.files[0];
      const result = await network.uploadImage(file);
      setUploading(false);
      if (result.success) {
        setCharacter({
          ...character,
          image: result.data!,
        });
      } else {
        showToast({
          type: "error",
          message: `Failed to upload image`,
        });
      }
    };
    input.click();
  };

  return (
    <div className="h-52 shadow rounded-2xl overflow-clip flex bg-base-100">
      <div className="w-36 h-full cursor-pointer relative" onClick={uploadImage}>
        {isUploading ? (
          <div className="absolute inset-0 bg-black/50 flex items-center justify-center z-10">
            <span className="loading loading-spinner loading-lg text-white"></span>
          </div>
        ) : null}
        <img
          className="w-full h-full object-cover bg-base-200/80 hover:bg-base-200 transition-colors"
          src={character.image === 0 ? "/cp.webp" : network.getImageUrl(character.image)}
          alt={character.name}
        />
      </div>

      <div className="flex-1 p-4 flex flex-col gap-2">
        <div className="flex gap-2 items-center">
          <input
            type="text"
            className="input input-sm input-bordered flex-1"
            placeholder={t("Name")}
            value={character.name}
            onChange={(e) => setCharacter({ ...character, name: e.target.value })}
          />
          <button className="btn btn-sm btn-error btn-square" onClick={onDelete}>
            <svg
              xmlns="http://www.w3.org/2000/svg"
              className="h-4 w-4"
              fill="none"
              viewBox="0 0 24 24"
              stroke="currentColor"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <input
          type="text"
          className="input input-sm input-bordered"
          placeholder="CV"
          value={character.cv}
          onChange={(e) => setCharacter({ ...character, cv: e.target.value })}
        />

        <select
          className="select select-sm select-bordered"
          value={character.role}
          onChange={(e) => setCharacter({ ...character, role: e.target.value as CharacterRole })}
        >
          <option value="primary">{t("Primary Role")}</option>
          <option value="side">{t("Side Role")}</option>
        </select>

        <div className="flex-1">
          <textarea
            className="textarea textarea-bordered w-full h-full resize-none text-xs"
            placeholder={t("Aliases (one per line)")}
            value={character.alias.join("\n")}
            onChange={(e) =>
              setCharacter({
                ...character,
                alias: e.target.value.split("\n").filter((line) => line.trim() !== ""),
              })
            }
          />
        </div>
      </div>
    </div>
  );
}

export function FetchVndbCharactersButton({
  vnID,
  onFetch,
}: {
  vnID: string;
  onFetch: (characters: CharacterParams[], releaseDate?: string) => void;
}) {
  const { t } = useTranslation();
  const [isFetching, setFetching] = useState(false);
  const fetchCharacters = async () => {
    const parsedVnID = parseVndbId(vnID);
    if (!parsedVnID) {
      showToast({
        type: "error",
        message: t("Invalid VNDB ID format"),
      });
      return;
    }
    setFetching(true);
    const res = await network.getInfoFromVNDB(parsedVnID);
    setFetching(false);
    if (res.success && res.data) {
      onFetch(res.data.characters ?? [], res.data.release_date);
    } else {
      showToast({
        type: "error",
        message: t("Failed to fetch characters from VNDB"),
      });
    }
  };

  return (
    <Button isLoading={isFetching} onClick={fetchCharacters}>
      {t("Fetch from VNDB")}
    </Button>
  );
}
