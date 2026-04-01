package store

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"waka-personal/internal/domain"
)

type Store struct {
	db *pgxpool.Pool
}

const upsertHeartbeatQuery = `
		INSERT INTO heartbeats (
			id, source_heartbeat_id, dedupe_hash, time, source_created_at, entity, type, category,
			project, branch, language, project_root_count, project_folder, lineno, cursorpos,
			lines, is_write, is_unsaved_entity, ai_line_changes, human_line_changes, machine_name,
			source_machine_name_id, plugin, source_user_agent_id, dependencies, import_batch_id,
			origin_payload, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21,
			$22, $23, $24, $25, $26,
			$27, NOW()
		)
		ON CONFLICT (dedupe_hash) DO UPDATE
		SET updated_at = NOW()
		RETURNING
			id, source_heartbeat_id, dedupe_hash, time, source_created_at, entity, type, category,
			project, branch, language, project_root_count, project_folder, lineno, cursorpos,
			lines, is_write, is_unsaved_entity, ai_line_changes, human_line_changes, machine_name,
			source_machine_name_id, plugin, source_user_agent_id, dependencies, import_batch_id,
			origin_payload
	`

func New(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

func Connect(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open database pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

func (s *Store) Close() {
	if s.db != nil {
		s.db.Close()
	}
}

func (s *Store) UpsertHeartbeats(ctx context.Context, records []domain.HeartbeatRecord) ([]domain.HeartbeatRecord, error) {
	if len(records) == 0 {
		return nil, nil
	}

	out := make([]domain.HeartbeatRecord, 0, len(records))
	for i := range records {
		scanned, err := s.upsertHeartbeatRecord(ctx, &records[i])
		if err != nil {
			return nil, err
		}
		out = append(out, scanned)
	}

	return out, nil
}

func (s *Store) upsertHeartbeatRecord(ctx context.Context, record *domain.HeartbeatRecord) (domain.HeartbeatRecord, error) {
	dependencies, err := json.Marshal(record.Dependencies)
	if err != nil {
		return domain.HeartbeatRecord{}, fmt.Errorf("marshal dependencies for %s: %w", record.Entity, err)
	}

	var importBatchID any
	if record.ImportBatchID != nil {
		importBatchID = *record.ImportBatchID
	}

	result, err := s.scanUpsertedHeartbeat(ctx, record, dependencies, importBatchID)
	if err != nil {
		return domain.HeartbeatRecord{}, err
	}
	if err := hydrateUpsertedHeartbeat(&result); err != nil {
		return domain.HeartbeatRecord{}, err
	}
	return result.scanned, nil
}

type heartbeatUpsertResult struct {
	scanned             domain.HeartbeatRecord
	sourceHeartbeatID   *string
	project             *string
	branch              *string
	language            *string
	projectFolder       *string
	machineName         *string
	sourceMachineNameID *string
	plugin              *string
	sourceUserAgentID   *string
	deps                []byte
	importBatchID       any
}

func (s *Store) scanUpsertedHeartbeat(ctx context.Context, record *domain.HeartbeatRecord, dependencies []byte, importBatchID any) (heartbeatUpsertResult, error) {
	result := heartbeatUpsertResult{importBatchID: importBatchID}
	err := s.db.QueryRow(
		ctx,
		upsertHeartbeatQuery,
		record.ID,
		nullableString(record.SourceHeartbeatID),
		record.DedupeHash,
		record.Time,
		record.SourceCreatedAt,
		record.Entity,
		record.Type,
		record.Category,
		nullableString(record.Project),
		nullableString(record.Branch),
		nullableString(record.Language),
		record.ProjectRootCount,
		nullableString(record.ProjectFolder),
		record.Lineno,
		record.Cursorpos,
		record.Lines,
		record.IsWrite,
		record.IsUnsavedEntity,
		record.AILineChanges,
		record.HumanLineChanges,
		nullableString(record.MachineName),
		nullableString(record.SourceMachineNameID),
		nullableString(record.Plugin),
		nullableString(record.SourceUserAgentID),
		dependencies,
		importBatchID,
		record.OriginPayload,
	).Scan(
		&result.scanned.ID,
		&result.sourceHeartbeatID,
		&result.scanned.DedupeHash,
		&result.scanned.Time,
		&result.scanned.SourceCreatedAt,
		&result.scanned.Entity,
		&result.scanned.Type,
		&result.scanned.Category,
		&result.project,
		&result.branch,
		&result.language,
		&result.scanned.ProjectRootCount,
		&result.projectFolder,
		&result.scanned.Lineno,
		&result.scanned.Cursorpos,
		&result.scanned.Lines,
		&result.scanned.IsWrite,
		&result.scanned.IsUnsavedEntity,
		&result.scanned.AILineChanges,
		&result.scanned.HumanLineChanges,
		&result.machineName,
		&result.sourceMachineNameID,
		&result.plugin,
		&result.sourceUserAgentID,
		&result.deps,
		&result.importBatchID,
		&result.scanned.OriginPayload,
	)
	if err != nil {
		return heartbeatUpsertResult{}, fmt.Errorf("upsert heartbeat %s: %w", record.Entity, err)
	}
	return result, nil
}

func hydrateUpsertedHeartbeat(result *heartbeatUpsertResult) error {
	result.scanned.SourceHeartbeatID = derefString(result.sourceHeartbeatID)
	result.scanned.Project = derefString(result.project)
	result.scanned.Branch = derefString(result.branch)
	result.scanned.Language = derefString(result.language)
	result.scanned.ProjectFolder = derefString(result.projectFolder)
	result.scanned.MachineName = derefString(result.machineName)
	result.scanned.SourceMachineNameID = derefString(result.sourceMachineNameID)
	result.scanned.Plugin = derefString(result.plugin)
	result.scanned.SourceUserAgentID = derefString(result.sourceUserAgentID)
	if err := setImportBatchID(&result.scanned, result.importBatchID); err != nil {
		return err
	}
	if len(result.deps) > 0 {
		if err := json.Unmarshal(result.deps, &result.scanned.Dependencies); err != nil {
			return fmt.Errorf("unmarshal dependencies for %s: %w", result.scanned.Entity, err)
		}
	}
	return nil
}

func setImportBatchID(record *domain.HeartbeatRecord, value any) error {
	if value == nil {
		return nil
	}

	batchID, ok := value.(string)
	if !ok {
		return fmt.Errorf("unexpected import_batch_id type %T", value)
	}
	record.ImportBatchID = &batchID
	return nil
}

func (s *Store) ListHeartbeatsByRange(ctx context.Context, start, end time.Time) ([]domain.HeartbeatRecord, error) {
	rows, err := s.db.Query(ctx, `
		SELECT
			id, source_heartbeat_id, dedupe_hash, time, source_created_at, entity, type, category,
			project, branch, language, project_root_count, project_folder, lineno, cursorpos,
			lines, is_write, is_unsaved_entity, ai_line_changes, human_line_changes, machine_name,
			source_machine_name_id, plugin, source_user_agent_id, dependencies, import_batch_id,
			origin_payload
		FROM heartbeats
		WHERE time >= $1 AND time < $2
		ORDER BY time ASC, entity ASC
	`, start, end)
	if err != nil {
		return nil, fmt.Errorf("list heartbeats by range: %w", err)
	}
	defer rows.Close()

	return scanHeartbeats(rows)
}

func (s *Store) ListHeartbeatsForEntity(ctx context.Context, entity, project string, projectRootCount *int) ([]domain.HeartbeatRecord, error) {
	builder := strings.Builder{}
	builder.WriteString(`
		SELECT
			id, source_heartbeat_id, dedupe_hash, time, source_created_at, entity, type, category,
			project, branch, language, project_root_count, project_folder, lineno, cursorpos,
			lines, is_write, is_unsaved_entity, ai_line_changes, human_line_changes, machine_name,
			source_machine_name_id, plugin, source_user_agent_id, dependencies, import_batch_id,
			origin_payload
		FROM heartbeats
		WHERE entity = $1
	`)
	args := []any{entity}
	argPos := 2
	if project != "" {
		_, _ = fmt.Fprintf(&builder, " AND project = $%d", argPos)
		args = append(args, project)
		argPos++
	}
	if projectRootCount != nil {
		_, _ = fmt.Fprintf(&builder, " AND project_root_count = $%d", argPos)
		args = append(args, *projectRootCount)
	}
	builder.WriteString(" ORDER BY time ASC, entity ASC")

	rows, err := s.db.Query(ctx, builder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("list heartbeats for entity: %w", err)
	}
	defer rows.Close()

	return scanHeartbeats(rows)
}

func (s *Store) DeleteHeartbeats(ctx context.Context, start, end time.Time, ids []string) (int64, error) {
	if len(ids) == 0 {
		tag, err := s.db.Exec(ctx, `DELETE FROM heartbeats WHERE time >= $1 AND time < $2`, start, end)
		if err != nil {
			return 0, fmt.Errorf("delete heartbeats by day: %w", err)
		}
		return tag.RowsAffected(), nil
	}

	tag, err := s.db.Exec(ctx, `
		DELETE FROM heartbeats
		WHERE time >= $1 AND time < $2 AND id = ANY($3)
	`, start, end, ids)
	if err != nil {
		return 0, fmt.Errorf("delete heartbeats by ids: %w", err)
	}
	return tag.RowsAffected(), nil
}

func (s *Store) GetProfileSnapshot(ctx context.Context) (*domain.ProfileSnapshot, error) {
	var snapshot domain.ProfileSnapshot
	var city []byte
	var profileJSON []byte
	var externalUserID, username, displayName, fullName, email, photo, profileURL, timezone, plan, lastBranch, lastLanguage, lastPlugin, lastProject *string
	err := s.db.QueryRow(ctx, `
		SELECT
			external_user_id, username, display_name, full_name, email, photo, profile_url,
			timezone, plan, timeout_minutes, writes_only, city, last_branch, last_language,
			last_plugin, last_project, profile_json
		FROM import_profile
		WHERE id = 1
	`).Scan(
		&externalUserID,
		&username,
		&displayName,
		&fullName,
		&email,
		&photo,
		&profileURL,
		&timezone,
		&plan,
		&snapshot.TimeoutMinutes,
		&snapshot.WritesOnly,
		&city,
		&lastBranch,
		&lastLanguage,
		&lastPlugin,
		&lastProject,
		&profileJSON,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get profile snapshot: %w", err)
	}
	snapshot.ExternalUserID = derefString(externalUserID)
	snapshot.Username = derefString(username)
	snapshot.DisplayName = derefString(displayName)
	snapshot.FullName = derefString(fullName)
	snapshot.Email = derefString(email)
	snapshot.Photo = derefString(photo)
	snapshot.ProfileURL = derefString(profileURL)
	snapshot.Timezone = derefString(timezone)
	snapshot.Plan = derefString(plan)
	snapshot.LastBranch = derefString(lastBranch)
	snapshot.LastLanguage = derefString(lastLanguage)
	snapshot.LastPlugin = derefString(lastPlugin)
	snapshot.LastProject = derefString(lastProject)
	snapshot.City = city
	snapshot.ProfileJSON = profileJSON
	return &snapshot, nil
}

func (s *Store) UpsertProfileSnapshot(ctx context.Context, snapshot *domain.ProfileSnapshot) error {
	if snapshot == nil {
		return errors.New("profile snapshot is required")
	}

	value := *snapshot
	if len(value.City) == 0 {
		value.City = []byte("null")
	}
	if len(value.ProfileJSON) == 0 {
		value.ProfileJSON = []byte("{}")
	}

	_, err := s.db.Exec(ctx, `
		INSERT INTO import_profile (
			id, external_user_id, username, display_name, full_name, email, photo, profile_url,
			timezone, plan, timeout_minutes, writes_only, city, last_branch, last_language,
			last_plugin, last_project, profile_json, updated_at
		) VALUES (
			1, $1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, $11, $12, $13, $14,
			$15, $16, $17, NOW()
		)
		ON CONFLICT (id) DO UPDATE
		SET external_user_id = EXCLUDED.external_user_id,
			username = EXCLUDED.username,
			display_name = EXCLUDED.display_name,
			full_name = EXCLUDED.full_name,
			email = EXCLUDED.email,
			photo = EXCLUDED.photo,
			profile_url = EXCLUDED.profile_url,
			timezone = EXCLUDED.timezone,
			plan = EXCLUDED.plan,
			timeout_minutes = EXCLUDED.timeout_minutes,
			writes_only = EXCLUDED.writes_only,
			city = EXCLUDED.city,
			last_branch = EXCLUDED.last_branch,
			last_language = EXCLUDED.last_language,
			last_plugin = EXCLUDED.last_plugin,
			last_project = EXCLUDED.last_project,
			profile_json = EXCLUDED.profile_json,
			updated_at = NOW()
	`,
		nullableString(value.ExternalUserID),
		nullableString(value.Username),
		nullableString(value.DisplayName),
		nullableString(value.FullName),
		nullableString(value.Email),
		nullableString(value.Photo),
		nullableString(value.ProfileURL),
		nullableString(value.Timezone),
		nullableString(value.Plan),
		value.TimeoutMinutes,
		value.WritesOnly,
		value.City,
		nullableString(value.LastBranch),
		nullableString(value.LastLanguage),
		nullableString(value.LastPlugin),
		nullableString(value.LastProject),
		value.ProfileJSON,
	)
	if err != nil {
		return fmt.Errorf("upsert profile snapshot: %w", err)
	}
	return nil
}

func (s *Store) CreateImportBatch(ctx context.Context, batch *domain.ImportBatch) (*domain.ImportBatch, error) {
	if batch == nil {
		return nil, errors.New("import batch is required")
	}

	var result domain.ImportBatch
	err := s.db.QueryRow(ctx, `
		INSERT INTO import_snapshot (
			id, source_path, source_format, source_sha256, status, range_start, range_end,
			imported_rows, skipped_rows, error_text, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7,
			$8, $9, $10, NOW()
		)
		RETURNING id, source_path, source_format, source_sha256, status, range_start, range_end, imported_rows, skipped_rows, error_text
	`,
		batch.ID,
		batch.SourcePath,
		batch.SourceFormat,
		batch.SourceSHA256,
		batch.Status,
		batch.RangeStart,
		batch.RangeEnd,
		batch.ImportedRows,
		batch.SkippedRows,
		batch.ErrorText,
	).Scan(
		&result.ID,
		&result.SourcePath,
		&result.SourceFormat,
		&result.SourceSHA256,
		&result.Status,
		&result.RangeStart,
		&result.RangeEnd,
		&result.ImportedRows,
		&result.SkippedRows,
		&result.ErrorText,
	)
	if err != nil {
		return nil, fmt.Errorf("create import batch: %w", err)
	}
	return &result, nil
}

func (s *Store) UpdateImportBatchStatus(ctx context.Context, batchID, status string, importedRows, skippedRows int64, errText *string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE import_snapshot
		SET status = $2,
			imported_rows = $3,
			skipped_rows = $4,
			error_text = $5,
			updated_at = NOW()
		WHERE id = $1
	`, batchID, status, importedRows, skippedRows, errText)
	if err != nil {
		return fmt.Errorf("update import batch status: %w", err)
	}
	return nil
}

func (s *Store) ImportHeartbeatsFromCSV(ctx context.Context, csvPath, batchID string) (inserted, skipped int64, err error) {
	conn, err := s.db.Acquire(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("acquire db connection: %w", err)
	}
	defer conn.Release()

	var tx pgx.Tx
	tx, err = conn.Begin(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("begin import tx: %w", err)
	}
	defer func() {
		if rollbackErr := rollbackImportTx(ctx, tx); rollbackErr != nil {
			if err == nil {
				err = rollbackErr
				return
			}
			err = errors.Join(err, rollbackErr)
		}
	}()

	if createErr := createTempImportTable(ctx, tx); createErr != nil {
		return 0, 0, createErr
	}
	if copyErr := copyHeartbeatCSVIntoTempTable(ctx, tx, csvPath, batchID); copyErr != nil {
		return 0, 0, copyErr
	}

	var totalRows int64
	totalRows, err = countTempImportRows(ctx, tx)
	if err != nil {
		return 0, 0, err
	}
	inserted, err = insertTempImportRows(ctx, tx)
	if err != nil {
		return 0, 0, err
	}
	skipped = totalRows - inserted

	if err = tx.Commit(ctx); err != nil {
		return 0, 0, fmt.Errorf("commit import tx: %w", err)
	}
	tx = nil

	return inserted, skipped, nil
}

func createTempImportTable(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `
		CREATE TEMP TABLE import_heartbeats_tmp (
			id TEXT NOT NULL,
			source_heartbeat_id TEXT,
			dedupe_hash TEXT NOT NULL,
			time TIMESTAMPTZ NOT NULL,
			source_created_at TIMESTAMPTZ,
			entity TEXT NOT NULL,
			type TEXT NOT NULL,
			category TEXT NOT NULL,
			project TEXT,
			branch TEXT,
			language TEXT,
			project_root_count INTEGER,
			project_folder TEXT,
			lineno INTEGER,
			cursorpos INTEGER,
			lines INTEGER,
			is_write BOOLEAN NOT NULL,
			is_unsaved_entity BOOLEAN NOT NULL,
			ai_line_changes INTEGER,
			human_line_changes INTEGER,
			machine_name TEXT,
			source_machine_name_id TEXT,
			plugin TEXT,
			source_user_agent_id TEXT,
			dependencies JSONB NOT NULL,
			import_batch_id TEXT,
			origin_payload JSONB NOT NULL
		) ON COMMIT DROP
	`)
	if err != nil {
		return fmt.Errorf("create temp import table: %w", err)
	}
	return nil
}

func copyHeartbeatCSVIntoTempTable(ctx context.Context, tx pgx.Tx, csvPath, batchID string) error {
	file, err := os.Open(csvPath)
	if err != nil {
		return fmt.Errorf("open csv %s: %w", csvPath, err)
	}
	defer func() {
		_ = file.Close()
	}()

	source, err := newHeartbeatCSVSource(csv.NewReader(file), batchID)
	if err != nil {
		return err
	}

	if _, err := tx.CopyFrom(
		ctx,
		pgx.Identifier{"import_heartbeats_tmp"},
		[]string{
			"id", "source_heartbeat_id", "dedupe_hash", "time", "source_created_at", "entity", "type",
			"category", "project", "branch", "language", "project_root_count", "project_folder",
			"lineno", "cursorpos", "lines", "is_write", "is_unsaved_entity", "ai_line_changes",
			"human_line_changes", "machine_name", "source_machine_name_id", "plugin",
			"source_user_agent_id", "dependencies", "import_batch_id", "origin_payload",
		},
		source,
	); err != nil {
		return fmt.Errorf("copy csv into temp table: %w", err)
	}
	return nil
}

func countTempImportRows(ctx context.Context, tx pgx.Tx) (int64, error) {
	var totalRows int64
	if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM import_heartbeats_tmp`).Scan(&totalRows); err != nil {
		return 0, fmt.Errorf("count temp import rows: %w", err)
	}
	return totalRows, nil
}

func insertTempImportRows(ctx context.Context, tx pgx.Tx) (int64, error) {
	tag, err := tx.Exec(ctx, `
		INSERT INTO heartbeats (
			id, source_heartbeat_id, dedupe_hash, time, source_created_at, entity, type, category,
			project, branch, language, project_root_count, project_folder, lineno, cursorpos,
			lines, is_write, is_unsaved_entity, ai_line_changes, human_line_changes, machine_name,
			source_machine_name_id, plugin, source_user_agent_id, dependencies, import_batch_id,
			origin_payload, created_at, updated_at
		)
		SELECT
			id, NULLIF(source_heartbeat_id, ''), dedupe_hash, time, source_created_at, entity, type, category,
			NULLIF(project, ''), NULLIF(branch, ''), NULLIF(language, ''), project_root_count,
			NULLIF(project_folder, ''), lineno, cursorpos, lines, is_write, is_unsaved_entity,
			ai_line_changes, human_line_changes, NULLIF(machine_name, ''),
			NULLIF(source_machine_name_id, ''), NULLIF(plugin, ''), NULLIF(source_user_agent_id, ''),
			dependencies, import_batch_id, origin_payload, NOW(), NOW()
		FROM import_heartbeats_tmp
		ON CONFLICT (dedupe_hash) DO NOTHING
	`)
	if err != nil {
		return 0, fmt.Errorf("insert temp import rows: %w", err)
	}
	return tag.RowsAffected(), nil
}

func rollbackImportTx(ctx context.Context, tx pgx.Tx) error {
	if tx == nil {
		return nil
	}
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		return fmt.Errorf("rollback import tx: %w", err)
	}
	return nil
}

