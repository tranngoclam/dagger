package core

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/dagger/dagger/core/pipeline"
	bkclient "github.com/moby/buildkit/client"
	"github.com/moby/buildkit/client/llb"
	bkgw "github.com/moby/buildkit/frontend/gateway/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/vito/progrock"
)

type Host struct {
	Workdir   string
	DisableRW bool
}

func NewHost(workdir string, disableRW bool) *Host {
	return &Host{
		Workdir:   workdir,
		DisableRW: disableRW,
	}
}

type HostVariable struct {
	Name string `json:"name"`
}

type CopyFilter struct {
	Exclude []string
	Include []string
}

func (host *Host) Directory(ctx context.Context, dirPath string, p pipeline.Path, platform specs.Platform, filter CopyFilter) (*Directory, error) {
	if host.DisableRW {
		return nil, ErrHostRWDisabled
	}

	var absPath string
	var err error
	if filepath.IsAbs(dirPath) {
		absPath = dirPath
	} else {
		absPath = filepath.Join(host.Workdir, dirPath)

		if !strings.HasPrefix(absPath, host.Workdir) {
			return nil, fmt.Errorf("path %q escapes workdir; use an absolute path instead", dirPath)
		}
	}

	absPath, err = filepath.EvalSymlinks(absPath)
	if err != nil {
		return nil, fmt.Errorf("eval symlinks: %w", err)
	}

	// Create a sub-pipeline to group llb.Local instructions
	pipelineName := fmt.Sprintf("host.directory %s", absPath)
	ctx, subRecorder := progrock.WithGroup(ctx, pipelineName, progrock.Weak())

	localID := fmt.Sprintf("host:%s", absPath)

	localOpts := []llb.LocalOption{
		// Custom name
		llb.WithCustomNamef("upload %s", absPath),

		// synchronize concurrent filesyncs for the same path
		llb.SharedKeyHint(localID),

		// make the LLB stable so we can test invariants like:
		//
		//   workdir == directory(".")
		llb.LocalUniqueID(localID),
	}

	if len(filter.Exclude) > 0 {
		localOpts = append(localOpts, llb.ExcludePatterns(filter.Exclude))
	}

	if len(filter.Include) > 0 {
		localOpts = append(localOpts, llb.IncludePatterns(filter.Include))
	}

	// copy to scratch to avoid making buildkit's snapshot of the local dir immutable,
	// which makes it unable to reused, which in turn creates cache invalidations
	// TODO: this should be optional, the above issue can also be avoided w/ readonly
	// mount when possible
	st := llb.Scratch().File(
		llb.Copy(llb.Local(absPath, localOpts...), "/", "/"),
		llb.WithCustomNamef("copy %s", absPath),
	)

	def, err := st.Marshal(ctx, llb.Platform(platform))
	if err != nil {
		return nil, err
	}

	defPB := def.ToPB()

	// associate vertexes to the 'host.directory' sub-pipeline
	recordVertexes(subRecorder, defPB)

	return NewDirectory(ctx, defPB, "", p, platform, nil), nil
}

func (host *Host) File(ctx context.Context, path string, p pipeline.Path, platform specs.Platform) (*File, error) {
	if host.DisableRW {
		return nil, ErrHostRWDisabled
	}

	var absPath string
	var err error
	if filepath.IsAbs(path) {
		absPath = path
	} else {
		absPath = filepath.Join(host.Workdir, path)

		if !strings.HasPrefix(absPath, host.Workdir) {
			return nil, fmt.Errorf("path %q escapes workdir; use an absolute path instead", path)
		}
	}

	// Create a sub-pipeline to group llb.Local instructions
	pipelineName := fmt.Sprintf("host.file %s", absPath)
	ctx, subRecorder := progrock.WithGroup(ctx, pipelineName, progrock.Weak())

	localID := fmt.Sprintf("host:%s", absPath)

	localOpts := []llb.LocalOption{
		// Custom name
		llb.WithCustomNamef("upload %s", absPath),

		// synchronize concurrent filesyncs for the same path
		llb.SharedKeyHint(localID),

		// make the LLB stable so we can test invariants like:
		//
		//   workdir == directory(".")
		llb.LocalUniqueID(localID),
	}

	// copy to scratch to avoid making buildkit's snapshot of the local dir immutable,
	// which makes it unable to reused, which in turn creates cache invalidations
	// TODO: this should be optional, the above issue can also be avoided w/ readonly
	// mount when possible
	st := llb.Scratch().File(
		llb.Copy(llb.Local(filepath.Dir(path), localOpts...), filepath.Base(path), "/file"),
		llb.WithCustomNamef("copy %s", absPath),
	)

	def, err := st.Marshal(ctx, llb.Platform(platform))
	if err != nil {
		return nil, err
	}

	defPB := def.ToPB()

	// associate vertexes to the 'host.file' sub-pipeline
	recordVertexes(subRecorder, defPB)

	return NewFile(ctx, defPB, "/file", p, platform, nil), nil
}

func (host *Host) Socket(ctx context.Context, sockPath string) (*Socket, error) {
	if host.DisableRW {
		return nil, ErrHostRWDisabled
	}

	var absPath string
	var err error
	if filepath.IsAbs(sockPath) {
		absPath = sockPath
	} else {
		absPath = filepath.Join(host.Workdir, sockPath)

		if !strings.HasPrefix(absPath, host.Workdir) {
			return nil, fmt.Errorf("path %q escapes workdir; use an absolute path instead", sockPath)
		}
	}

	absPath, err = filepath.EvalSymlinks(absPath)
	if err != nil {
		return nil, fmt.Errorf("eval symlinks: %w", err)
	}

	return NewHostSocket(absPath), nil
}

func (host *Host) Export(
	ctx context.Context,
	export bkclient.ExportEntry,
	bkClient *bkclient.Client,
	solveOpts bkclient.SolveOpt,
	solveCh chan<- *bkclient.SolveStatus,
	buildFn bkgw.BuildFunc,
) error {
	if host.DisableRW {
		return ErrHostRWDisabled
	}

	ch, wg := mirrorCh(solveCh)
	defer wg.Wait()

	solveOpts.Exports = []bkclient.ExportEntry{export}

	_, err := bkClient.Build(ctx, solveOpts, "", buildFn, ch)
	return err
}

func (host *Host) NormalizeDest(dest string) (string, error) {
	if filepath.IsAbs(dest) {
		return dest, nil
	}

	wd, err := filepath.EvalSymlinks(host.Workdir)
	if err != nil {
		return "", err
	}

	dest = filepath.Clean(filepath.Join(wd, dest))

	if dest == wd {
		// writing directly to workdir
		return dest, nil
	}

	// filepath.ToSlash is needed for Windows
	// filepath.Clean transforms / to \ on Windows
	if !strings.HasPrefix(filepath.ToSlash(dest), filepath.ToSlash(wd+"/")) {
		// writing outside of workdir
		return "", fmt.Errorf("destination %q escapes workdir", dest)
	}

	// writing within workdir
	return dest, nil
}
