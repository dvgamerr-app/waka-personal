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

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"waka-personal/internal/domain"
)

type Store struct {
	db *pgxpool.Pool
}

type sourceUserAgentDB interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

const upsertHeartbeatQuery = `
		INSERT INTO heartbeats (
			id, source_heartbeat_id, dedupe_hash, time, source_created_at, entity, type, category,
			project, branch, language, project_root_count, project_folder, lineno, cursorpos,
			lines, is_write, is_unsaved_entity, ai_line_changes, human_line_changes, machine_name,
			source_machine_name_id, plugin, source_user_agent_id, dependencies, import_batch_id,
			origin_payload, ai_session, ai_subscription_plan, ai_input_tokens, ai_output_tokens,
			ai_prompt_length, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8,
			$9, $10, $11, $12, $13, $14, $15,
			$16, $17, $18, $19, $20, $21,
			$22, $23, $24, $25, $26,
			$27, $28, $29, $30, $31,
			$32, NOW()
		)
		ON CONFLICT (dedupe_hash) DO UPDATE
		SET source_heartbeat_id = EXCLUDED.source_heartbeat_id,
			time = EXCLUDED.time,
			source_created_at = EXCLUDED.source_created_at,
			entity = EXCLUDED.entity,
			type = EXCLUDED.type,
			category = EXCLUDED.category,
			project = EXCLUDED.project,
			branch = EXCLUDED.branch,
			language = EXCLUDED.language,
			project_root_count = EXCLUDED.project_root_count,
			project_folder = EXCLUDED.project_folder,
			lineno = EXCLUDED.lineno,
			cursorpos = EXCLUDED.cursorpos,
			lines = EXCLUDED.lines,
			is_write = EXCLUDED.is_write,
			is_unsaved_entity = EXCLUDED.is_unsaved_entity,
			ai_line_changes = EXCLUDED.ai_line_changes,
			human_line_changes = EXCLUDED.human_line_changes,
			machine_name = EXCLUDED.machine_name,
			source_machine_name_id = EXCLUDED.source_machine_name_id,
			plugin = EXCLUDED.plugin,
			source_user_agent_id = EXCLUDED.source_user_agent_id,
			dependencies = EXCLUDED.dependencies,
			import_batch_id = EXCLUDED.import_batch_id,
			origin_payload = EXCLUDED.origin_payload,
			ai_session = EXCLUDED.ai_session,
			ai_subscription_plan = EXCLUDED.ai_subscription_plan,
			ai_input_tokens = EXCLUDED.ai_input_tokens,
			ai_output_tokens = EXCLUDED.ai_output_tokens,
			ai_prompt_length = EXCLUDED.ai_prompt_length,
			updated_at = NOW()
		RETURNING
			id, source_heartbeat_id, dedupe_hash, time, source_created_at, entity, type, category,
			project, branch, language, project_root_count, project_folder, lineno, cursorpos,
			lines, is_write, is_unsaved_entity, ai_line_changes, human_line_changes, machine_name,
			source_machine_name_id, plugin, source_user_agent_id, dependencies, import_batch_id,
			origin_payload, ai_session, ai_subscription_plan, ai_input_tokens, ai_output_tokens,
			ai_prompt_length
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

func (s *Store) ResolveSourceUserAgentID(ctx context.Context, preferredID, userAgent string) (string, error) {
	id, err := upsertSourceUserAgent(ctx, s.db, preferredID, userAgent, "api")
	if err != nil {
		return "", fmt.Errorf("resolve source user agent: %w", err)
	}
	return id, nil
}

func upsertSourceUserAgent(ctx context.Context, db sourceUserAgentDB, preferredID, userAgent, source string) (string, error) {
	preferredID = strings.TrimSpace(preferredID)
	userAgent = strings.TrimSpace(userAgent)
	if preferredID == "" && userAgent == "" {
		return "", nil
	}

	agentKey, agentName := domain.InferAIAgent(userAgent)
	if source == "" {
		source = "local"
	}

	if preferredID != "" && userAgent != "" {
		if err := replaceGeneratedSourceUserAgentID(ctx, db, preferredID, userAgent, agentKey, agentName, source); err != nil {
			return "", err
		}
	}

	id := preferredID
	if id == "" {
		id = uuid.NewString()
	}

	var resolvedID string
	err := db.QueryRow(ctx, `
		INSERT INTO source_user_agents (id, user_agent, ai_agent_key, ai_agent_name, source, updated_at)
		VALUES ($1, NULLIF($2, ''), NULLIF($3, ''), NULLIF($4, ''), $5, NOW())
		ON CONFLICT (id) DO UPDATE
		SET user_agent = COALESCE(EXCLUDED.user_agent, source_user_agents.user_agent),
			ai_agent_key = COALESCE(EXCLUDED.ai_agent_key, source_user_agents.ai_agent_key),
			ai_agent_name = COALESCE(EXCLUDED.ai_agent_name, source_user_agents.ai_agent_name),
			source = EXCLUDED.source,
			updated_at = NOW()
		RETURNING id
	`, id, userAgent, agentKey, agentName, source).Scan(&resolvedID)
	if err == nil {
		return resolvedID, nil
	}
	if !IsUniqueViolation(err) || userAgent == "" {
		return "", fmt.Errorf("upsert source user agent: %w", err)
	}

	err = db.QueryRow(ctx, `
		UPDATE source_user_agents
		SET ai_agent_key = COALESCE(NULLIF($2, ''), ai_agent_key),
			ai_agent_name = COALESCE(NULLIF($3, ''), ai_agent_name),
			source = $4,
			updated_at = NOW()
		WHERE user_agent = $1
		RETURNING id
	`, userAgent, agentKey, agentName, source).Scan(&resolvedID)
	if err != nil {
		return "", fmt.Errorf("lookup source user agent by user_agent: %w", err)
	}
	return resolvedID, nil
}

func replaceGeneratedSourceUserAgentID(ctx context.Context, db sourceUserAgentDB, preferredID, userAgent, agentKey, agentName, source string) error {
	var existingID string
	err := db.QueryRow(ctx, `
		SELECT id
		FROM source_user_agents
		WHERE user_agent = $1
		LIMIT 1
	`, userAgent).Scan(&existingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lookup existing source user agent: %w", err)
	}
	if existingID == preferredID {
		return nil
	}

	_, err = db.Exec(ctx, `
		UPDATE source_user_agents
		SET id = $1,
			ai_agent_key = COALESCE(NULLIF($3, ''), ai_agent_key),
			ai_agent_name = COALESCE(NULLIF($4, ''), ai_agent_name),
			source = $5,
			updated_at = NOW()
		WHERE id = $2
	`, preferredID, existingID, agentKey, agentName, source)
	if err == nil {
		return nil
	}
	if !IsUniqueViolation(err) {
		return fmt.Errorf("replace generated source user agent id: %w", err)
	}

	if _, err := db.Exec(ctx, `
		UPDATE heartbeats
		SET source_user_agent_id = $1
		WHERE source_user_agent_id = $2
	`, preferredID, existingID); err != nil {
		return fmt.Errorf("move heartbeats to trusted source user agent id: %w", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM source_user_agents WHERE id = $1`, existingID); err != nil {
		return fmt.Errorf("delete generated source user agent id: %w", err)
	}
	return nil
}

