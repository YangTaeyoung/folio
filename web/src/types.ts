export type Visibility = "private" | "organization" | "selected" | "public";
export interface User {
  id: string;
  email: string;
  name: string;
  role: "admin" | "member";
  disabled: boolean;
  mustChangePassword: boolean;
  loginMethod?: "password" | "google";
  googleLinked?: boolean;
  createdAt: string;
}
export interface Organization {
  id: string;
  name: string;
}
export interface Artifact {
  id: string;
  slug: string;
  title: string;
  description: string;
  ownerId: string;
  ownerName: string;
  visibility: Visibility;
  memberIds: string[];
  archived: boolean;
  expiresAt: string | null;
  createdAt: string;
  updatedAt: string;
  sizeBytes: number;
  currentVersionId: string;
  shareUrl: string;
  kind: string;
}
export interface Version {
  id: string;
  artifactId: string;
  number: number;
  createdAt: string;
  sizeBytes: number;
  backendId: string;
  files: { path: string; mimeType: string; size: number }[];
  message: string;
}
export interface Policy {
  quotaBytes: number;
  maxUploadBytes: number;
  retentionDays: number;
  overflowPolicy: "reject" | "delete_oldest";
}
export interface Backend {
  id: string;
  name: string;
  provider: "local" | "s3" | "gcs" | "azure";
  bucket: string;
  region: string;
  endpoint: string;
  prefix: string;
  accountName: string;
  credentialsConfigured: boolean;
  isDefault: boolean;
  createdAt: string;
}
export interface Domain {
  id: string;
  hostname: string;
  artifactId: string;
  provider: string;
  status: string;
  records: { type: string; name: string; value: string }[];
  lastError: string;
  verifiedAt: string;
}
export interface Audit {
  id: string;
  actorId: string;
  actorName: string;
  action: string;
  targetId: string;
  createdAt: string;
  detail: string;
}
export interface Token {
  id: string;
  name: string;
  createdAt: string;
  expiresAt: string | null;
  lastUsedAt: string | null;
  scopes: string[];
}
export interface Session {
  user: User;
  csrfToken: string;
  organization: Organization;
}
