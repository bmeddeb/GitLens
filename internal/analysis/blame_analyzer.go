package analysis

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/user/gitlens-pro/internal/gitclient"
	"github.com/user/gitlens-pro/internal/repository"
	"go.uber.org/zap"
)

const (
	maxBlameLines    = 10000
	maxBlameSizeBytes = 5 * 1024 * 1024 // 5MB
)

type BlameAnalyzer interface {
	GetBlame(ctx context.Context, repoID uint64, repoPath, filePath, ref string) (*gitclient.BlameResult, error)
}

type blameAnalyzer struct {
	gitService gitclient.GitService
	cacheRepo  repository.BlameCacheRepository
	logger     *zap.Logger
}

func NewBlameAnalyzer(
	gitService gitclient.GitService,
	cacheRepo repository.BlameCacheRepository,
	logger *zap.Logger,
) BlameAnalyzer {
	return &blameAnalyzer{
		gitService: gitService,
		cacheRepo:  cacheRepo,
		logger:     logger,
	}
}

func (a *blameAnalyzer) GetBlame(ctx context.Context, repoID uint64, repoPath, filePath, ref string) (*gitclient.BlameResult, error) {
	// Check cache
	cached, err := a.cacheRepo.Get(ctx, repoID, filePath, ref)
	if err != nil {
		return nil, fmt.Errorf("checking blame cache: %w", err)
	}

	if cached != nil {
		result, err := decompressBlameResult(cached.BlameData)
		if err != nil {
			a.logger.Warn("failed to decompress cached blame, recomputing",
				zap.Uint64("repo_id", repoID),
				zap.String("file", filePath),
				zap.Error(err),
			)
		} else {
			return result, nil
		}
	}

	// Compute blame
	result, err := a.gitService.Blame(ctx, repoPath, filePath, ref)
	if err != nil {
		return nil, fmt.Errorf("computing blame: %w", err)
	}

	// Cache if within limits
	if len(result.Lines) <= maxBlameLines {
		compressed, err := compressBlameResult(result)
		if err == nil && len(compressed) <= maxBlameSizeBytes {
			lineCount := uint(len(result.Lines))
			bc := &repository.BlameCache{
				RepoID:    repoID,
				FilePath:  filePath,
				CommitSHA: ref,
				BlameData: compressed,
				LineCount: &lineCount,
			}
			if err := a.cacheRepo.Store(ctx, bc); err != nil {
				a.logger.Warn("failed to cache blame result",
					zap.Uint64("repo_id", repoID),
					zap.String("file", filePath),
					zap.Error(err),
				)
			}
		}
	}

	return result, nil
}

func compressBlameResult(result *gitclient.BlameResult) ([]byte, error) {
	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func decompressBlameResult(data []byte) (*gitclient.BlameResult, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	decompressed, err := io.ReadAll(gz)
	if err != nil {
		return nil, err
	}

	var result gitclient.BlameResult
	if err := json.Unmarshal(decompressed, &result); err != nil {
		return nil, err
	}

	return &result, nil
}
