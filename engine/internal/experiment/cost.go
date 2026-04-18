package experiment

import "github.com/mustafaselman/frameval/engine/internal/models"

func EstimateCost(task models.Task, experiment models.Experiment, modelConfigs []models.ModelConfig) float64 {
	price := 0.01
	for _, config := range modelConfigs {
		if config.ModelID == experiment.Model {
			price = config.InputPricePer1K + config.OutputPricePer1K
			break
		}
	}
	variantCount := len(experiment.Variants)
	if variantCount == 0 {
		variantCount = 1
	}
	return task.ComplexityScore * float64(experiment.RunsPerVariant*variantCount) * price
}
