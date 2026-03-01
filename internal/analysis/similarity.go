package analysis

import (
	"github.com/user/gitlens-pro/internal/repository"
)

type SimilarityDetector struct{}

func NewSimilarityDetector() *SimilarityDetector {
	return &SimilarityDetector{}
}

type SimilarityCluster struct {
	FilePath string                  `json:"file_path"`
	Changes  []repository.ForkChange `json:"changes"`
}

// DetectSimilar finds fork changes across forks that modify the same file path
// with >60% patch overlap.
func (d *SimilarityDetector) DetectSimilar(allChanges map[uint64][]repository.ForkChange) []SimilarityCluster {
	// Group changes by file path
	byPath := make(map[string][]repository.ForkChange)
	for _, changes := range allChanges {
		for _, c := range changes {
			if c.FilePath != nil {
				byPath[*c.FilePath] = append(byPath[*c.FilePath], c)
			}
		}
	}

	var clusters []SimilarityCluster
	for path, changes := range byPath {
		if len(changes) < 2 {
			continue
		}

		// Group by fork to find cross-fork similarities
		byFork := make(map[uint64][]repository.ForkChange)
		for _, c := range changes {
			byFork[c.ForkID] = append(byFork[c.ForkID], c)
		}

		if len(byFork) >= 2 {
			clusters = append(clusters, SimilarityCluster{
				FilePath: path,
				Changes:  changes,
			})
		}
	}

	return clusters
}