func scanHeartbeats(rows pgx.Rows) ([]domain.HeartbeatRecord, error) {
	items := make([]domain.HeartbeatRecord, 0)
	for rows.Next() {
		var record domain.HeartbeatRecord
		var sourceHeartbeatID *string
		var project, branch, language, projectFolder, machineName, sourceMachineNameID, plugin, sourceUserAgentID *string
		var dependencies []byte
		var importBatchID *string
		if err := rows.Scan(
			&record.ID,
			&sourceHeartbeatID,
			&record.DedupeHash,
			&record.Time,
			&record.SourceCreatedAt,
			&record.Entity,
			&record.Type,
			&record.Category,
			&project,
			&branch,
			&language,
			&record.ProjectRootCount,
			&projectFolder,
			&record.Lineno,
			&record.Cursorpos,
			&record.Lines,
			&record.IsWrite,
			&record.IsUnsavedEntity,
			&record.AILineChanges,
			&record.HumanLineChanges,
			&machineName,
			&sourceMachineNameID,
			&plugin,
			&sourceUserAgentID,
			&dependencies,
			&importBatchID,
			&record.OriginPayload,
		); err != nil {
			return nil, fmt.Errorf("scan heartbeat row: %w", err)
		}

		record.SourceHeartbeatID = derefString(sourceHeartbeatID)
		record.Project = derefString(project)
		record.Branch = derefString(branch)
		record.Language = derefString(language)
		record.ProjectFolder = derefString(projectFolder)
		record.MachineName = derefString(machineName)
		record.SourceMachineNameID = derefString(sourceMachineNameID)
		record.Plugin = derefString(plugin)
		record.SourceUserAgentID = derefString(sourceUserAgentID)
		record.ImportBatchID = importBatchID
		if len(dependencies) > 0 {
			if err := json.Unmarshal(dependencies, &record.Dependencies); err != nil {
				return nil, fmt.Errorf("unmarshal heartbeat dependencies: %w", err)
			}
		}

		items = append(items, record)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate heartbeat rows: %w", rows.Err())
	}

	return items, nil
}

