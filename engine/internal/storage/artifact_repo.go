package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func (s *Store) CreateArtifactVersion(ctx context.Context, variantID string, req models.ArtifactRequest, dimensions map[string]any) (*models.ArtifactVersion, error) {
	hash := sha256.Sum256([]byte(req.Content))
	artifactID := uuid.NewString()
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO artifact_versions (id, variant_id, artifact_type, source_kind, display_name, source_ref, file_path, content, content_hash, dimensions_json)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, artifactID, variantID, req.ArtifactType, fallbackString(req.SourceKind, "custom_file"), nullableString(req.DisplayName), nullableString(req.SourceRef), req.FilePath, req.Content, fmt.Sprintf("%x", hash[:]), marshalJSON(dimensions))
	if err != nil {
		return nil, fmt.Errorf("create artifact version: %w", err)
	}
	return s.GetArtifactVersion(ctx, artifactID)
}

func (s *Store) ListArtifactVersionsByVariant(ctx context.Context, variantID string) ([]models.ArtifactVersion, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, variant_id, artifact_type, source_kind, display_name, source_ref, file_path, content, content_hash, dimensions_json, created_at
		FROM artifact_versions WHERE variant_id = ? ORDER BY created_at DESC
	`, variantID)
	if err != nil {
		return nil, fmt.Errorf("list artifact versions: %w", err)
	}
	defer rows.Close()
	artifacts := make([]models.ArtifactVersion, 0)
	for rows.Next() {
		artifact, scanErr := scanArtifactVersion(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		artifacts = append(artifacts, artifact)
	}
	return artifacts, rows.Err()
}

func (s *Store) GetArtifactVersion(ctx context.Context, artifactID string) (*models.ArtifactVersion, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, variant_id, artifact_type, source_kind, display_name, source_ref, file_path, content, content_hash, dimensions_json, created_at
		FROM artifact_versions WHERE id = ?
	`, artifactID)
	artifact, err := scanArtifactVersion(row)
	if err != nil {
		return nil, err
	}
	return &artifact, nil
}

func (s *Store) GetLatestArtifactByVariant(ctx context.Context, variantID string) (*models.ArtifactVersion, error) {
	row := s.DB.QueryRowContext(ctx, `
		SELECT id, variant_id, artifact_type, source_kind, display_name, source_ref, file_path, content, content_hash, dimensions_json, created_at
		FROM artifact_versions WHERE variant_id = ? ORDER BY created_at DESC LIMIT 1
	`, variantID)
	artifact, err := scanArtifactVersion(row)
	if err != nil {
		return nil, err
	}
	return &artifact, nil
}

func (s *Store) ListEffectiveArtifactsByVariant(ctx context.Context, variantID string) ([]models.ArtifactVersion, error) {
	artifacts, err := s.ListArtifactVersionsByVariant(ctx, variantID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	effective := make([]models.ArtifactVersion, 0, len(artifacts))
	for _, artifact := range artifacts {
		if seen[artifact.FilePath] {
			continue
		}
		seen[artifact.FilePath] = true
		effective = append(effective, artifact)
	}
	return effective, nil
}

func (s *Store) DiffArtifactVersions(ctx context.Context, fromID string, toID string) (*models.ArtifactDiff, error) {
	fromArtifact, err := s.GetArtifactVersion(ctx, fromID)
	if err != nil {
		return nil, err
	}
	toArtifact, err := s.GetArtifactVersion(ctx, toID)
	if err != nil {
		return nil, err
	}
	return &models.ArtifactDiff{From: *fromArtifact, To: *toArtifact, Diff: simpleTextDiff(fromArtifact.Content, toArtifact.Content)}, nil
}

func scanArtifactVersion(scanner interface{ Scan(dest ...any) error }) (models.ArtifactVersion, error) {
	var artifact models.ArtifactVersion
	var dimensions, displayName, sourceRef sql.NullString
	if err := scanner.Scan(&artifact.ID, &artifact.VariantID, &artifact.ArtifactType, &artifact.SourceKind, &displayName, &sourceRef, &artifact.FilePath, &artifact.Content, &artifact.ContentHash, &dimensions, &artifact.CreatedAt); err != nil {
		return artifact, fmt.Errorf("scan artifact version: %w", err)
	}
	artifact.DisplayName = displayName.String
	artifact.SourceRef = sourceRef.String
	artifact.Dimensions = unmarshalJSON(dimensions.String, map[string]any{})
	return artifact, nil
}

func simpleTextDiff(from string, to string) string {
	fromLines := strings.Split(from, "\n")
	toLines := strings.Split(to, "\n")
	var builder strings.Builder
	maxLen := len(fromLines)
	if len(toLines) > maxLen {
		maxLen = len(toLines)
	}
	for idx := 0; idx < maxLen; idx++ {
		var left, right string
		if idx < len(fromLines) {
			left = fromLines[idx]
		}
		if idx < len(toLines) {
			right = toLines[idx]
		}
		if left == right {
			continue
		}
		if left != "" {
			builder.WriteString("- " + left + "\n")
		}
		if right != "" {
			builder.WriteString("+ " + right + "\n")
		}
	}
	return strings.TrimSpace(builder.String())
}
