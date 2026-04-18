package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func (s *Store) CreateVariant(ctx context.Context, variant models.Variant) (*models.Variant, error) {
	if variant.ID == "" {
		variant.ID = uuid.NewString()
	}
	_, err := s.DB.ExecContext(ctx, `
		INSERT INTO variants (id, experiment_id, name, description, is_control, ordering)
		VALUES (?, ?, ?, ?, ?, ?)
	`, variant.ID, variant.ExperimentID, variant.Name, variant.Description, boolToInt(variant.IsControl), variant.Ordering)
	if err != nil {
		return nil, fmt.Errorf("create variant: %w", err)
	}
	return s.GetVariant(ctx, variant.ID)
}

func (s *Store) ListVariantsByExperiment(ctx context.Context, experimentID string) ([]models.Variant, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, experiment_id, name, description, is_control, ordering FROM variants WHERE experiment_id = ? ORDER BY ordering ASC`, experimentID)
	if err != nil {
		return nil, fmt.Errorf("list variants: %w", err)
	}
	defer rows.Close()
	variants := make([]models.Variant, 0)
	for rows.Next() {
		variant, scanErr := scanVariant(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		variants = append(variants, variant)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for idx := range variants {
		artifacts, artifactsErr := s.ListArtifactVersionsByVariant(ctx, variants[idx].ID)
		if artifactsErr != nil {
			return nil, artifactsErr
		}
		variants[idx].ArtifactVersions = artifacts
	}
	return variants, nil
}

func (s *Store) GetVariant(ctx context.Context, variantID string) (*models.Variant, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT id, experiment_id, name, description, is_control, ordering FROM variants WHERE id = ?`, variantID)
	variant, err := scanVariant(row)
	if err != nil {
		return nil, err
	}
	artifacts, err := s.ListArtifactVersionsByVariant(ctx, variant.ID)
	if err != nil {
		return nil, err
	}
	variant.ArtifactVersions = artifacts
	return &variant, nil
}

func (s *Store) UpdateVariant(ctx context.Context, variantID string, variant models.Variant) (*models.Variant, error) {
	_, err := s.DB.ExecContext(ctx, `UPDATE variants SET name = ?, description = ?, is_control = ?, ordering = ? WHERE id = ?`, variant.Name, variant.Description, boolToInt(variant.IsControl), variant.Ordering, variantID)
	if err != nil {
		return nil, fmt.Errorf("update variant: %w", err)
	}
	return s.GetVariant(ctx, variantID)
}

func (s *Store) DeleteVariant(ctx context.Context, variantID string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM variants WHERE id = ?`, variantID)
	if err != nil {
		return fmt.Errorf("delete variant: %w", err)
	}
	return nil
}

func scanVariant(scanner interface{ Scan(dest ...any) error }) (models.Variant, error) {
	var variant models.Variant
	var description sql.NullString
	var isControl int
	if err := scanner.Scan(&variant.ID, &variant.ExperimentID, &variant.Name, &description, &isControl, &variant.Ordering); err != nil {
		return variant, fmt.Errorf("scan variant: %w", err)
	}
	variant.Description = description.String
	variant.IsControl = isControl == 1
	return variant, nil
}
