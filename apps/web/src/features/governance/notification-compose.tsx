import { Link, useNavigate } from "react-router";
import { useTranslation } from "react-i18next";
import { ArrowLeft } from "lucide-react";
import { PageHeading } from "@/features/administration/shared";
import { buttonVariants } from "@/components/ui/button";
import { useSession } from "@/features/administration/auth";
import { NoAccess } from "./shared";
import { NotificationComposer } from "./notification-composer";

export function NotificationComposePage() {
  const { t } = useTranslation("governance");
  const session = useSession();
  const navigate = useNavigate();
  if (!session.permissions.includes("notifications:send")) return <NoAccess />;
  return (
    <>
      <PageHeading title={t("composeNotification")} subtitle={t("composeHint")}>
        <Link
          to="/notifications"
          className={buttonVariants({ variant: "outline" })}
          data-slot="button"
        >
          <ArrowLeft size={16} aria-hidden="true" />
          {t("backToCenter")}
        </Link>
      </PageHeading>
      <NotificationComposer
        onSent={() => navigate("/notifications?tab=sent", { replace: true })}
      />
    </>
  );
}
