-- -------------------------------------------------------------
-- TablePlus 6.6.4(624)
--
-- https://tableplus.com/
--
-- Database: gopay
-- Generation Time: 2025-07-07 10:09:24.4920
-- -------------------------------------------------------------


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
CREATE SEQUENCE IF NOT EXISTS iyzico_id_seq;

-- Table Definition
CREATE TABLE "public"."iyzico" (
    "id" int8 NOT NULL DEFAULT nextval('iyzico_id_seq'::regclass),
    "tenant_id" int4 NOT NULL,
    "request" jsonb,
    "response" jsonb,
    "request_at" timestamp DEFAULT now(),
    "response_at" timestamp,
    "method" varchar(50),
    "endpoint" varchar(255),
    "request_id" varchar(100),
    "payment_id" varchar(100),
    "transaction_id" varchar(100),
    "amount" numeric(15,2),
    "currency" varchar(3),
    "status" varchar(50),
    "error_code" varchar(50),
    "processing_ms" int8,
    "user_agent" varchar(500),
    "client_ip" varchar(45),
    PRIMARY KEY ("id")
);

-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS providers_id_seq;

-- Table Definition
CREATE TABLE "public"."providers" (
    "id" int2 NOT NULL DEFAULT nextval('providers_id_seq'::regclass),
    "name" varchar NOT NULL,
    "active" boolean NOT NULL DEFAULT false,
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
    "method" varchar(50),
    "endpoint" varchar(255),
    "request_id" varchar(100),
    "payment_id" varchar(100),
    "transaction_id" varchar(100),
    "amount" numeric(15,2),
    "currency" varchar(3),
    "status" varchar(50),
    "error_code" varchar(50),
    "processing_ms" int8,
    "user_agent" varchar(500),
    "client_ip" varchar(45),
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
    "method" varchar(50),
    "endpoint" varchar(255),
    "request_id" varchar(100),
    "payment_id" varchar(100),
    "transaction_id" varchar(100),
    "amount" numeric(15,2),
    "currency" varchar(3),
    "status" varchar(50),
    "error_code" varchar(50),
    "processing_ms" int8,
    "user_agent" varchar(500),
    "client_ip" varchar(45),
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
CREATE SEQUENCE IF NOT EXISTS nkolay_id_seq;

-- Table Definition
CREATE TABLE "public"."nkolay" (
    "id" int4 NOT NULL DEFAULT nextval('nkolay_id_seq'::regclass),
    "tenant_id" int4 NOT NULL,
    "request" jsonb,
    "response" jsonb,
    "request_at" timestamp DEFAULT now(),
    "response_at" timestamp,
    "method" varchar(50),
    "endpoint" varchar(255),
    "request_id" varchar(100),
    "payment_id" varchar(100),
    "transaction_id" varchar(100),
    "amount" numeric(15,2),
    "currency" varchar(3),
    "status" varchar(50),
    "error_code" varchar(50),
    "processing_ms" int8,
    "user_agent" varchar(500),
    "client_ip" varchar(45),
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
    "method" varchar(50),
    "endpoint" varchar(255),
    "request_id" varchar(100),
    "payment_id" varchar(100),
    "transaction_id" varchar(100),
    "amount" numeric(15,2),
    "currency" varchar(3),
    "status" varchar(50),
    "error_code" varchar(50),
    "processing_ms" int8,
    "user_agent" varchar(500),
    "client_ip" varchar(45),
    PRIMARY KEY ("id")
);

-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS tenant_configs_id_seq;

-- Table Definition
CREATE TABLE "public"."tenant_configs" (
    "id" int4 NOT NULL DEFAULT nextval('tenant_configs_id_seq'::regclass),
    "tenant_id" int4 NOT NULL,
    "provider_id" int2 NOT NULL,
    "environment" varchar NOT NULL CHECK ((environment)::text = ANY ((ARRAY['sandbox'::character varying, 'production'::character varying])::text[])),
    "key" varchar NOT NULL,
    "value" varchar NOT NULL,
    "created_at" timestamp DEFAULT now(),
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
    "method" varchar(50),
    "endpoint" varchar(255),
    "request_id" varchar(100),
    "payment_id" varchar(100),
    "transaction_id" varchar(100),
    "amount" numeric(15,2),
    "currency" varchar(3),
    "status" varchar(50),
    "error_code" varchar(50),
    "processing_ms" int8,
    "user_agent" varchar(500),
    "client_ip" varchar(45),
    PRIMARY KEY ("id")
);

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
    "method" varchar(50),
    "endpoint" varchar(255),
    "request_id" varchar(100),
    "payment_id" varchar(100),
    "transaction_id" varchar(100),
    "amount" numeric(15,2),
    "currency" varchar(3),
    "status" varchar(50),
    "error_code" varchar(50),
    "processing_ms" int8,
    "user_agent" varchar(500),
    "client_ip" varchar(45),
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
    "method" varchar(50),
    "endpoint" varchar(255),
    "request_id" varchar(100),
    "payment_id" varchar(100),
    "transaction_id" varchar(100),
    "amount" numeric(15,2),
    "currency" varchar(3),
    "status" varchar(50),
    "error_code" varchar(50),
    "processing_ms" int8,
    "user_agent" varchar(500),
    "client_ip" varchar(45),
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
    "method" varchar(50),
    "endpoint" varchar(255),
    "request_id" varchar(100),
    "payment_id" varchar(100),
    "transaction_id" varchar(100),
    "amount" numeric(15,2),
    "currency" varchar(3),
    "status" varchar(50),
    "error_code" varchar(50),
    "processing_ms" int8,
    "user_agent" varchar(500),
    "client_ip" varchar(45),
    PRIMARY KEY ("id")
);

