-- Schema of a GARM v0.2.1 objects (blob) database. See v021_schema.sql.
CREATE TABLE `file_objects` (`id` integer PRIMARY KEY AUTOINCREMENT,`created_at` datetime,`updated_at` datetime,`deleted_at` datetime,`name` text,`description` text,`file_type` text,`size` integer,`sha256` text);
CREATE TABLE `file_blobs` (`id` integer PRIMARY KEY AUTOINCREMENT,`created_at` datetime,`updated_at` datetime,`deleted_at` datetime,`file_object_id` integer,`content` blob,CONSTRAINT `fk_file_objects_content` FOREIGN KEY (`file_object_id`) REFERENCES `file_objects`(`id`) ON DELETE CASCADE);
CREATE TABLE `file_object_tags` (`id` integer PRIMARY KEY AUTOINCREMENT,`file_object_id` integer NOT NULL,`tag` TEXT COLLATE NOCASE NOT NULL,CONSTRAINT `fk_file_objects_tags_list` FOREIGN KEY (`file_object_id`) REFERENCES `file_objects`(`id`) ON DELETE CASCADE);
CREATE INDEX `idx_fo_chksum` ON `file_objects`(`sha256`);
CREATE INDEX `idx_fo_name` ON `file_objects`(`name`);
CREATE INDEX `idx_file_objects_deleted_at` ON `file_objects`(`deleted_at`);
CREATE INDEX `idx_fileobject_blob_id` ON `file_blobs`(`file_object_id`);
CREATE INDEX `idx_file_blobs_deleted_at` ON `file_blobs`(`deleted_at`);
CREATE INDEX `idx_fileobject_tags_tag` ON `file_object_tags`(`file_object_id`,`tag`);
CREATE INDEX `idx_fileobject_tags_doc_id` ON `file_object_tags`(`file_object_id`);
