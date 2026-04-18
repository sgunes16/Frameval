package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/mustafaselman/frameval/engine/internal/models"
)

func (s *Store) ReplaceExperimentStats(ctx context.Context, experimentID string, stats []models.ExperimentStat) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin stats tx: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM experiment_stats WHERE experiment_id = ?`, experimentID); err != nil {
		return fmt.Errorf("delete existing stats: %w", err)
	}
	for _, stat := range stats {
		if stat.ID == "" {
			stat.ID = uuid.NewString()
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO experiment_stats (
				id, experiment_id, variant_a_id, variant_b_id, metric_name, mean_a, mean_b, median_a, median_b,
				std_a, std_b, mann_whitney_u, p_value, cohens_d, ci_lower, ci_upper, is_significant, observed_power
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`, stat.ID, experimentID, stat.VariantAID, stat.VariantBID, stat.MetricName, stat.MeanA, stat.MeanB, stat.MedianA, stat.MedianB, stat.StdA, stat.StdB, stat.MannWhitneyU, stat.PValue, stat.CohensD, stat.CILower, stat.CIUpper, boolToInt(stat.IsSignificant), stat.ObservedPower)
		if err != nil {
			return fmt.Errorf("insert experiment stat: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit stats tx: %w", err)
	}
	return nil
}

func (s *Store) ListExperimentStats(ctx context.Context, experimentID string) ([]models.ExperimentStat, error) {
	rows, err := s.DB.QueryContext(ctx, `
		SELECT id, experiment_id, variant_a_id, variant_b_id, metric_name, mean_a, mean_b, median_a, median_b,
		       std_a, std_b, mann_whitney_u, p_value, cohens_d, ci_lower, ci_upper, is_significant, observed_power
		FROM experiment_stats WHERE experiment_id = ? ORDER BY metric_name ASC
	`, experimentID)
	if err != nil {
		return nil, fmt.Errorf("list experiment stats: %w", err)
	}
	defer rows.Close()
	stats := make([]models.ExperimentStat, 0)
	for rows.Next() {
		var stat models.ExperimentStat
		var isSignificant int
		var meanA, meanB, medianA, medianB, stdA, stdB, mannWhitneyU, pValue, cohensD, ciLower, ciUpper, observedPower sql.NullFloat64
		if err := rows.Scan(&stat.ID, &stat.ExperimentID, &stat.VariantAID, &stat.VariantBID, &stat.MetricName, &meanA, &meanB, &medianA, &medianB, &stdA, &stdB, &mannWhitneyU, &pValue, &cohensD, &ciLower, &ciUpper, &isSignificant, &observedPower); err != nil {
			return nil, fmt.Errorf("scan experiment stat: %w", err)
		}
		stat.MeanA = nullFloat(meanA)
		stat.MeanB = nullFloat(meanB)
		stat.MedianA = nullFloat(medianA)
		stat.MedianB = nullFloat(medianB)
		stat.StdA = nullFloat(stdA)
		stat.StdB = nullFloat(stdB)
		stat.MannWhitneyU = nullFloat(mannWhitneyU)
		stat.PValue = nullFloat(pValue)
		stat.CohensD = nullFloat(cohensD)
		stat.CILower = nullFloat(ciLower)
		stat.CIUpper = nullFloat(ciUpper)
		stat.ObservedPower = nullFloat(observedPower)
		stat.IsSignificant = isSignificant == 1
		stats = append(stats, stat)
	}
	return stats, rows.Err()
}

func nullFloat(value sql.NullFloat64) float64 {
	if !value.Valid {
		return 0
	}
	return value.Float64
}
