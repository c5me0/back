package transcript

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// encodeTrack renders one participant's segments, each delayed to its offset from the call start, into a mono AAC m4a.
func (s *Service) encodeTrack(ctx context.Context, dir string, segments []segment, output string) error {
	args := []string{"-y"}
	var graph, labels strings.Builder
	for i, seg := range segments {
		args = append(args, "-i", filepath.Join(dir, seg.File))

		label := fmt.Sprintf("a%d", i)
		if len(segments) == 1 {
			label = "out"
		}
		fmt.Fprintf(&graph, "[%d:a]aresample=async=1,adelay=delays=%d:all=1[%s];", i, seg.OffsetMS, label)
		fmt.Fprintf(&labels, "[%s]", label)
	}
	if len(segments) > 1 {
		fmt.Fprintf(&graph, "%samix=inputs=%d:normalize=0:duration=longest[out]", labels.String(), len(segments))
	}

	args = append(args,
		"-filter_complex", strings.TrimSuffix(graph.String(), ";"),
		"-map", "[out]", "-ac", "1", "-ar", "48000", "-c:a", "aac", "-b:a", "48k", "-movflags", "+faststart", output,
	)
	return s.ffmpeg(ctx, args)
}

// mix overlays the participants' tracks into one mono AAC m4a.
func (s *Service) mix(ctx context.Context, inputs []string, output string) error {
	args := []string{"-y"}
	var labels strings.Builder
	for i, input := range inputs {
		args = append(args, "-i", input)
		fmt.Fprintf(&labels, "[%d:a]", i)
	}

	args = append(args,
		"-filter_complex", fmt.Sprintf("%samix=inputs=%d:normalize=0:duration=longest[out]", labels.String(), len(inputs)),
		"-map", "[out]", "-ac", "1", "-c:a", "aac", "-b:a", "64k", "-movflags", "+faststart", output,
	)
	return s.ffmpeg(ctx, args)
}

func (s *Service) ffmpeg(ctx context.Context, args []string) error {
	var stderr bytes.Buffer
	//nolint:gosec // the binary comes from the server config and the arguments are paths the pipeline built
	cmd := exec.CommandContext(ctx, s.cfg.Recording.FFmpegPath, append([]string{"-nostdin", "-hide_banner", "-loglevel", "error"}, args...)...)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