func (s *Store) ResolveSourceMachineNameID(ctx context.Context, preferredID, machineName string) (string, error) {
	id, err := upsertSourceMachineName(ctx, s.db, preferredID, machineName, "api")
	if err != nil {
		return "", fmt.Errorf("resolve source machine name: %w", err)
	}
	return id, nil
}

func upsertSourceMachineName(ctx context.Context, db sourceUserAgentDB, preferredID, machineName, source string) (string, error) {
	preferredID = strings.TrimSpace(preferredID)
	machineName = strings.TrimSpace(machineName)
	if preferredID == "" && machineName == "" {
		return "", nil
	}

	if source == "" {
		source = "local"
	}

	if preferredID != "" && machineName != "" {
		if err := replaceGeneratedSourceMachineNameID(ctx, db, preferredID, machineName, source); err != nil {
			return "", err
		}
	}

	id := preferredID
	if id == "" {
		id = uuid.NewString()
	}

	var resolvedID string
	err := db.QueryRow(ctx, `
		INSERT INTO source_machine_names (id, machine_name, source, updated_at)
		VALUES ($1, NULLIF($2, ''), $3, NOW())
		ON CONFLICT (id) DO UPDATE
		SET machine_name = COALESCE(EXCLUDED.machine_name, source_machine_names.machine_name),
			source = EXCLUDED.source,
			updated_at = NOW()
		RETURNING id
	`, id, machineName, source).Scan(&resolvedID)
	if err == nil {
		return resolvedID, nil
	}
	if !IsUniqueViolation(err) || machineName == "" {
		return "", fmt.Errorf("upsert source machine name: %w", err)
	}

	err = db.QueryRow(ctx, `
		UPDATE source_machine_names
		SET source = $2,
			updated_at = NOW()
		WHERE machine_name = $1
		RETURNING id
	`, machineName, source).Scan(&resolvedID)
	if err != nil {
		return "", fmt.Errorf("lookup source machine name by machine_name: %w", err)
	}
	return resolvedID, nil
}

