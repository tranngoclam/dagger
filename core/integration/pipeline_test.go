package core

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"dagger.io/dagger"
	"github.com/stretchr/testify/require"
)

type safeBuffer struct {
	bu bytes.Buffer
	mu sync.Mutex
}

func (s *safeBuffer) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bu.Write(p)
}

func (s *safeBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.bu.String()
}

func TestPipeline(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	cacheBuster := fmt.Sprintf("%d", time.Now().UTC().UnixNano())

	t.Run("container pipeline", func(t *testing.T) {
		t.Parallel()

		var logs safeBuffer
		c, err := dagger.Connect(ctx, dagger.WithLogOutput(&logs))
		require.NoError(t, err)
		defer c.Close()

		_, err = c.
			Container().
			Pipeline("container pipeline").
			From("alpine:3.16.2").
			WithExec([]string{"echo", cacheBuster}).
			ExitCode(ctx)

		require.NoError(t, err)

		require.NoError(t, c.Close()) // close + flush logs

		require.Contains(t, logs.String(), "container pipeline")
	})

	t.Run("directory pipeline", func(t *testing.T) {
		t.Parallel()

		var logs safeBuffer
		c, err := dagger.Connect(ctx, dagger.WithLogOutput(&logs))
		require.NoError(t, err)
		defer c.Close()

		contents, err := c.
			Directory().
			Pipeline("directory pipeline").
			WithNewFile("/foo", cacheBuster).
			File("/foo").
			Contents(ctx)

		require.NoError(t, err)
		require.Equal(t, contents, cacheBuster)

		require.NoError(t, c.Close()) // close + flush logs

		require.Contains(t, logs.String(), "directory pipeline")
	})

	t.Run("service pipeline", func(t *testing.T) {
		t.Parallel()

		var logs safeBuffer
		c, err := dagger.Connect(ctx, dagger.WithLogOutput(&logs))
		require.NoError(t, err)
		defer c.Close()

		srv, url := httpService(ctx, t, c, "Hello, world!")

		hostname, err := srv.Hostname(ctx)
		require.NoError(t, err)

		client := c.Container().
			From("alpine:3.16.2").
			WithServiceBinding("www", srv).
			WithExec([]string{"apk", "add", "curl"}).
			WithExec([]string{"curl", "-v", url})

		code, err := client.ExitCode(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, code)

		require.NoError(t, c.Close()) // close + flush logs

		require.Contains(t, logs.String(), "service "+hostname)
	})
}
