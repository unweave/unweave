package builder

import (
	"context"
	"io"

	"github.com/unweave/unweave/api/types"
)

// BuildLogsV1 versions the build logs format stored and fetched by the LogDriver.
type BuildLogsV1 struct {
	Version int16            `json:"version"`
	Logs    []types.LogEntry `json:"logs"`
}

// LogDriver defines the interface for storing and retrieving build logs.
type LogDriver interface {
	// GetLogs returns the logs for a build.
	GetLogs(ctx context.Context, buildID string) (logs []types.LogEntry, err error)
	// SaveLogs saves the logs for a build in long term storage.
	SaveLogs(ctx context.Context, buildID string, logs []types.LogEntry) error
}

// Builder defines the interface for building and storing container images.
type Builder interface {
	GetBuilder() string
	// Build builds a container image from a build context.
	// The build context is a zip file containing the source code and any other files
	// needed to build the image.
	Build(ctx context.Context, buildID string, buildCtx io.Reader) (err error)
	// Logs returns the logs for a build.
	Logs(ctx context.Context, buildID string) (logs []types.LogEntry, err error)
	// Push pushes an image to the container registry. The buildID is used as the tag.
	// If you want to use a different tag, use the Tag method instead.
	Push(ctx context.Context, buildID, namespace, reponame string) error
}