func replaceGeneratedSourceMachineNameID(ctx context.Context, db sourceUserAgentDB, preferredID, machineName, source string) error {
	var existingID string
	err := db.QueryRow(ctx, `
		SELECT id
		FROM source_machine_names
		WHERE machine_name = $1
		LIMIT 1
	`, machineName).Scan(&existingID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("lookup existing source machine name: %w", err)
	}
	if existingID == preferredID {
		return nil
	}

	_, err = db.Exec(ctx, `
		UPDATE source_machine_names
		SET id = $1,
			source = $3,
			updated_at = NOW()
		WHERE id = $2
	`, preferredID, existingID, source)
	if err == nil {
		return nil
	}
	if !IsUniqueViolation(err) {
		return fmt.Errorf("replace generated source machine name id: %w", err)
	}

	if _, err := db.Exec(ctx, `
		UPDATE heartbeats
		SET source_machine_name_id = $1
		WHERE source_machine_name_id = $2
	`, preferredID, existingID); err != nil {
		return fmt.Errorf("move heartbeats to trusted source machine name id: %w", err)
	}
	if _, err := db.Exec(ctx, `DELETE FROM source_machine_names WHERE id = $1`, existingID); err != nil {
		return fmt.Errorf("delete generated source machine name id: %w", err)
	}
	return nil
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
	aiSession           *string
	aiSubscriptionPlan  *string
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
		nullableString(record.AISession),
		nullableString(record.AISubscriptionPlan),
		record.AIInputTokens,
		record.AIOutputTokens,
		record.AIPromptLength,
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
		&result.aiSession,
		&result.aiSubscriptionPlan,
		&result.scanned.AIInputTokens,
		&result.scanned.AIOutputTokens,
		&result.scanned.AIPromptLength,
	)
	if err != nil {
		return heartbeatUpsertResult{}, fmt.Errorf("upsert heartbeat %s: %w", record.Entity, err)
	}
	return result, nil
}