type heartbeatCSVSource struct {
	reader    *csv.Reader
	current   []any
	err       error
	batchID   string
	lineIndex int
}

func newHeartbeatCSVSource(reader *csv.Reader, batchID string) (*heartbeatCSVSource, error) {
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read import csv header: %w", err)
	}

	expected := []string{
		"source_heartbeat_id", "time", "source_created_at", "entity", "type", "category",
		"project", "branch", "language", "project_root_count", "project_folder", "lineno",
		"cursorpos", "lines", "is_write", "is_unsaved_entity", "ai_line_changes",
		"human_line_changes", "machine_name", "source_machine_name_id", "plugin",
		"source_user_agent_id", "dependencies_json", "origin_payload_json",
	}
	if strings.Join(header, ",") != strings.Join(expected, ",") {
		return nil, fmt.Errorf("unexpected import csv header: %v", header)
	}

	return &heartbeatCSVSource{
		reader:  reader,
		batchID: batchID,
	}, nil
}

func (s *heartbeatCSVSource) Next() bool {
	if s.err != nil {
		return false
	}

	record, err := s.reader.Read()
	if errors.Is(err, io.EOF) {
		return false
	}
	if err != nil {
		s.err = fmt.Errorf("read import csv row: %w", err)
		return false
	}

	s.lineIndex++

	id, dedupeHash, values, err := parseHeartbeatCSVRecord(record, s.batchID)
	if err != nil {
		s.err = fmt.Errorf("parse import csv row %d: %w", s.lineIndex, err)
		return false
	}
	s.current = append([]any{id, nullableString(values.SourceHeartbeatID), dedupeHash}, values.Values()...)
	return true
}

