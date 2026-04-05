package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"broadcast/internal/asr"
	"broadcast/internal/config"
	"broadcast/internal/media"
	"broadcast/internal/model"

	"github.com/joho/godotenv"
)

type transcriptResult struct {
	SourceFile  string             `json:"source_file"`
	SourceType  string             `json:"source_type"`
	AudioPath   string             `json:"audio_path,omitempty"`
	Model       string             `json:"model"`
	Duration    float64            `json:"duration"`
	Text        string             `json:"text"`
	Segments    []model.ASRSegment `json:"segments"`
	GeneratedAt string             `json:"generated_at"`
}

func main() {
	_ = godotenv.Load()

	filePathFlag := flag.String("file", "", "source file path, e.g. ./data/input/video/episode.mp4")
	typeFlag := flag.String("type", "auto", "source type: auto|audio|video")
	modelFlag := flag.String("model", "", "asr model, default from ASR_MODEL")
	outputFlag := flag.String("output", "", "output transcript json path")
	flag.Parse()

	if strings.TrimSpace(*filePathFlag) == "" {
		fmt.Fprintln(os.Stderr, "missing required flag: --file")
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}

	sourcePath, err := filepath.Abs(strings.TrimSpace(*filePathFlag))
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve file path failed: %v\n", err)
		os.Exit(1)
	}

	info, err := os.Stat(sourcePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "source file not found: %v\n", err)
		os.Exit(1)
	}
	if info.IsDir() {
		fmt.Fprintln(os.Stderr, "source file must be a file, got directory")
		os.Exit(1)
	}

	sourceType, err := normalizeSourceType(*typeFlag, sourcePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid --type: %v\n", err)
		os.Exit(1)
	}

	mediaSvc := media.NewService(cfg.FFmpegBin, cfg.FFprobeBin, cfg.WorkDir)
	asrClient := asr.NewClient(cfg.ASRBaseURL, cfg.ASRAPIKey, cfg.ASRModel)

	ctx := context.Background()
	audioPath := sourcePath
	duration := 0.0

	if sourceType == "video" {
		out, err := mediaSvc.ExtractAudio(ctx, media.ExtractAudioInput{
			VideoPath:   sourcePath,
			AudioFormat: "mp3",
			SampleRate:  16000,
			Channels:    1,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "extract audio failed: %v\n", err)
			os.Exit(1)
		}
		audioPath = out.AudioPath
		duration = out.Duration
	}

	modelName := strings.TrimSpace(*modelFlag)
	audioDuration, err := mediaSvc.ProbeDuration(ctx, audioPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "probe audio duration failed: %v\n", err)
		os.Exit(1)
	}
	if duration <= 0 {
		duration = audioDuration
	}

	var asrOut asr.TranscribeOutput
	if audioDuration > float64(cfg.ASRChunkSeconds) {
		fmt.Fprintf(os.Stderr, "long media detected (%.2f sec), use chunked transcription\n", audioDuration)
		asrOut, err = transcribeInChunks(ctx, mediaSvc, asrClient, audioPath, modelName, audioDuration, cfg)
	} else {
		asrOut, err = transcribeWithRetry(ctx, asrClient, asr.TranscribeInput{
			FilePath: audioPath,
			Model:    modelName,
		}, cfg.ASRRetryCount)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "asr failed: %v\n", err)
		os.Exit(1)
	}
	if duration <= 0 {
		duration = asrOut.Duration
	}

	segments := make([]model.ASRSegment, 0, len(asrOut.Segments))
	for _, seg := range asrOut.Segments {
		segments = append(segments, model.ASRSegment{
			ID:    seg.ID,
			Start: seg.Start,
			End:   seg.End,
			Text:  seg.Text,
		})
	}

	if modelName == "" {
		modelName = cfg.ASRModel
	}

	result := transcriptResult{
		SourceFile:  sourcePath,
		SourceType:  sourceType,
		AudioPath:   audioPath,
		Model:       modelName,
		Duration:    duration,
		Text:        asrOut.Text,
		Segments:    segments,
		GeneratedAt: time.Now().Format(time.RFC3339),
	}

	outputPath := strings.TrimSpace(*outputFlag)
	if outputPath == "" {
		base := strings.TrimSuffix(filepath.Base(sourcePath), filepath.Ext(sourcePath))
		outputPath = filepath.Join(cfg.WorkDir, "output", "transcripts", base+".transcript.json")
	}

	absOutputPath, err := writeJSON(outputPath, result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "write transcript failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(absOutputPath)
}