func hydrateUpsertedHeartbeat(result *heartbeatUpsertResult) error {
	result.scanned.SourceHeartbeatID = derefString(result.sourceHeartbeatID)
	result.scanned.AISession = derefString(result.aiSession)
	result.scanned.AISubscriptionPlan = derefString(result.aiSubscriptionPlan)
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
			h.id, h.source_heartbeat_id, h.dedupe_hash, h.time, h.source_created_at, h.entity, h.type, h.category,
			h.project, h.branch, h.language, h.project_root_count, h.project_folder, h.lineno, h.cursorpos,
			h.lines, h.is_write, h.is_unsaved_entity, h.ai_line_changes, h.human_line_changes, h.machine_name,
			h.source_machine_name_id, h.plugin, h.source_user_agent_id, h.dependencies, h.import_batch_id,
			h.origin_payload, h.ai_session, h.ai_subscription_plan, h.ai_input_tokens, h.ai_output_tokens,
			h.ai_prompt_length, sua.ai_agent_name
		FROM heartbeats h
		LEFT JOIN source_user_agents sua ON sua.id = h.source_user_agent_id
		WHERE h.time >= $1 AND h.time < $2
		ORDER BY h.time ASC, h.entity ASC
	`, start, end)
	if err != nil {
		return nil, fmt.Errorf("list heartbeats by range: %w", err)
	}
	defer rows.Close()

	return scanHeartbeats(rows)
}

func (s *Store) GetHeartbeatBounds(ctx context.Context) (*time.Time, *time.Time, error) {
	var minTime *time.Time
	var maxTime *time.Time
	if err := s.db.QueryRow(ctx, `
		SELECT MIN(time), MAX(time)
		FROM heartbeats
	`).Scan(&minTime, &maxTime); err != nil {
		return nil, nil, fmt.Errorf("get heartbeat bounds: %w", err)
	}
	return minTime, maxTime, nil
}

func (s *Store) ListHeartbeatsForEntity(ctx context.Context, entity, project string, projectRootCount *int) ([]domain.HeartbeatRecord, error) {
	builder := strings.Builder{}
	builder.WriteString(`
		SELECT
			h.id, h.source_heartbeat_id, h.dedupe_hash, h.time, h.source_created_at, h.entity, h.type, h.category,
			h.project, h.branch, h.language, h.project_root_count, h.project_folder, h.lineno, h.cursorpos,
			h.lines, h.is_write, h.is_unsaved_entity, h.ai_line_changes, h.human_line_changes, h.machine_name,
			h.source_machine_name_id, h.plugin, h.source_user_agent_id, h.dependencies, h.import_batch_id,
			h.origin_payload, h.ai_session, h.ai_subscription_plan, h.ai_input_tokens, h.ai_output_tokens,
			h.ai_prompt_length, sua.ai_agent_name
		FROM heartbeats h
		LEFT JOIN source_user_agents sua ON sua.id = h.source_user_agent_id
		WHERE h.entity = $1
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
	if userAgentErr := upsertTempSourceUserAgents(ctx, tx); userAgentErr != nil {
		return 0, 0, userAgentErr
	}
	if machineNameErr := upsertTempSourceMachineNames(ctx, tx); machineNameErr != nil {
		return 0, 0, machineNameErr
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
			origin_payload JSONB NOT NULL,
			ai_session TEXT,
			ai_subscription_plan TEXT,
			ai_input_tokens BIGINT,
			ai_output_tokens BIGINT,
			ai_prompt_length INTEGER
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
			"ai_session", "ai_subscription_plan", "ai_input_tokens", "ai_output_tokens",
			"ai_prompt_length",
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

const upsertTempSourceUserAgentsQuery = `
		INSERT INTO source_user_agents (id, user_agent, ai_agent_key, ai_agent_name, source, updated_at)
		SELECT DISTINCT ON (plugin)
			'local-' || md5(plugin),
			plugin,
			CASE
				WHEN lower(plugin) LIKE '%claude%' THEN 'claude'
				WHEN lower(plugin) LIKE '%codex%' THEN 'codex'
				WHEN lower(plugin) LIKE '%copilot%' THEN 'copilot'
				WHEN lower(plugin) LIKE '%cursor%' THEN 'cursor'
				WHEN lower(plugin) LIKE '%gemini%' THEN 'gemini'
				WHEN lower(plugin) LIKE '%qwen%' THEN 'qwen'
				WHEN lower(plugin) LIKE '%kiro%' THEN 'kiro'
				WHEN lower(plugin) LIKE '%goose%' THEN 'goose'
				WHEN lower(plugin) LIKE '%continue%' THEN 'continue'
				WHEN lower(plugin) LIKE '%windsurf%' THEN 'windsurf'
				WHEN lower(plugin) LIKE '%cline%' THEN 'cline'
				WHEN lower(plugin) LIKE '%roo%' THEN 'roo'
				WHEN lower(plugin) LIKE '%cody%' THEN 'cody'
				WHEN lower(plugin) LIKE '%opencode%' THEN 'opencode'
				WHEN lower(plugin) LIKE '%qoder%' THEN 'qoder'
				ELSE NULL
			END,
			CASE
				WHEN lower(plugin) LIKE '%claude%' THEN 'Claude'
				WHEN lower(plugin) LIKE '%codex%' THEN 'Codex'
				WHEN lower(plugin) LIKE '%copilot%' THEN 'Copilot'
				WHEN lower(plugin) LIKE '%cursor%' THEN 'Cursor'
				WHEN lower(plugin) LIKE '%gemini%' THEN 'Gemini'
				WHEN lower(plugin) LIKE '%qwen%' THEN 'Qwen'
				WHEN lower(plugin) LIKE '%kiro%' THEN 'Kiro'
				WHEN lower(plugin) LIKE '%goose%' THEN 'Goose'
				WHEN lower(plugin) LIKE '%continue%' THEN 'Continue'
				WHEN lower(plugin) LIKE '%windsurf%' THEN 'Windsurf'
				WHEN lower(plugin) LIKE '%cline%' THEN 'Cline'
				WHEN lower(plugin) LIKE '%roo%' THEN 'Roo'
				WHEN lower(plugin) LIKE '%cody%' THEN 'Cody'
				WHEN lower(plugin) LIKE '%opencode%' THEN 'OpenCode'
				WHEN lower(plugin) LIKE '%qoder%' THEN 'Qoder'
				ELSE NULL
			END,
			'local',
			NOW()
		FROM import_heartbeats_tmp
		WHERE (source_user_agent_id IS NULL OR source_user_agent_id = '')
		  AND plugin IS NOT NULL
		  AND plugin <> ''
		ORDER BY plugin, time DESC
		ON CONFLICT DO NOTHING;

		UPDATE import_heartbeats_tmp AS tmp
		SET source_user_agent_id = agents.id
		FROM source_user_agents AS agents
		WHERE (tmp.source_user_agent_id IS NULL OR tmp.source_user_agent_id = '')
		  AND tmp.plugin = agents.user_agent;

		WITH exported AS (
			SELECT DISTINCT ON (source_user_agent_id)
				source_user_agent_id AS id,
				NULLIF(plugin, '') AS user_agent,
				CASE
					WHEN lower(plugin) LIKE '%claude%' THEN 'claude'
					WHEN lower(plugin) LIKE '%codex%' THEN 'codex'
					WHEN lower(plugin) LIKE '%copilot%' THEN 'copilot'
					WHEN lower(plugin) LIKE '%cursor%' THEN 'cursor'
					WHEN lower(plugin) LIKE '%gemini%' THEN 'gemini'
					WHEN lower(plugin) LIKE '%qwen%' THEN 'qwen'
					WHEN lower(plugin) LIKE '%kiro%' THEN 'kiro'
					WHEN lower(plugin) LIKE '%goose%' THEN 'goose'
					WHEN lower(plugin) LIKE '%continue%' THEN 'continue'
					WHEN lower(plugin) LIKE '%windsurf%' THEN 'windsurf'
					WHEN lower(plugin) LIKE '%cline%' THEN 'cline'
					WHEN lower(plugin) LIKE '%roo%' THEN 'roo'
					WHEN lower(plugin) LIKE '%cody%' THEN 'cody'
					WHEN lower(plugin) LIKE '%opencode%' THEN 'opencode'
					WHEN lower(plugin) LIKE '%qoder%' THEN 'qoder'
					ELSE NULL
				END AS ai_agent_key,
				CASE
					WHEN lower(plugin) LIKE '%claude%' THEN 'Claude'
					WHEN lower(plugin) LIKE '%codex%' THEN 'Codex'
					WHEN lower(plugin) LIKE '%copilot%' THEN 'Copilot'
					WHEN lower(plugin) LIKE '%cursor%' THEN 'Cursor'
					WHEN lower(plugin) LIKE '%gemini%' THEN 'Gemini'
					WHEN lower(plugin) LIKE '%qwen%' THEN 'Qwen'
					WHEN lower(plugin) LIKE '%kiro%' THEN 'Kiro'
					WHEN lower(plugin) LIKE '%goose%' THEN 'Goose'
					WHEN lower(plugin) LIKE '%continue%' THEN 'Continue'
					WHEN lower(plugin) LIKE '%windsurf%' THEN 'Windsurf'
					WHEN lower(plugin) LIKE '%cline%' THEN 'Cline'
					WHEN lower(plugin) LIKE '%roo%' THEN 'Roo'
					WHEN lower(plugin) LIKE '%cody%' THEN 'Cody'
					WHEN lower(plugin) LIKE '%opencode%' THEN 'OpenCode'
					WHEN lower(plugin) LIKE '%qoder%' THEN 'Qoder'
					ELSE NULL
				END AS ai_agent_name
			FROM import_heartbeats_tmp
			WHERE source_user_agent_id IS NOT NULL
			  AND source_user_agent_id <> ''
			ORDER BY source_user_agent_id, CASE WHEN plugin IS NULL OR plugin = '' THEN 1 ELSE 0 END, time DESC
		),
		replacements AS (
			SELECT DISTINCT ON (user_agent) *
			FROM exported
			WHERE user_agent IS NOT NULL
			ORDER BY user_agent, id
		)
		UPDATE source_user_agents AS agents
		SET id = replacements.id,
			ai_agent_key = COALESCE(replacements.ai_agent_key, agents.ai_agent_key),
			ai_agent_name = COALESCE(replacements.ai_agent_name, agents.ai_agent_name),
			source = 'wakatime-export',
			updated_at = NOW()
		FROM replacements
		WHERE agents.user_agent = replacements.user_agent
		  AND agents.id <> replacements.id
		  AND NOT EXISTS (
			SELECT 1
			FROM source_user_agents AS existing
			WHERE existing.id = replacements.id
		  );

		WITH exported AS (
			SELECT DISTINCT ON (source_user_agent_id)
				source_user_agent_id AS id,
				NULLIF(plugin, '') AS user_agent
			FROM import_heartbeats_tmp
			WHERE source_user_agent_id IS NOT NULL
			  AND source_user_agent_id <> ''
			ORDER BY source_user_agent_id, CASE WHEN plugin IS NULL OR plugin = '' THEN 1 ELSE 0 END, time DESC
		),
		replacements AS (
			SELECT DISTINCT ON (user_agent) *
			FROM exported
			WHERE user_agent IS NOT NULL
			ORDER BY user_agent, id
		),
		moved_heartbeats AS (
			UPDATE heartbeats AS heartbeats
			SET source_user_agent_id = replacements.id
			FROM source_user_agents AS agents
			JOIN replacements ON replacements.user_agent = agents.user_agent
			WHERE agents.id <> replacements.id
			  AND heartbeats.source_user_agent_id = agents.id
			  AND EXISTS (
				SELECT 1
				FROM source_user_agents AS existing
				WHERE existing.id = replacements.id
			  )
			RETURNING agents.id
		)
		DELETE FROM source_user_agents AS agents
		USING replacements
		WHERE agents.user_agent = replacements.user_agent
		  AND agents.id <> replacements.id
		  AND EXISTS (
			SELECT 1
			FROM source_user_agents AS existing
			WHERE existing.id = replacements.id
		  );

		INSERT INTO source_user_agents (id, user_agent, ai_agent_key, ai_agent_name, source, updated_at)
		SELECT DISTINCT ON (source_user_agent_id)
			source_user_agent_id,
			NULLIF(plugin, ''),
			CASE
				WHEN lower(plugin) LIKE '%claude%' THEN 'claude'
				WHEN lower(plugin) LIKE '%codex%' THEN 'codex'
				WHEN lower(plugin) LIKE '%copilot%' THEN 'copilot'
				WHEN lower(plugin) LIKE '%cursor%' THEN 'cursor'
				WHEN lower(plugin) LIKE '%gemini%' THEN 'gemini'
				WHEN lower(plugin) LIKE '%qwen%' THEN 'qwen'
				WHEN lower(plugin) LIKE '%kiro%' THEN 'kiro'
				WHEN lower(plugin) LIKE '%goose%' THEN 'goose'
				WHEN lower(plugin) LIKE '%continue%' THEN 'continue'
				WHEN lower(plugin) LIKE '%windsurf%' THEN 'windsurf'
				WHEN lower(plugin) LIKE '%cline%' THEN 'cline'
				WHEN lower(plugin) LIKE '%roo%' THEN 'roo'
				WHEN lower(plugin) LIKE '%cody%' THEN 'cody'
				WHEN lower(plugin) LIKE '%opencode%' THEN 'opencode'
				WHEN lower(plugin) LIKE '%qoder%' THEN 'qoder'
				ELSE NULL
			END,
			CASE
				WHEN lower(plugin) LIKE '%claude%' THEN 'Claude'
				WHEN lower(plugin) LIKE '%codex%' THEN 'Codex'
				WHEN lower(plugin) LIKE '%copilot%' THEN 'Copilot'
				WHEN lower(plugin) LIKE '%cursor%' THEN 'Cursor'
				WHEN lower(plugin) LIKE '%gemini%' THEN 'Gemini'
				WHEN lower(plugin) LIKE '%qwen%' THEN 'Qwen'
				WHEN lower(plugin) LIKE '%kiro%' THEN 'Kiro'
				WHEN lower(plugin) LIKE '%goose%' THEN 'Goose'
				WHEN lower(plugin) LIKE '%continue%' THEN 'Continue'
				WHEN lower(plugin) LIKE '%windsurf%' THEN 'Windsurf'
				WHEN lower(plugin) LIKE '%cline%' THEN 'Cline'
				WHEN lower(plugin) LIKE '%roo%' THEN 'Roo'
				WHEN lower(plugin) LIKE '%cody%' THEN 'Cody'
				WHEN lower(plugin) LIKE '%opencode%' THEN 'OpenCode'
				WHEN lower(plugin) LIKE '%qoder%' THEN 'Qoder'
				ELSE NULL
			END,
			'wakatime-export',
			NOW()
		FROM import_heartbeats_tmp
		WHERE source_user_agent_id IS NOT NULL
		  AND source_user_agent_id <> ''
		ORDER BY source_user_agent_id, CASE WHEN plugin IS NULL OR plugin = '' THEN 1 ELSE 0 END, time DESC
		ON CONFLICT (id) DO UPDATE
		SET user_agent = COALESCE(EXCLUDED.user_agent, source_user_agents.user_agent),
			ai_agent_key = COALESCE(EXCLUDED.ai_agent_key, source_user_agents.ai_agent_key),
			ai_agent_name = COALESCE(EXCLUDED.ai_agent_name, source_user_agents.ai_agent_name),
			source = EXCLUDED.source,
			updated_at = NOW();

		UPDATE import_heartbeats_tmp AS tmp
		SET source_user_agent_id = agents.id
		FROM source_user_agents AS agents
		WHERE tmp.plugin = agents.user_agent
		  AND tmp.plugin IS NOT NULL
		  AND tmp.plugin <> ''
		  AND (
			tmp.source_user_agent_id IS NULL
			OR tmp.source_user_agent_id = ''
			OR tmp.source_user_agent_id LIKE 'local-%'
		  )
	`

func upsertTempSourceUserAgents(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, upsertTempSourceUserAgentsQuery); err != nil {
		return fmt.Errorf("upsert temp source user agents: %w", err)
	}
	return nil
}

const upsertTempSourceMachineNamesQuery = `
		INSERT INTO source_machine_names (id, machine_name, source, updated_at)
		SELECT DISTINCT ON (machine_name)
			'local-' || md5(machine_name),
			machine_name,
			'local',
			NOW()
		FROM import_heartbeats_tmp
		WHERE (source_machine_name_id IS NULL OR source_machine_name_id = '')
		  AND machine_name IS NOT NULL
		  AND machine_name <> ''
		ORDER BY machine_name, time DESC
		ON CONFLICT DO NOTHING;

		UPDATE import_heartbeats_tmp AS tmp
		SET source_machine_name_id = machines.id
		FROM source_machine_names AS machines
		WHERE (tmp.source_machine_name_id IS NULL OR tmp.source_machine_name_id = '')
		  AND tmp.machine_name = machines.machine_name;

		WITH exported AS (
			SELECT DISTINCT ON (source_machine_name_id)
				source_machine_name_id AS id,
				NULLIF(machine_name, '') AS machine_name
			FROM import_heartbeats_tmp
			WHERE source_machine_name_id IS NOT NULL
			  AND source_machine_name_id <> ''
			ORDER BY source_machine_name_id, CASE WHEN machine_name IS NULL OR machine_name = '' THEN 1 ELSE 0 END, time DESC
		),
		replacements AS (
			SELECT DISTINCT ON (machine_name) *
			FROM exported
			WHERE machine_name IS NOT NULL
			ORDER BY machine_name, id
		),
		moved_heartbeats AS (
			UPDATE heartbeats AS heartbeats
			SET source_machine_name_id = replacements.id
			FROM source_machine_names AS machines
			JOIN replacements ON replacements.machine_name = machines.machine_name
			WHERE machines.id <> replacements.id
			  AND heartbeats.source_machine_name_id = machines.id
			  AND EXISTS (
				SELECT 1
				FROM source_machine_names AS existing
				WHERE existing.id = replacements.id
			  )
			RETURNING machines.id
		)
		DELETE FROM source_machine_names AS machines
		USING replacements
		WHERE machines.machine_name = replacements.machine_name
		  AND machines.id <> replacements.id
		  AND EXISTS (
			SELECT 1
			FROM source_machine_names AS existing
			WHERE existing.id = replacements.id
		  );

		INSERT INTO source_machine_names (id, machine_name, source, updated_at)
		SELECT DISTINCT ON (source_machine_name_id)
			source_machine_name_id,
			NULLIF(machine_name, ''),
			'wakatime-export',
			NOW()
		FROM import_heartbeats_tmp
		WHERE source_machine_name_id IS NOT NULL
		  AND source_machine_name_id <> ''
		ORDER BY source_machine_name_id, CASE WHEN machine_name IS NULL OR machine_name = '' THEN 1 ELSE 0 END, time DESC
		ON CONFLICT (id) DO UPDATE
		SET machine_name = COALESCE(EXCLUDED.machine_name, source_machine_names.machine_name),
			source = EXCLUDED.source,
			updated_at = NOW();

		UPDATE import_heartbeats_tmp AS tmp
		SET source_machine_name_id = machines.id
		FROM source_machine_names AS machines
		WHERE tmp.machine_name = machines.machine_name
		  AND tmp.machine_name IS NOT NULL
		  AND tmp.machine_name <> ''
		  AND (
			tmp.source_machine_name_id IS NULL
			OR tmp.source_machine_name_id = ''
			OR tmp.source_machine_name_id LIKE 'local-%'
		  )
	`

func upsertTempSourceMachineNames(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, upsertTempSourceMachineNamesQuery); err != nil {
		return fmt.Errorf("upsert temp source machine names: %w", err)
	}
	return nil
}

