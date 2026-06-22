package main

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"igloo/cmd/internal/ffmpeg"
	"igloo/cmd/internal/ffmpeg/fmp4testutil"
)

type fakeFFmpegRunPlan struct {
	StartErr   error
	ExitErr    error
	WriteFiles func(outDir string) error
}

type fakeFFmpeg struct {
	mu    sync.Mutex
	plans []fakeFFmpegRunPlan
	calls []ffmpeg.HLSParams
}

func (f *fakeFFmpeg) RunHLS(
	_ context.Context,
	params ffmpeg.HLSParams,
	onExit func(exitErr error, stderrTail []string),
) (*exec.Cmd, error) {
	f.mu.Lock()
	callIndex := len(f.calls)
	f.calls = append(f.calls, params)

	if callIndex >= len(f.plans) {
		f.mu.Unlock()
		return nil, fmt.Errorf("unexpected RunHLS call %d", callIndex)
	}

	plan := f.plans[callIndex]
	f.mu.Unlock()

	if plan.StartErr != nil {
		return nil, plan.StartErr
	}

	if plan.WriteFiles != nil {
		err := plan.WriteFiles(params.OutDir)
		if err != nil {
			return nil, err
		}
	}

	if onExit != nil {
		onExit(plan.ExitErr, nil)
	}

	return &exec.Cmd{}, nil
}

func (f *fakeFFmpeg) ExtractSubtitleAsWebVTT(_ context.Context, _ string, _ int64) ([]byte, error) {
	return []byte("WEBVTT\n"), nil
}

func (f *fakeFFmpeg) Capabilities() ffmpeg.Capabilities {
	return ffmpeg.Capabilities{}
}

func (f *fakeFFmpeg) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

func (f *fakeFFmpeg) Calls() []ffmpeg.HLSParams {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]ffmpeg.HLSParams, len(f.calls))
	copy(out, f.calls)
	return out
}

type testFMP4Fixture = fmp4testutil.Fixture

func writeTestHLSFixture(outDir string, fixture testFMP4Fixture) error {
	return fmp4testutil.WriteHLSFixture(outDir, fixture)
}
