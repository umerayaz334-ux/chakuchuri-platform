import { useEffect, useState } from "react";
import { apiBlob } from "./api";
import type { User } from "./types";

export function ProfileAvatar({ user, token, className = "" }: { user: Pick<User, "name" | "profileImageFileId">; token: string; className?: string }) {
  const [src, setSrc] = useState("");

  useEffect(() => {
    if (!user.profileImageFileId) {
      setSrc("");
      return;
    }
    let active = true;
    let objectUrl = "";
    apiBlob(`/api/files/${user.profileImageFileId}/thumbnail`, token)
      .catch(() => apiBlob(`/api/files/${user.profileImageFileId}/content`, token))
      .then((blob) => {
        if (!active || !blob.type.startsWith("image/")) return;
        objectUrl = URL.createObjectURL(blob);
        setSrc(objectUrl);
      })
      .catch(() => setSrc(""));
    return () => {
      active = false;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [token, user.profileImageFileId]);

  return (
    <span className={`cc-profile-avatar ${className}`.trim()}>
      {src ? <img alt={`${user.name} profile`} src={src} /> : initials(user.name)}
    </span>
  );
}

function initials(value: string) {
  return value.trim().split(/\s+/).slice(0, 2).map((part) => part[0]?.toUpperCase() || "").join("") || "CC";
}