const insertTempImportRowsQuery = `
		WITH deduped AS (
			SELECT DISTINCT ON (dedupe_hash) *
			FROM import_heartbeats_tmp
			ORDER BY dedupe_hash, source_created_at DESC NULLS LAST, time DESC
		)
		INSERT INTO heartbeats (
			id, source_heartbeat_id, dedupe_hash, time, source_created_at, entity, type, category,
			project, branch, language, project_root_count, project_folder, lineno, cursorpos,
			lines, is_write, is_unsaved_entity, ai_line_changes, human_line_changes, machine_name,
			source_machine_name_id, plugin, source_user_agent_id, dependencies, import_batch_id,
			origin_payload, ai_session, ai_subscription_plan, ai_input_tokens, ai_output_tokens,
			ai_prompt_length, created_at, updated_at
		)
		SELECT
			id, NULLIF(source_heartbeat_id, ''), dedupe_hash, time, source_created_at, entity, type, category,
			NULLIF(project, ''), NULLIF(branch, ''), NULLIF(language, ''), project_root_count,
			NULLIF(project_folder, ''), lineno, cursorpos, lines, is_write, is_unsaved_entity,
			ai_line_changes, human_line_changes, NULLIF(machine_name, ''),
			NULLIF(source_machine_name_id, ''), NULLIF(plugin, ''), NULLIF(source_user_agent_id, ''),
			dependencies, import_batch_id, origin_payload, NULLIF(ai_session, ''),
			NULLIF(ai_subscription_plan, ''), ai_input_tokens, ai_output_tokens, ai_prompt_length,
			NOW(), NOW()
		FROM deduped
		ON CONFLICT (dedupe_hash) DO UPDATE
		SET source_heartbeat_id = EXCLUDED.source_heartbeat_id,
			time = EXCLUDED.time,
			source_created_at = EXCLUDED.source_created_at,
			entity = EXCLUDED.entity,
			type = EXCLUDED.type,
			category = EXCLUDED.category,
			project = EXCLUDED.project,
			branch = EXCLUDED.branch,
			language = EXCLUDED.language,
			project_root_count = EXCLUDED.project_root_count,
			project_folder = EXCLUDED.project_folder,
			lineno = EXCLUDED.lineno,
			cursorpos = EXCLUDED.cursorpos,
			lines = EXCLUDED.lines,
			is_write = EXCLUDED.is_write,
			is_unsaved_entity = EXCLUDED.is_unsaved_entity,
			ai_line_changes = EXCLUDED.ai_line_changes,
			human_line_changes = EXCLUDED.human_line_changes,
			machine_name = EXCLUDED.machine_name,
			source_machine_name_id = EXCLUDED.source_machine_name_id,
			plugin = EXCLUDED.plugin,
			source_user_agent_id = EXCLUDED.source_user_agent_id,
			dependencies = EXCLUDED.dependencies,
			import_batch_id = EXCLUDED.import_batch_id,
			origin_payload = EXCLUDED.origin_payload,
			ai_session = EXCLUDED.ai_session,
			ai_subscription_plan = EXCLUDED.ai_subscription_plan,
			ai_input_tokens = EXCLUDED.ai_input_tokens,
			ai_output_tokens = EXCLUDED.ai_output_tokens,
			ai_prompt_length = EXCLUDED.ai_prompt_length,
			updated_at = NOW()
	`

