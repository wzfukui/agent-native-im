ALTER TABLE IF EXISTS file_records OWNER TO agent_im;
ALTER SEQUENCE IF EXISTS file_records_id_seq OWNER TO agent_im;

GRANT SELECT, INSERT, UPDATE, DELETE ON TABLE file_records TO agent_im;
GRANT USAGE, SELECT ON SEQUENCE file_records_id_seq TO agent_im;
