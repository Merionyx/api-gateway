--- Create main control plane schema
CREATE SCHEMA IF NOT EXISTS control_plane;

CREATE TABLE "control_plane"."tenants" (
    "uuid" UUID NOT NULL,
    "name" VARCHAR(1024) NOT NULL DEFAULT NULL,
    PRIMARY KEY ("uuid")
);

CREATE TABLE "control_plane"."environments" (
    "uuid" UUID NOT NULL,
    "name" VARCHAR(1024) NOT NULL DEFAULT NULL,
    "config" JSONB NOT NULL DEFAULT NULL,
    PRIMARY KEY ("uuid")
);

CREATE TABLE "control_plane"."tenants_environments" (
    "tenant_uuid" UUID NOT NULL,
    "environmet_uuid" UUID NOT NULL DEFAULT NULL,
    PRIMARY KEY ("tenant_uuid", "environmet_uuid")
);

CREATE TABLE "control_plane"."listeners" (
    "uuid" UUID NOT NULL,
    "name" VARCHAR(1024) NOT NULL DEFAULT NULL,
    "config" JSONB NOT NULL DEFAULT NULL,
    PRIMARY KEY ("uuid")
);

CREATE TABLE "control_plane"."listeners_environments" (
    "listener_uuid" UUID NOT NULL,
    "environmet_uuid" UUID NOT NULL DEFAULT NULL,
    PRIMARY KEY ("listener_uuid", "environmet_uuid")
);