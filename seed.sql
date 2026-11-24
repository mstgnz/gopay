-- Reference data: the providers the application implements. Load after gopay.sql into a new
-- database. Each name is a provider package and its log table; the application serves only active ones.
INSERT INTO "public"."providers" ("name", "active") VALUES
    ('iyzico', true),
    ('ozanpay', true),
    ('stripe', true),
    ('paycell', true),
    ('papara', true),
    ('paytr', true),
    ('payu', true),
    ('nkolay', true),
    ('shopier', false),
    ('payten', true),
    ('ziraat', true),
    ('akbank', true),
    ('yapikredi', true);