func normalizeSourceType(inputType, sourcePath string) (string, error) {
	t := strings.ToLower(strings.TrimSpace(inputType))
	switch t {
	case "", "auto":
		if isVideoExt(filepath.Ext(sourcePath)) {
			return "video", nil
		}
		return "audio", nil
	case "audio", "video":
		return t, nil
	default:
		return "", fmt.Errorf("must be one of auto|audio|video")
	}
}

func isVideoExt(ext string) bool {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".mp4", ".mov", ".mkv", ".avi", ".webm", ".m4v", ".mpeg", ".mpg":
		return true
	default:
		return false
	}
}

func writeJSON(path string, v any) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve output path failed: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
		return "", fmt.Errorf("create output dir failed: %w", err)
	}

	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal json failed: %w", err)
	}
	b = append(b, '\n')

	if err := os.WriteFile(absPath, b, 0o644); err != nil {
		return "", fmt.Errorf("write file failed: %w", err)
	}
	return absPath, nil
}

func transcribeInChunks(
	ctx context.Context,
	mediaSvc *media.Service,
	asrClient *asr.Client,
	audioPath, modelName string,
	audioDuration float64,
	cfg config.App,
) (asr.TranscribeOutput, error) {
	chunkSec := float64(cfg.ASRChunkSeconds)
	overlapSec := float64(cfg.ASRChunkOverlapSeconds)
	stride := chunkSec - overlapSec
	if stride <= 0 {
		stride = chunkSec
	}

	chunkDir := filepath.Join(
		cfg.WorkDir,
		"temp",
		"asr_chunks",
		fmt.Sprintf("%s_%d", strings.TrimSuffix(filepath.Base(audioPath), filepath.Ext(audioPath)), time.Now().Unix()),
	)
	if err := os.MkdirAll(chunkDir, 0o755); err != nil {
		return asr.TranscribeOutput{}, fmt.Errorf("create chunk dir failed: %w", err)
	}
	defer os.RemoveAll(chunkDir)

	merged := make([]asr.Segment, 0, 1024)
	nextID := 0
	lastEnd := 0.0
	chunkIndex := 0

	for start := 0.0; start < audioDuration; start += stride {
		end := start + chunkSec
		if end > audioDuration {
			end = audioDuration
		}
		dur := end - start
		if dur <= 0 {
			break
		}

		chunkIndex++
		fmt.Fprintf(os.Stderr, "chunk %d: [%.2f, %.2f]\n", chunkIndex, start, end)
		chunkPath := filepath.Join(chunkDir, fmt.Sprintf("chunk_%04d.mp3", chunkIndex))
		if err := mediaSvc.ExtractAudioSegment(ctx, audioPath, chunkPath, start, dur, 16000, 1); err != nil {
			return asr.TranscribeOutput{}, err
		}

		out, err := transcribeWithRetry(ctx, asrClient, asr.TranscribeInput{
			FilePath: chunkPath,
			Model:    modelName,
		}, cfg.ASRRetryCount)
		if err != nil {
			return asr.TranscribeOutput{}, fmt.Errorf("chunk %d transcribe failed: %w", chunkIndex, err)
		}

		for _, seg := range out.Segments {
			absStart := seg.Start + start
			absEnd := seg.End + start
			if absEnd <= lastEnd+0.05 {
				continue
			}
			if absStart < lastEnd {
				absStart = lastEnd
			}
			if absEnd <= absStart {
				continue
			}
			text := strings.TrimSpace(seg.Text)
			if text == "" {
				continue
			}

			merged = append(merged, asr.Segment{
				ID:    nextID,
				Start: absStart,
				End:   absEnd,
				Text:  text,
			})
			nextID++
			lastEnd = absEnd
		}
	}

	if len(merged) == 0 {
		return asr.TranscribeOutput{}, errors.New("chunked transcription returned empty segments")
	}

	texts := make([]string, 0, len(merged))
	for _, seg := range merged {
		texts = append(texts, seg.Text)
	}

	return asr.TranscribeOutput{
		Text:     strings.Join(texts, "\n"),
		Segments: merged,
		Duration: audioDuration,
	}, nil
}

func transcribeWithRetry(ctx context.Context, asrClient *asr.Client, in asr.TranscribeInput, retryCount int) (asr.TranscribeOutput, error) {
	if retryCount <= 0 {
		retryCount = 1
	}

	var lastErr error
	for i := 1; i <= retryCount; i++ {
		out, err := asrClient.Transcribe(ctx, in)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if i < retryCount {
			fmt.Fprintf(os.Stderr, "asr retry %d/%d for %s\n", i, retryCount, filepath.Base(in.FilePath))
			time.Sleep(time.Duration(i) * time.Second)
		}
	}

	return asr.TranscribeOutput{}, lastErr
}