func (s *heartbeatCSVSource) Values() ([]any, error) {
	return s.current, nil
}

func (s *heartbeatCSVSource) Err() error {
	return s.err
}

type parsedHeartbeatCSV struct {
	SourceHeartbeatID   string
	Time                time.Time
	SourceCreatedAt     *time.Time
	Entity              string
	Type                string
	Category            string
	Project             string
	Branch              string
	Language            string
	ProjectRootCount    *int
	ProjectFolder       string
	Lineno              *int
	Cursorpos           *int
	Lines               *int
	IsWrite             bool
	IsUnsavedEntity     bool
	AILineChanges       *int
	HumanLineChanges    *int
	MachineName         string
	SourceMachineNameID string
	Plugin              string
	SourceUserAgentID   string
	Dependencies        []byte
	OriginPayload       []byte
	ImportBatchID       string
}

func (p *parsedHeartbeatCSV) Values() []any {
	return []any{
		p.Time,
		p.SourceCreatedAt,
		p.Entity,
		p.Type,
		p.Category,
		nullableString(p.Project),
		nullableString(p.Branch),
		nullableString(p.Language),
		p.ProjectRootCount,
		nullableString(p.ProjectFolder),
		p.Lineno,
		p.Cursorpos,
		p.Lines,
		p.IsWrite,
		p.IsUnsavedEntity,
		p.AILineChanges,
		p.HumanLineChanges,
		nullableString(p.MachineName),
		nullableString(p.SourceMachineNameID),
		nullableString(p.Plugin),
		nullableString(p.SourceUserAgentID),
		p.Dependencies,
		p.ImportBatchID,
		p.OriginPayload,
	}
}

