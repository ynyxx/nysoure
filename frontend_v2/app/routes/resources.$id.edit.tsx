import type { Route } from "./+types/resources.$id.edit";
import { network } from "../network/network";
import { data, useNavigate } from "react-router";
import { useTranslation } from "../hook/i18n";
import { ErrorAlert } from "../components/alert";
import ResourceForm, { type RelationFormItem, type ResourceFormData } from "../components/resource_form";
import { configFromMatches, useConfig } from "../hook/config";

export function meta({ matches }: Route.MetaArgs) {
  const config = configFromMatches(matches);
  return [
    { title: `Edit Resource - ${config.server_name}` },
    { name: "description", content: config.site_description },
  ];
}

export async function loader({ params, request }: Route.LoaderArgs) {
  const id = parseInt(params.id);
  if (isNaN(id)) {
    throw data("Not Found", { status: 404 });
  }

  const cookie = request.headers.get("Cookie");
  const res = await network.getResourceDetails(id, cookie || undefined);
  if (!res.success || !res.data) {
    if (res.status === 404 || (res.message ?? "").includes("Resource not found")) {
      throw data("Not Found", { status: 404 });
    }
    throw new Error(res.message || "Failed to load resource");
  }

  return {
    resourceId: id,
    resource: res.data,
  };
}

export default function EditResource({ loaderData }: Route.ComponentProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const config = useConfig();
  const { resourceId, resource } = loaderData;

  const initialData: ResourceFormData = {
    title: resource.title,
    altTitles: resource.alternativeTitles ?? [],
    releaseDate: resource.releaseDate?.split("T")[0] ?? undefined,
    skipUpdateTime: false,
    tags: resource.tags,
    article: resource.article,
    images: resource.images.map((i) => i.id),
    coverId: resource.coverId,
    links: resource.links ?? [],
    galleryImages: resource.gallery ?? [],
    galleryNsfw: resource.galleryNsfw ?? [],
    characters: resource.characters ?? [],
    relations: (resource.relations ?? []).map((rel): RelationFormItem => ({
      toId: rel.resource.id,
      toTitle: rel.resource.title,
      description: rel.description,
    })),
  };

  const handleSubmit = async (data: ResourceFormData) => {
    const res = await network.editResource(resourceId, {
      title: data.title,
      alternative_titles: data.altTitles,
      tags: data.tags.map((tag) => tag.id),
      article: data.article,
      skip_update_time: data.skipUpdateTime,
      images: data.images,
      cover_id: data.coverId,
      links: data.links,
      gallery: data.galleryImages,
      gallery_nsfw: data.galleryNsfw,
      characters: data.characters,
      release_date: data.releaseDate,
      relations: data.relations.map((r) => ({ to_id: r.toId, description: r.description })),
    });
    if (res.success) {
      navigate("/resources/" + resourceId.toString(), { replace: true });
    } else {
      throw new Error(res.message);
    }
  };

  if (!config.isLoggedIn || !config.user) {
    return (
      <ErrorAlert
        className={"m-4"}
        message={t("You are not logged in. Please log in to access this page.")}
      />
    );
  }

  return (
    <ResourceForm
      initialData={initialData}
      onSubmit={handleSubmit}
      submitButtonText={t("Publish")}
      title={t("Edit Resource")}
      excludeId={resourceId}
      showSkipUpdateTimeOption
    />
  );
}