-- Sequence and defined type
CREATE SEQUENCE IF NOT EXISTS callbacks_id_seq;

-- Table Definition
CREATE TABLE "public"."callbacks" (
    "id" int4 NOT NULL DEFAULT nextval('callbacks_id_seq'::regclass),
    "tenant_id" int4 NOT NULL,
    "provider" varchar(50) NOT NULL,
    "payment_id" varchar(100) NOT NULL,
    "original_callback" varchar(1000),
    "amount" numeric(15,2),
    "currency" varchar(3),
    "conversation_id" varchar(100),
    "log_id" int8,
    "environment" varchar(20),
    "client_ip" varchar(45),
    "state_data" jsonb NOT NULL,
    "created_at" timestamp DEFAULT now(),
    "expires_at" timestamp NOT NULL,
    "used" boolean DEFAULT false,
    PRIMARY KEY ("id")
);

-- Create index for cleanup queries
CREATE INDEX callbacks_tenant_id ON public.callbacks USING btree (tenant_id);
ALTER TABLE "public"."callbacks" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");

INSERT INTO "public"."providers" ("name", "active") VALUES
('iyzico', true),
('ozanpay', true),
('strpie', true),
('paycell', true),
('papara', true),
('paytr', true),
('payu', true),
('nkolay', true),
('shopier', false);

ALTER TABLE "public"."iyzico" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");


-- Indices
CREATE INDEX iyzico_tenant_id ON public.iyzico USING btree (tenant_id);
ALTER TABLE "public"."stripe" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");


-- Indices
CREATE INDEX stripe_tenant_id ON public.stripe USING btree (tenant_id);
ALTER TABLE "public"."shopier" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");


-- Indices
CREATE INDEX shopier_tenant_id ON public.shopier USING btree (tenant_id);
ALTER TABLE "public"."nkolay" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");


-- Indices
CREATE INDEX nkolay_tenant_id ON public.nkolay USING btree (tenant_id);
ALTER TABLE "public"."ozanpay" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");


-- Indices
CREATE INDEX ozanpay_tenant_id ON public.ozanpay USING btree (tenant_id);
ALTER TABLE "public"."tenant_configs" ADD FOREIGN KEY ("provider_id") REFERENCES "public"."providers"("id");
ALTER TABLE "public"."tenant_configs" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");


-- Indices
CREATE INDEX tc_tenant_id ON public.tenant_configs USING btree (tenant_id);
CREATE INDEX tc_provider_id ON public.tenant_configs USING btree (provider_id);
CREATE INDEX configs_tenant_id ON public.tenant_configs USING btree (tenant_id);
CREATE INDEX configs_provider_id ON public.tenant_configs USING btree (provider_id);
ALTER TABLE "public"."papara" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");


-- Indices
CREATE INDEX papara_tenant_id ON public.papara USING btree (tenant_id);
ALTER TABLE "public"."paycell" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");
ALTER TABLE "public"."paytr" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");


-- Indices
CREATE INDEX paytr_tenant_id ON public.paytr USING btree (tenant_id);
ALTER TABLE "public"."payu" ADD FOREIGN KEY ("tenant_id") REFERENCES "public"."tenants"("id");


-- Indices
CREATE INDEX payu_tenant_id ON public.payu USING btree (tenant_id);
