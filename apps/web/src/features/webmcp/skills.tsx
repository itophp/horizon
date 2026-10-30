import { useTranslation } from "react-i18next";
import { BookOpen, ShieldCheck, Workflow } from "lucide-react";
import "./webmcp.css";
const pageSkills = ["navigate", "accounts", "diagnose"] as const;
export function WebSkillsSection() {
  const { t } = useTranslation("webmcp");
  return (
    <div className="webmcp-stack">
      <p className="muted-text">{t("skillsIntro")}</p>
      <section className="admin-panel webmcp-section">
        <div className="webmcp-section-heading">
          <Workflow size={18} />
          <h2>{t("skills")}</h2>
          <span className="neutral-badge">{t("skillsCount")}</span>
        </div>
        <ul className="webmcp-skills">
          {pageSkills.map((skill) => (
            <li key={skill}>
              <strong>{t(`skillNames.${skill}`)}</strong>
              <span>{t(`skillHints.${skill}`)}</span>
              <code>{`webskills/baseline-web-${skill}`}</code>
            </li>
          ))}
        </ul>
        <p className="muted-text">{t("skillsInstall")}</p>
      </section>
      <section className="admin-panel webmcp-section">
        <div className="webmcp-section-heading">
          <BookOpen size={18} />
          <h2>{t("businessSkill")}</h2>
        </div>
        <p className="muted-text">{t("businessSkillHint")}</p>
        <pre className="webmcp-code">
          <code>{t("businessSkillPath")}</code>
        </pre>
      </section>
      <section className="admin-panel webmcp-section">
        <div className="webmcp-section-heading">
          <ShieldCheck size={18} />
          <h2>{t("safetyTitle")}</h2>
        </div>
        <p className="muted-text">{t("safety")}</p>
      </section>
    </div>
  );
}
