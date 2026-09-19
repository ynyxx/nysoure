import type { Resource } from "../network/models.ts";
import { network } from "../network/network.ts";
import { useNavigate } from "react-router";
import Badge from "~/components/badge.tsx";
import React from "react";

export default function ResourceCard({
  resource,
  action,
}: {
  resource: Resource;
  action?: React.ReactNode;
}) {
  const navigate = useNavigate();

  let tags = resource.tags;
  if (tags.length > 10) {
    tags = tags.slice(0, 10);
  }

  return (
    <a
      href={`/resources/${resource.id}`}
      className={"cursor-pointer block"}
      onClick={(e) => {
        e.preventDefault();
        navigate(`/resources/${resource.id}`);
      }}
    >
      <div
        className={
          "card shadow hover:shadow-md transition-shadow bg-base-100/40 backdrop-blur-xs"
        }
      >
        {resource.image != null && (
          <figure>
            <img
              src={network.getResampledImageUrl(resource.image.id)}
              alt="cover"
              style={{
                width: "100%",
                aspectRatio: resource.image.width / resource.image.height,
              }}
            />
          </figure>
        )}
        <div className="flex flex-col p-4">
          <h2 className="card-title break-all">{resource.title}</h2>
          {resource.subtitle && (
            <p className="text-sm text-base-content/60 break-all mt-0.5">
              {resource.subtitle}
            </p>
          )}
          <div className="h-2"></div>
          <p>
            {tags.map((tag) => {
              return (
                <Badge key={tag.id} className={"m-0.5"}>
                  {tag.name}
                </Badge>
              );
            })}
          </p>
          <div className="h-2"></div>
          <div className="flex items-center">
            <div className="avatar">
              <div className="w-6 rounded-full">
                <img src={network.getUserAvatar(resource.author)} />
              </div>
            </div>
            <div className="w-2"></div>
            <div className="text-sm">{resource.author.username}</div>
            <div className="flex-1"></div>
            {action}
          </div>
        </div>
      </div>
    </a>
  );
}
