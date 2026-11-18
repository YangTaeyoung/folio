import { Cloud, HardDrive, Network } from "lucide-react";
import "./ServiceIcon.css";

type ServiceAsset = {
  name: string;
  src?: string;
  plate?: boolean;
  generic?: "cloud" | "disk" | "dns";
};

/** Local identity assets. Provenance and hashes: docs/design/service-icons.md. */
export const SERVICE_ICONS = {
  aws: { name: "Amazon S3", src: "/services/aws.svg", plate: true },
  r2: { name: "Cloudflare R2", src: "/services/r2.svg", plate: true },
  minio: { name: "MinIO", src: "/services/minio.png" },
  gcs: { name: "Google Cloud Storage", src: "/services/gcs.svg" },
  azure: { name: "Azure Blob Storage", src: "/services/azure.svg" },
  cloudflare: { name: "Cloudflare", src: "/services/cloudflare.svg" },
  google: { name: "Google", src: "/images/google-signin-icon.png" },
  codex: { name: "Codex", src: "/services/codex.svg", plate: true },
  claudecode: { name: "Claude Code", src: "/services/claudecode.svg" },
  antigravity: { name: "Antigravity CLI", src: "/services/antigravity.svg" },
  geminicli: {
    name: "Gemini CLI",
    src: "/services/geminicli.svg",
    plate: true,
  },
  vscode: { name: "Visual Studio Code", src: "/services/vscode.svg" },
  copilot: {
    name: "GitHub Copilot",
    src: "/services/copilot.svg",
    plate: true,
  },
  cursor: { name: "Cursor", src: "/services/cursor.svg", plate: true },
  opencode: { name: "OpenCode", src: "/services/opencode.svg", plate: true },
  mcp: {
    name: "Model Context Protocol",
    src: "/services/mcp.svg",
    plate: true,
  },
  local: { name: "Local storage", generic: "disk" },
  generic: { name: "Object storage", generic: "cloud" },
  dns: { name: "DNS", generic: "dns" },
  // A neutral DNS symbol, deliberately not a fabricated Domain Connect logo.
  domainconnect: { name: "Domain Connect", generic: "dns" },
} as const satisfies Record<string, ServiceAsset>;

export type ServiceId = keyof typeof SERVICE_ICONS;

export type ServiceIconProps = {
  service: ServiceId;
  size?: number;
  /** Omit beside visible text; supply only when the icon stands alone. */
  label?: string;
  className?: string;
};

export function ServiceIcon({
  service,
  size = 20,
  label,
  className = "",
}: ServiceIconProps) {
  const asset: ServiceAsset = SERVICE_ICONS[service];
  const GenericIcon =
    asset.generic === "disk"
      ? HardDrive
      : asset.generic === "dns"
        ? Network
        : Cloud;
  return (
    <span
      className={`service-icon${asset.plate ? " service-icon--plate" : ""} ${className}`.trim()}
      style={{ width: size, height: size }}
      role={label ? "img" : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
      data-service={service}
    >
      {asset.src ? (
        <img
          src={asset.src}
          width={size}
          height={size}
          alt=""
          draggable={false}
        />
      ) : (
        <GenericIcon size={size} aria-hidden="true" strokeWidth={1.75} />
      )}
    </span>
  );
}

type StorageIdentity = { provider: string; endpoint?: string; name?: string };

/** Identify vendors from explicit provider IDs or known hosts, never display names. */
export function resolveStorageService({
  provider,
  endpoint = "",
}: StorageIdentity): ServiceId {
  switch (provider.toLowerCase()) {
    case "local":
      return "local";
    case "gcs":
    case "google":
    case "google-cloud-storage":
      return "gcs";
    case "azure":
    case "azure-blob":
      return "azure";
    case "aws":
    case "amazon-s3":
      return "aws";
    case "r2":
    case "cloudflare-r2":
      return "r2";
    case "minio":
      return "minio";
    case "s3":
      break;
    default:
      return "generic";
  }
  let host: string;
  try {
    host = new URL(
      endpoint.includes("://") ? endpoint : `https://${endpoint}`,
    ).hostname.toLowerCase();
  } catch {
    return "generic";
  }
  if (/^[a-z0-9]+(?:\.(?:eu|fedramp))?\.r2\.cloudflarestorage\.com$/.test(host))
    return "r2";
  if (/^(?:.+\.)?s3(?:[.-][a-z0-9-]+)*\.amazonaws\.com(?:\.cn)?$/.test(host))
    return "aws";
  if (host === "play.min.io") return "minio";
  return "generic";
}