func parseHeartbeatCSVRecord(record []string, batchID string) (id, dedupeHash string, parsed *parsedHeartbeatCSV, err error) {
	if len(record) != 24 {
		return "", "", nil, fmt.Errorf("expected 24 columns, got %d", len(record))
	}

	heartbeatTime, err := parseHeartbeatCSVTime(record[1])
	if err != nil {
		return "", "", nil, fmt.Errorf("parse time %q: %w", record[1], err)
	}

	fields, err := parseHeartbeatCSVFields(record)
	if err != nil {
		return "", "", nil, err
	}

	id, dedupeHash = domain.BuildDedupeIdentifiers(
		record[0],
		heartbeatTime,
		record[3],
		record[4],
		record[5],
		record[6],
		record[7],
		record[8],
		fields.IsWrite,
		fields.Lineno,
		fields.Cursorpos,
		record[20],
	)

	parsed = &parsedHeartbeatCSV{
		SourceHeartbeatID:   record[0],
		Time:                heartbeatTime,
		SourceCreatedAt:     fields.SourceCreatedAt,
		Entity:              record[3],
		Type:                defaultIfEmpty(record[4], "file"),
		Category:            defaultIfEmpty(record[5], "coding"),
		Project:             record[6],
		Branch:              record[7],
		Language:            record[8],
		ProjectRootCount:    fields.ProjectRootCount,
		ProjectFolder:       record[10],
		Lineno:              fields.Lineno,
		Cursorpos:           fields.Cursorpos,
		Lines:               fields.Lines,
		IsWrite:             fields.IsWrite,
		IsUnsavedEntity:     fields.IsUnsavedEntity,
		AILineChanges:       fields.AILineChanges,
		HumanLineChanges:    fields.HumanLineChanges,
		MachineName:         record[18],
		SourceMachineNameID: record[19],
		Plugin:              record[20],
		SourceUserAgentID:   record[21],
		Dependencies:        []byte(defaultIfEmpty(record[22], "[]")),
		OriginPayload:       []byte(defaultIfEmpty(record[23], "{}")),
		ImportBatchID:       batchID,
	}

	return id, dedupeHash, parsed, nil
}

