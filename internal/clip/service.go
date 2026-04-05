package clip

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"broadcast/internal/media"
	"broadcast/internal/model"
)

type Service struct {
	ffmpegBin string
	workDir   string
	mediaSvc  *media.Service
}

func NewService(ffmpegBin, workDir string, mediaSvc *media.Service) *Service {
	return &Service{
		ffmpegBin: ffmpegBin,
		workDir:   workDir,
		mediaSvc:  mediaSvc,
	}
}

func (s *Service) ClipVideo(ctx context.Context, req model.ClipRequest) (model.ClipResponse, error) {
	videoPath, err := filepath.Abs(strings.TrimSpace(req.VideoPath))
	if err != nil {
		return model.ClipResponse{}, fmt.Errorf("resolve video_path failed: %w", err)
	}
	if _, err := os.Stat(videoPath); err != nil {
		return model.ClipResponse{}, fmt.Errorf("video_path not found: %w", err)
	}
	if len(req.Clips) == 0 {
		return model.ClipResponse{}, fmt.Errorf("clips is required")
	}

	duration, err := s.mediaSvc.ProbeDuration(ctx, videoPath)
	if err != nil {
		return model.ClipResponse{}, err
	}

	outputDir := strings.TrimSpace(req.OutputDir)
	if outputDir == "" {
		outputDir = filepath.Join(s.workDir, "output", "clips")
	}
	outputDir, err = filepath.Abs(outputDir)
	if err != nil {
		return model.ClipResponse{}, fmt.Errorf("resolve output_dir failed: %w", err)
	}
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return model.ClipResponse{}, fmt.Errorf("create output dir failed: %w", err)
	}

	results := make([]model.ClipResult, 0, len(req.Clips))
	base := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))
	ts := time.Now().Unix()

	for i, c := range req.Clips {
		start := c.Start
		end := c.End
		if start < 0 || end <= start || end > duration {
			return model.ClipResponse{}, fmt.Errorf("invalid clip range at index %d: start=%v end=%v duration=%v", i, start, end, duration)
		}

		clipDuration := end - start
		title := strings.TrimSpace(c.Title)
		if title == "" {
			title = fmt.Sprintf("clip_%03d", i+1)
		}

		outName := fmt.Sprintf("%s_%03d_%s_%d.mp4", base, i+1, sanitize(title), ts)
		outPath := filepath.Join(outputDir, outName)
		if err := s.runFFmpegClip(ctx, videoPath, outPath, start, clipDuration); err != nil {
			return model.ClipResponse{}, fmt.Errorf("clip failed at index %d: %w", i, err)
		}

		results = append(results, model.ClipResult{
			Title:    title,
			Start:    round2(start),
			End:      round2(end),
			Duration: round2(clipDuration),
			FilePath: outPath,
		})
	}

	return model.ClipResponse{Outputs: results}, nil
}

func (s *Service) runFFmpegClip(ctx context.Context, input, output string, start, dur float64) error {
	args := []string{
		"-y",
		"-ss", strconv.FormatFloat(start, 'f', 3, 64),
		"-i", input,
		"-t", strconv.FormatFloat(dur, 'f', 3, 64),
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "23",
		"-c:a", "aac",
		"-movflags", "+faststart",
		output,
	}

	cmd := exec.CommandContext(ctx, s.ffmpegBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w, output: %s", err, truncate(string(out), 800))
	}
	return nil
}

func sanitize(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "clip"
	}
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return replacer.Replace(s)
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
