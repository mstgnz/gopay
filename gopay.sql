-- -------------------------------------------------------------
-- TablePlus 6.6.4(624)
--
-- https://tableplus.com/
--
-- Database: gopay
-- Generation Time: 2025-07-07 01:54:48.5600
-- -------------------------------------------------------------


-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS providers_id_seq;

-- Table Definition
CREATE TABLE "public"."providers" (
    "id" int4 NOT NULL DEFAULT nextval('providers_id_seq'::regclass),
    "name" varchar NOT NULL,
    PRIMARY KEY ("id")
);

-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS tenants_id_seq;

-- Table Definition
CREATE TABLE "public"."tenants" (
    "id" int4 NOT NULL DEFAULT nextval('tenants_id_seq'::regclass),
    "username" varchar NOT NULL,
    "password" varchar NOT NULL,
    "last_login" timestamp,
    "created_at" timestamp DEFAULT now(),
    "code" varchar,
    PRIMARY KEY ("id")
);

-- Column Comment
COMMENT ON COLUMN "public"."tenants"."code" IS 'şifre unuttum veya sms kod';

-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS paycell_id_seq;

-- Table Definition
CREATE TABLE "public"."paycell" (
    "id" int8 NOT NULL DEFAULT nextval('paycell_id_seq'::regclass),
    "tenant_id" int4 NOT NULL,
    "request" jsonb,
    "response" jsonb,
    "request_at" timestamp DEFAULT now(),
    "response_at" timestamp,
    PRIMARY KEY ("id")
);

-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS iyzico_id_seq;

-- Table Definition
CREATE TABLE "public"."iyzico" (
    "id" int8 NOT NULL DEFAULT nextval('iyzico_id_seq'::regclass),
    "tenant_id" int4 NOT NULL,
    "request" jsonb,
    "response" jsonb,
    "request_at" timestamp DEFAULT now(),
    "response_at" timestamp,
    PRIMARY KEY ("id")
);

-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS ozanpay_id_seq;

-- Table Definition
CREATE TABLE "public"."ozanpay" (
    "id" int8 NOT NULL DEFAULT nextval('ozanpay_id_seq'::regclass),
    "tenant_id" int4 NOT NULL,
    "request" jsonb,
    "response" jsonb,
    "request_at" timestamp DEFAULT now(),
    "response_at" timestamp,
    PRIMARY KEY ("id")
);

-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS stripe_id_seq;

-- Table Definition
CREATE TABLE "public"."stripe" (
    "id" int8 NOT NULL DEFAULT nextval('stripe_id_seq'::regclass),
    "tenant_id" int4 NOT NULL,
    "request" jsonb,
    "response" jsonb,
    "request_at" timestamp DEFAULT now(),
    "response_at" timestamp,
    PRIMARY KEY ("id")
);

-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS papara_id_seq;

-- Table Definition
CREATE TABLE "public"."papara" (
    "id" int4 NOT NULL DEFAULT nextval('papara_id_seq'::regclass),
    "tenant_id" int4 NOT NULL,
    "request" jsonb,
    "response" jsonb,
    "request_at" timestamp DEFAULT now(),
    "response_at" timestamp,
    PRIMARY KEY ("id")
);

-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS nkolay_id_seq;

-- Table Definition
CREATE TABLE "public"."nkolay" (
    "id" int4 NOT NULL DEFAULT nextval('nkolay_id_seq'::regclass),
    "tenant_id" int4 NOT NULL,
    "request" jsonb,
    "response" jsonb,
    "request_at" timestamp DEFAULT now(),
    "response_at" timestamp,
    PRIMARY KEY ("id")
);

-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS paytr_id_seq;

-- Table Definition
CREATE TABLE "public"."paytr" (
    "id" int4 NOT NULL DEFAULT nextval('paytr_id_seq'::regclass),
    "tenant_id" int4 NOT NULL,
    "request" jsonb,
    "response" jsonb,
    "request_at" timestamp DEFAULT now(),
    "response_at" timestamp,
    PRIMARY KEY ("id")
);

-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS payu_id_seq;

-- Table Definition
CREATE TABLE "public"."payu" (
    "id" int4 NOT NULL DEFAULT nextval('payu_id_seq'::regclass),
    "tenant_id" int4 NOT NULL,
    "request" jsonb,
    "response" jsonb,
    "request_at" timestamp DEFAULT now(),
    "response_at" timestamp,
    PRIMARY KEY ("id")
);

-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS shopier_id_seq;

-- Table Definition
CREATE TABLE "public"."shopier" (
    "id" int4 NOT NULL DEFAULT nextval('shopier_id_seq'::regclass),
    "tenant_id" int4 NOT NULL,
    "request" jsonb,
    "response" jsonb,
    "request_at" timestamp DEFAULT now(),
    "response_at" timestamp,
    PRIMARY KEY ("id")
);

-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS system_logs_id_seq;

-- Table Definition
CREATE TABLE "public"."system_logs" (
    "id" int8 NOT NULL DEFAULT nextval('system_logs_id_seq'::regclass),
    "level" varchar,
    "log" jsonb,
    "created_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id")
);

-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS tenant_configs_id_seq;

-- Table Definition
CREATE TABLE "public"."tenant_configs" (
    "id" int4 NOT NULL DEFAULT nextval('tenant_configs_id_seq'::regclass),
    "tenant_id" varchar NOT NULL,
    "provider_name" varchar NOT NULL,
    "config_data" jsonb NOT NULL,
    "created_at" timestamp DEFAULT now(),
    "updated_at" timestamp DEFAULT now(),
    PRIMARY KEY ("id")
);

-- Create unique index for tenant_id and provider_name combination
CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_configs_unique ON "public"."tenant_configs" ("tenant_id", "provider_name");

-- Create index for faster queries
CREATE INDEX IF NOT EXISTS idx_tenant_configs_tenant_id ON "public"."tenant_configs" ("tenant_id");
CREATE INDEX IF NOT EXISTS idx_tenant_configs_provider_name ON "public"."tenant_configs" ("provider_name");

ALTER TABLE "public"."paycell" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");
ALTER TABLE "public"."iyzico" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");
ALTER TABLE "public"."ozanpay" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");
ALTER TABLE "public"."stripe" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");
ALTER TABLE "public"."papara" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");
ALTER TABLE "public"."nkolay" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");
ALTER TABLE "public"."paytr" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");
ALTER TABLE "public"."payu" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");
ALTER TABLE "public"."shopier" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");