type parsedHeartbeatCSVFields struct {
	SourceCreatedAt  *time.Time
	ProjectRootCount *int
	Lineno           *int
	Cursorpos        *int
	Lines            *int
	AILineChanges    *int
	HumanLineChanges *int
	IsWrite          bool
	IsUnsavedEntity  bool
}

func parseHeartbeatCSVTime(value string) (time.Time, error) {
	timestamp, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(0, int64(timestamp*float64(time.Second))).UTC(), nil
}

func parseHeartbeatCSVFields(record []string) (parsedHeartbeatCSVFields, error) {
	fields := parsedHeartbeatCSVFields{}
	var err error

	fields.SourceCreatedAt, err = parseOptionalTimestamp(record[2])
	if err != nil {
		return fields, fmt.Errorf("parse source_created_at %q: %w", record[2], err)
	}
	fields.ProjectRootCount, err = parseOptionalInt(record[9])
	if err != nil {
		return fields, fmt.Errorf("parse project_root_count: %w", err)
	}
	fields.Lineno, err = parseOptionalInt(record[11])
	if err != nil {
		return fields, fmt.Errorf("parse lineno: %w", err)
	}
	fields.Cursorpos, err = parseOptionalInt(record[12])
	if err != nil {
		return fields, fmt.Errorf("parse cursorpos: %w", err)
	}
	fields.Lines, err = parseOptionalInt(record[13])
	if err != nil {
		return fields, fmt.Errorf("parse lines: %w", err)
	}
	fields.AILineChanges, err = parseOptionalInt(record[16])
	if err != nil {
		return fields, fmt.Errorf("parse ai_line_changes: %w", err)
	}
	fields.HumanLineChanges, err = parseOptionalInt(record[17])
	if err != nil {
		return fields, fmt.Errorf("parse human_line_changes: %w", err)
	}
	fields.IsWrite, err = parseOptionalBool(record[14], false)
	if err != nil {
		return fields, fmt.Errorf("parse is_write: %w", err)
	}
	fields.IsUnsavedEntity, err = parseOptionalBool(record[15], false)
	if err != nil {
		return fields, fmt.Errorf("parse is_unsaved_entity: %w", err)
	}

	return fields, nil
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func derefString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func parseOptionalInt(value string) (*int, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseOptionalBool(value string, fallback bool) (bool, error) {
	return strconv.ParseBool(strings.ToLower(defaultIfEmpty(value, strconv.FormatBool(fallback))))
}

func parseOptionalTimestamp(value string) (*time.Time, error) {
	if value == "" {
		return nil, nil
	}

	layouts := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999Z07:00",
		"2006-01-02 15:04:05.999999Z07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02 15:04:05.999999999",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		var (
			parsed time.Time
			err    error
		)

		if strings.Contains(layout, "Z07:00") {
			parsed, err = time.Parse(layout, value)
		} else {
			parsed, err = time.ParseInLocation(layout, value, time.UTC)
		}
		if err == nil {
			parsed = parsed.UTC()
			return &parsed, nil
		}
	}

	return nil, fmt.Errorf("unsupported timestamp format")
}

func defaultIfEmpty(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func IsUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}
