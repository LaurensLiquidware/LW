-- The MSP operator's own company name/logo, shown alongside Liquidware's
-- existing branding on PDF reports (see internal/reportpdf.Branding) --
-- follow-up to 0005_runtime_settings.sql, same singleton-row pattern.
ALTER TABLE runtime_settings ADD COLUMN company_name TEXT NOT NULL DEFAULT '';
ALTER TABLE runtime_settings ADD COLUMN company_logo_image BLOB;
ALTER TABLE runtime_settings ADD COLUMN company_logo_image_type TEXT NOT NULL DEFAULT '';
