import "./drive.css";
import { Files, Trash2, Users } from "lucide-react";
import { Link } from "react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/components/ui/button";
import { type DriveSpace } from "@/lib/capabilities";
export function SpaceNavigation({
  space,
  current,
}: {
  space: DriveSpace;
  current: "files" | "members" | "trash";
}) {
  const { t } = useTranslation("drive");
  return (
    <nav
      aria-label={space.kind === "personal" ? t("personal") : space.name}
      className="drive-space-navigation"
    >
      <Button
        variant={current === "files" ? "secondary" : "ghost"}
        render={
          <Link
            to={`/drive/${space.id}`}
            aria-current={current === "files" ? "page" : undefined}
          />
        }
      >
        <Files />
        {t("file")}
      </Button>
      {space.kind === "shared" && space.role === "admin" && (
        <Button
          variant={current === "members" ? "secondary" : "ghost"}
          render={
            <Link
              to={`/drive/${space.id}/members`}
              aria-current={current === "members" ? "page" : undefined}
            />
          }
        >
          <Users />
          {t("members")}
        </Button>
      )}
      {space.role !== "viewer" && (
        <Button
          variant={current === "trash" ? "secondary" : "ghost"}
          render={
            <Link
              to={`/drive/${space.id}/trash`}
              aria-current={current === "trash" ? "page" : undefined}
            />
          }
        >
          <Trash2 />
          {t("trashTitle")}
        </Button>
      )}
    </nav>
  );
}
