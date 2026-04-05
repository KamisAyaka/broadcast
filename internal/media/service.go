package media

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Service struct {
	ffmpegBin  string
	ffprobeBin string
	workDir    string
}

type ExtractAudioInput struct {
	VideoPath   string
	AudioFormat string
	SampleRate  int
	Channels    int
}

type ExtractAudioOutput struct {
	AudioPath string
	Duration  float64
}

func NewService(ffmpegBin, ffprobeBin, workDir string) *Service {
	return &Service{
		ffmpegBin:  ffmpegBin,
		ffprobeBin: ffprobeBin,
		workDir:    workDir,
	}
}

func (s *Service) ExtractAudio(ctx context.Context, in ExtractAudioInput) (ExtractAudioOutput, error) {
	if strings.TrimSpace(in.VideoPath) == "" {
		return ExtractAudioOutput{}, errors.New("video_path is required")
	}

	videoPath, err := filepath.Abs(in.VideoPath)
	if err != nil {
		return ExtractAudioOutput{}, fmt.Errorf("resolve video_path failed: %w", err)
	}

	stat, err := os.Stat(videoPath)
	if err != nil {
		return ExtractAudioOutput{}, fmt.Errorf("video file not found: %w", err)
	}
	if stat.IsDir() {
		return ExtractAudioOutput{}, errors.New("video_path must be a file")
	}

	format := normalizeFormat(in.AudioFormat)
	sampleRate := in.SampleRate
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	channels := in.Channels
	if channels <= 0 {
		channels = 1
	}

	duration, err := s.ProbeDuration(ctx, videoPath)
	if err != nil {
		return ExtractAudioOutput{}, err
	}

	outputDir := filepath.Join(s.workDir, "output", "audio")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return ExtractAudioOutput{}, fmt.Errorf("create output dir failed: %w", err)
	}

	outputFile := filepath.Join(outputDir, fmt.Sprintf("%s_%d.%s", trimFileName(filepath.Base(videoPath)), time.Now().Unix(), format))

	if err := s.runFFmpeg(ctx, videoPath, outputFile, sampleRate, channels, format); err != nil {
		return ExtractAudioOutput{}, err
	}

	return ExtractAudioOutput{
		AudioPath: outputFile,
		Duration:  duration,
	}, nil
}

func (s *Service) runFFmpeg(ctx context.Context, input, output string, sampleRate, channels int, format string) error {
	args := []string{"-y", "-i", input, "-vn", "-ac", strconv.Itoa(channels), "-ar", strconv.Itoa(sampleRate)}

	if format == "wav" {
		args = append(args, "-c:a", "pcm_s16le")
	}

	args = append(args, output)

	cmd := exec.CommandContext(ctx, s.ffmpegBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg failed: %w, output: %s", err, truncate(string(out), 800))
	}

	return nil
}

func (s *Service) ProbeDuration(ctx context.Context, input string) (float64, error) {
	args := []string{"-v", "error", "-show_entries", "format=duration", "-of", "json", input}

	cmd := exec.CommandContext(ctx, s.ffprobeBin, args...)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	var payload struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &payload); err != nil {
		return 0, fmt.Errorf("parse ffprobe output failed: %w", err)
	}
	if payload.Format.Duration == "" {
		return 0, errors.New("duration not found")
	}

	duration, err := strconv.ParseFloat(payload.Format.Duration, 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration failed: %w", err)
	}
	return duration, nil
}

func (s *Service) ExtractAudioSegment(ctx context.Context, input, output string, startSec, durationSec float64, sampleRate, channels int) error {
	if strings.TrimSpace(input) == "" {
		return errors.New("input is required")
	}
	if strings.TrimSpace(output) == "" {
		return errors.New("output is required")
	}
	if durationSec <= 0 {
		return errors.New("durationSec must be > 0")
	}
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	if channels <= 0 {
		channels = 1
	}
	if startSec < 0 {
		startSec = 0
	}

	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return fmt.Errorf("create segment dir failed: %w", err)
	}

	args := []string{
		"-y",
		"-ss", strconv.FormatFloat(startSec, 'f', 3, 64),
		"-i", input,
		"-t", strconv.FormatFloat(durationSec, 'f', 3, 64),
		"-vn",
		"-ac", strconv.Itoa(channels),
		"-ar", strconv.Itoa(sampleRate),
		output,
	}

	cmd := exec.CommandContext(ctx, s.ffmpegBin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg segment failed: %w, output: %s", err, truncate(string(out), 800))
	}
	return nil
}

func normalizeFormat(in string) string {
	s := strings.ToLower(strings.TrimSpace(in))
	switch s {
	case "wav", "mp3", "m4a":
		return s
	default:
		return "mp3"
	}
}

func trimFileName(name string) string {
	base := strings.TrimSuffix(name, filepath.Ext(name))
	base = strings.ReplaceAll(base, " ", "_")
	if base == "" {
		return "audio"
	}
	return base
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
