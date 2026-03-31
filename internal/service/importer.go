package service

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"

	"waka-personal/internal/domain"
	"waka-personal/internal/store"
)

type ImportService struct {
	store *store.Store
}

func NewImportService(store *store.Store) *ImportService {
	return &ImportService{store: store}
}

type ImportOptions struct {
	FilePath string
	Format   string
	DryRun   bool
}

type ImportResult struct {
	BatchID      string
	ImportedRows int64
	SkippedRows  int64
}

type backupRange struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

const duckDBMaximumObjectSizeBytes = 1024 * 1024 * 1024

func (s *ImportService) ImportBackupFile(ctx context.Context, options ImportOptions) (*ImportResult, error) {
	resolvedPath, err := filepath.Abs(options.FilePath)
	if err != nil {
		return nil, fmt.Errorf("resolve import file path: %w", err)
	}
	if options.Format == "" {
		options.Format = "backup-json"
	}
	if options.Format != "backup-json" {
		return nil, fmt.Errorf("unsupported import format %q", options.Format)
	}
	if !strings.HasSuffix(strings.ToLower(resolvedPath), ".json") && !strings.HasSuffix(strings.ToLower(resolvedPath), ".json.gz") {
		return nil, fmt.Errorf("import file must end with .json or .json.gz")
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, fmt.Errorf("stat import file: %w", err)
	}
	if info.IsDir() {
		return nil, fmt.Errorf("import file must not be a directory")
	}

	checksum, err := checksumFile(resolvedPath)
	if err != nil {
		return nil, err
	}

	rawUser, fileRange, err := readBackupMetadata(resolvedPath)
	if err != nil {
		return nil, err
	}

	batch, err := s.store.UpsertImportBatch(ctx, domain.ImportBatch{
		ID:           uuid.NewString(),
		SourcePath:   resolvedPath,
		SourceFormat: options.Format,
		SourceSHA256: checksum,
		Status:       "running",
		RangeStart:   epochToTime(fileRange.Start),
		RangeEnd:     epochToTime(fileRange.End),
	})
	if err != nil {
		return nil, err
	}

	if len(rawUser) > 0 {
		snapshot, err := mapBackupUserToSnapshot(rawUser)
		if err != nil {
			errText := err.Error()
			_ = s.store.UpdateImportBatchStatus(ctx, batch.ID, "failed", 0, 0, &errText)
			return nil, err
		}
		if err := s.store.UpsertProfileSnapshot(ctx, snapshot); err != nil {
			errText := err.Error()
			_ = s.store.UpdateImportBatchStatus(ctx, batch.ID, "failed", 0, 0, &errText)
			return nil, err
		}
	}

	if options.DryRun {
		if err := s.store.UpdateImportBatchStatus(ctx, batch.ID, "dry-run", 0, 0, nil); err != nil {
			return nil, err
		}
		return &ImportResult{BatchID: batch.ID}, nil
	}

	tempDir, err := os.MkdirTemp("", "waka-import-*")
	if err != nil {
		errText := err.Error()
		_ = s.store.UpdateImportBatchStatus(ctx, batch.ID, "failed", 0, 0, &errText)
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	outputCSV := filepath.Join(tempDir, "heartbeats.csv")
	if err := flattenBackupToCSV(ctx, resolvedPath, outputCSV); err != nil {
		errText := err.Error()
		_ = s.store.UpdateImportBatchStatus(ctx, batch.ID, "failed", 0, 0, &errText)
		return nil, err
	}

	imported, skipped, err := s.store.ImportHeartbeatsFromCSV(ctx, outputCSV, batch.ID)
	if err != nil {
		errText := err.Error()
		_ = s.store.UpdateImportBatchStatus(ctx, batch.ID, "failed", 0, 0, &errText)
		return nil, err
	}

	if err := s.store.UpdateImportBatchStatus(ctx, batch.ID, "completed", imported, skipped, nil); err != nil {
		return nil, err
	}

	return &ImportResult{
		BatchID:      batch.ID,
		ImportedRows: imported,
		SkippedRows:  skipped,
	}, nil
}

func readBackupMetadata(path string) (json.RawMessage, backupRange, error) {
	reader, closeFn, err := openMaybeCompressed(path)
	if err != nil {
		return nil, backupRange{}, err
	}
	defer closeFn()

	decoder := json.NewDecoder(reader)
	token, err := decoder.Token()
	if err != nil {
		return nil, backupRange{}, fmt.Errorf("read backup root token: %w", err)
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return nil, backupRange{}, fmt.Errorf("backup root must be a json object")
	}

	var rawUser json.RawMessage
	var fileRange backupRange
	for decoder.More() {
		keyToken, err := decoder.Token()
		if err != nil {
			return nil, backupRange{}, fmt.Errorf("read backup key: %w", err)
		}
		key, _ := keyToken.(string)

		switch key {
		case "user":
			if err := decoder.Decode(&rawUser); err != nil {
				return nil, backupRange{}, fmt.Errorf("decode backup user: %w", err)
			}
		case "range":
			if err := decoder.Decode(&fileRange); err != nil {
				return nil, backupRange{}, fmt.Errorf("decode backup range: %w", err)
			}
		default:
			var discard json.RawMessage
			if err := decoder.Decode(&discard); err != nil {
				return nil, backupRange{}, fmt.Errorf("skip backup key %q: %w", key, err)
			}
		}

		if len(rawUser) > 0 && (fileRange.Start != 0 || fileRange.End != 0) {
			break
		}
	}

	return rawUser, fileRange, nil
}

func mapBackupUserToSnapshot(raw json.RawMessage) (domain.ProfileSnapshot, error) {
	type backupUser struct {
		ID           string          `json:"id"`
		Username     string          `json:"username"`
		DisplayName  string          `json:"display_name"`
		FullName     string          `json:"full_name"`
		Email        string          `json:"email"`
		Photo        string          `json:"photo"`
		ProfileURL   string          `json:"profile_url"`
		Timezone     string          `json:"timezone"`
		Plan         string          `json:"plan"`
		Timeout      *int            `json:"timeout"`
		WritesOnly   *bool           `json:"writes_only"`
		City         json.RawMessage `json:"city"`
		LastBranch   string          `json:"last_branch"`
		LastLanguage string          `json:"last_language"`
		LastPlugin   string          `json:"last_plugin"`
		LastProject  string          `json:"last_project"`
	}

	var user backupUser
	if err := json.Unmarshal(raw, &user); err != nil {
		return domain.ProfileSnapshot{}, fmt.Errorf("decode backup user: %w", err)
	}
	if len(user.City) == 0 {
		user.City = []byte("null")
	}

	return domain.ProfileSnapshot{
		ExternalUserID: user.ID,
		Username:       user.Username,
		DisplayName:    user.DisplayName,
		FullName:       user.FullName,
		Email:          user.Email,
		Photo:          user.Photo,
		ProfileURL:     user.ProfileURL,
		Timezone:       user.Timezone,
		Plan:           user.Plan,
		TimeoutMinutes: user.Timeout,
		WritesOnly:     user.WritesOnly,
		City:           user.City,
		LastBranch:     user.LastBranch,
		LastLanguage:   user.LastLanguage,
		LastPlugin:     user.LastPlugin,
		LastProject:    user.LastProject,
		ProfileJSON:    raw,
	}, nil
}

func buildDuckDBSQL(inputPath, outputPath string) string {
	return fmt.Sprintf(`
COPY (
  WITH source AS (
    SELECT UNNEST(days) AS day
    FROM read_json_auto('%s', maximum_object_size = %d)
  ),
  flat AS (
    SELECT to_json(UNNEST(day.heartbeats)) AS hb_json
    FROM source
  )
  SELECT
    COALESCE(json_extract_string(hb_json, '$.id'), '') AS source_heartbeat_id,
    COALESCE(json_extract_string(hb_json, '$.time'), '') AS time,
    COALESCE(json_extract_string(hb_json, '$.created_at'), '') AS source_created_at,
    COALESCE(json_extract_string(hb_json, '$.entity'), '') AS entity,
    COALESCE(json_extract_string(hb_json, '$.type'), 'file') AS type,
    COALESCE(json_extract_string(hb_json, '$.category'), 'coding') AS category,
    COALESCE(json_extract_string(hb_json, '$.project'), '') AS project,
    COALESCE(json_extract_string(hb_json, '$.branch'), '') AS branch,
    COALESCE(json_extract_string(hb_json, '$.language'), '') AS language,
    COALESCE(json_extract_string(hb_json, '$.project_root_count'), '') AS project_root_count,
    COALESCE(json_extract_string(hb_json, '$.project_folder'), '') AS project_folder,
    COALESCE(json_extract_string(hb_json, '$.lineno'), '') AS lineno,
    COALESCE(json_extract_string(hb_json, '$.cursorpos'), '') AS cursorpos,
    COALESCE(json_extract_string(hb_json, '$.lines'), '') AS lines,
    COALESCE(json_extract_string(hb_json, '$.is_write'), 'false') AS is_write,
    COALESCE(json_extract_string(hb_json, '$.is_unsaved_entity'), 'false') AS is_unsaved_entity,
    COALESCE(json_extract_string(hb_json, '$.ai_line_changes'), '') AS ai_line_changes,
    COALESCE(json_extract_string(hb_json, '$.human_line_changes'), '') AS human_line_changes,
    COALESCE(json_extract_string(hb_json, '$.machine_name'), '') AS machine_name,
    COALESCE(json_extract_string(hb_json, '$.machine_name_id'), '') AS source_machine_name_id,
    COALESCE(json_extract_string(hb_json, '$.plugin'), '') AS plugin,
    COALESCE(json_extract_string(hb_json, '$.user_agent_id'), '') AS source_user_agent_id,
    COALESCE(CAST(json_extract(hb_json, '$.dependencies') AS VARCHAR), '[]') AS dependencies_json,
    COALESCE(CAST(hb_json AS VARCHAR), '{}') AS origin_payload_json
  FROM flat
) TO '%s' (HEADER, DELIMITER ',');
`, inputPath, duckDBMaximumObjectSizeBytes, outputPath)
}

func checksumFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open file for checksum: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("checksum file: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func duckDBPath(path string) string {
	return escapeSQLString(filepath.ToSlash(path))
}

func openMaybeCompressed(path string) (io.Reader, func(), error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("open backup file: %w", err)
	}

	closeFn := func() { _ = file.Close() }
	if strings.HasSuffix(strings.ToLower(path), ".gz") {
		reader, err := gzip.NewReader(file)
		if err != nil {
			closeFn()
			return nil, nil, fmt.Errorf("open gzip reader: %w", err)
		}
		return reader, func() {
			_ = reader.Close()
			closeFn()
		}, nil
	}
	return file, closeFn, nil
}

func epochToTime(value int64) *time.Time {
	if value <= 0 {
		return nil
	}
	parsed := time.Unix(value, 0).UTC()
	return &parsed
}

func escapeSQLString(value string) string {
	return strings.ReplaceAll(value, "'", "''")
}
