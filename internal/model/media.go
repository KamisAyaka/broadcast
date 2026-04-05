package model

type ExtractAudioRequest struct {
	VideoPath   string `json:"video_path"`
	AudioFormat string `json:"audio_format"`
	SampleRate  int    `json:"sample_rate"`
	Channels    int    `json:"channels"`
}

type ExtractAudioResponse struct {
	AudioPath string  `json:"audio_path"`
	Duration  float64 `json:"duration"`
}

type TranscribeResponse struct {
	Text      string       `json:"text"`
	Segments  []ASRSegment `json:"segments"`
	Duration  float64      `json:"duration"`
	AudioPath string       `json:"audio_path,omitempty"`
}

type ASRSegment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}
