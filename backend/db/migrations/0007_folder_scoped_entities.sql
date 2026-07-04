-- Every connection and workflow must live in a folder. "General" is seeded
-- purely to satisfy the NOT NULL backfill for rows that predate folders -
-- after this migration it is an entirely ordinary, deletable-once-empty
-- folder with no special-cased behavior in application code. Going forward,
-- folder_id is a required field callers must supply on create; nothing
-- defaults to "General" automatically.
INSERT INTO folders (name, description, created_by)
VALUES ('General', 'Default folder for connections and workflows that existed before folders were introduced.', 'system');

ALTER TABLE connections ADD COLUMN folder_id UUID REFERENCES folders(id) ON DELETE RESTRICT;
UPDATE connections SET folder_id = (SELECT id FROM folders WHERE name = 'General' AND parent_id IS NULL);
ALTER TABLE connections ALTER COLUMN folder_id SET NOT NULL;

-- Connection names were globally unique; nothing in the backend looks a
-- connection up by name (only by id), so relaxing this to per-folder
-- uniqueness is safe and is in fact the point of folder namespacing - the
-- same name can now exist under two different folders.
ALTER TABLE connections DROP CONSTRAINT connections_name_key;
CREATE UNIQUE INDEX uq_connections_folder_name ON connections (folder_id, name);
CREATE INDEX idx_connections_folder_id ON connections (folder_id);

ALTER TABLE workflows ADD COLUMN folder_id UUID REFERENCES folders(id) ON DELETE RESTRICT;
UPDATE workflows SET folder_id = (SELECT id FROM folders WHERE name = 'General' AND parent_id IS NULL);
ALTER TABLE workflows ALTER COLUMN folder_id SET NOT NULL;

ALTER TABLE workflows DROP CONSTRAINT workflows_name_key;
CREATE UNIQUE INDEX uq_workflows_folder_name ON workflows (folder_id, name);
CREATE INDEX idx_workflows_folder_id ON workflows (folder_id);