func insertTempImportRows(ctx context.Context, tx pgx.Tx) (int64, error) {
	tag, err := tx.Exec(ctx, insertTempImportRowsQuery)
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
		var aiSession, aiSubscriptionPlan, aiAgentName *string
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
			&aiSession,
			&aiSubscriptionPlan,
			&record.AIInputTokens,
			&record.AIOutputTokens,
			&record.AIPromptLength,
			&aiAgentName,
		); err != nil {
			return nil, fmt.Errorf("scan heartbeat row: %w", err)
		}

		record.SourceHeartbeatID = derefString(sourceHeartbeatID)
		record.AISession = derefString(aiSession)
		record.AISubscriptionPlan = derefString(aiSubscriptionPlan)
		record.Project = derefString(project)
		record.Branch = derefString(branch)
		record.Language = derefString(language)
		record.ProjectFolder = derefString(projectFolder)
		record.MachineName = derefString(machineName)
		record.SourceMachineNameID = derefString(sourceMachineNameID)
		record.Plugin = derefString(plugin)
		record.SourceUserAgentID = derefString(sourceUserAgentID)
		record.AIAgentName = derefString(aiAgentName)
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
		"ai_session", "ai_subscription_plan", "ai_input_tokens", "ai_output_tokens",
		"ai_prompt_length",
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
	AISession           string
	AISubscriptionPlan  string
	AIInputTokens       *int64
	AIOutputTokens      *int64
	AIPromptLength      *int
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
		nullableString(p.AISession),
		nullableString(p.AISubscriptionPlan),
		p.AIInputTokens,
		p.AIOutputTokens,
		p.AIPromptLength,
	}
}

func parseHeartbeatCSVRecord(record []string, batchID string) (id, dedupeHash string, parsed *parsedHeartbeatCSV, err error) {
	if len(record) != 29 {
		return "", "", nil, fmt.Errorf("expected 29 columns, got %d", len(record))
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
		AISession:           record[24],
		AISubscriptionPlan:  record[25],
		AIInputTokens:       fields.AIInputTokens,
		AIOutputTokens:      fields.AIOutputTokens,
		AIPromptLength:      fields.AIPromptLength,
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
	AIInputTokens    *int64
	AIOutputTokens   *int64
	AIPromptLength   *int
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
	fields.AIInputTokens, err = parseOptionalInt64(record[26])
	if err != nil {
		return fields, fmt.Errorf("parse ai_input_tokens: %w", err)
	}
	fields.AIOutputTokens, err = parseOptionalInt64(record[27])
	if err != nil {
		return fields, fmt.Errorf("parse ai_output_tokens: %w", err)
	}
	fields.AIPromptLength, err = parseOptionalInt(record[28])
	if err != nil {
		return fields, fmt.Errorf("parse ai_prompt_length: %w", err)
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

func parseOptionalInt64(value string) (*int64, error) {
	if value == "" {
		return nil, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
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
